package auth

import (
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (shared with memory/eventstore + auth stores)
)

// AuthFailureReason explains why a callback token became invalid. An empty
// reason means the token is still healthy.
type AuthFailureReason string

const (
	// ReasonExpired means the token passed its expiry.
	ReasonExpired AuthFailureReason = "expired"
	// ReasonRevoked means an operator revoked the token.
	ReasonRevoked AuthFailureReason = "revoked"
	// ReasonMissing means the token was never issued / not found.
	ReasonMissing AuthFailureReason = "missing"
)

// DegradePolicy says what to do when a callback auth fails.
type DegradePolicy string

const (
	// DegradeDeny hard-denies (fail-closed default).
	DegradeDeny DegradePolicy = "deny"
	// DegradeReadOnly downgrades the callback to read-only.
	DegradeReadOnly DegradePolicy = "readonly"
	// DegradeQueue holds the callback for retry.
	DegradeQueue DegradePolicy = "queue"
)

// CallbackToken is a credential issued to an external integration so it can
// call back into the platform (roadmap P1-D, F174). It carries scopes, an
// expiry, an optional failure reason, and a degrade policy used when auth
// fails so the caller degrades safely instead of crashing.
type CallbackToken struct {
	mu sync.Mutex

	Token     string
	UserID    string
	Scopes    []string
	ExpiresAt time.Time
	Failure   AuthFailureReason
	Degrade   DegradePolicy
}

// NewCallbackToken issues a token valid until expiresAt.
func NewCallbackToken(token, userID string, scopes []string, expiresAt time.Time) *CallbackToken {
	return &CallbackToken{
		Token:     token,
		UserID:    userID,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		Degrade:   DegradeDeny,
	}
}

// Valid reports whether the token may be used at now. Fail-closed: a token with
// a recorded failure reason, or past expiry, is invalid.
func (c *CallbackToken) Valid(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Failure != "" {
		return false
	}
	return !now.After(c.ExpiresAt)
}

// MarkFailure records a failure reason (e.g. on expiry detection).
func (c *CallbackToken) MarkFailure(r AuthFailureReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Failure = r
}

// Refresh extends the token's expiry (rotation). It clears any failure reason.
func (c *CallbackToken) Refresh(expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ExpiresAt = expiresAt
	c.Failure = ""
}

// CallbackTokenStore persists callback tokens in a dedicated SQLite file so a
// process restart does not drop issued credentials (roadmap P1-D / F174: the
// original gap was "进程重启会丢 token"). It is the SQLite-backed source of
// truth for callback auth (F174). The store is fail-closed: Lookup only yields
// a token when it is currently valid at now; missing / expired / failed tokens
// return (nil, false) — never a usable credential.
type CallbackTokenStore struct {
	db *sql.DB
}

// NewCallbackTokenStore opens (or creates) the callback token store at path.
// An empty path yields a nil store: callers must treat it as "callback auth
// unavailable" and fail closed (no token persisted, no token accepted). path is
// used verbatim as the SQLite database file name; a future server wiring passes
// "<ConfigRoot>/callback-auth.db".
func NewCallbackTokenStore(path string) *CallbackTokenStore {
	if path == "" {
		return nil
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(3)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS callback_token (
		token      TEXT PRIMARY KEY,
		user_id    TEXT,
		scopes     TEXT DEFAULT '[]',
		expires_at INTEGER,
		failure    TEXT DEFAULT '',
		degrade    TEXT DEFAULT 'deny')`); err != nil {
		db.Close()
		return nil
	}
	return &CallbackTokenStore{db: db}
}

// Put upserts a callback token, persisting its current state (including any
// recorded failure reason and expiry). Callers that mutate a token via
// MarkFailure/Refresh must Put it again to land the change to disk.
func (s *CallbackTokenStore) Put(c *CallbackToken) {
	if s == nil || s.db == nil || c == nil {
		return
	}
	scopes, err := json.Marshal(c.Scopes)
	if err != nil {
		scopes = []byte("[]")
	}
	_, _ = s.db.Exec(
		`INSERT OR REPLACE INTO callback_token
		 (token, user_id, scopes, expires_at, failure, degrade)
		 VALUES (?,?,?,?,?,?)`,
		c.Token, c.UserID, string(scopes), c.ExpiresAt.UnixMilli(),
		string(c.Failure), string(c.Degrade))
}

// Lookup returns the token only if currently valid at now. Invalid/expired/
// failed tokens return (nil, false) — never a usable credential (fail-closed).
func (s *CallbackTokenStore) Lookup(token string, now time.Time) (*CallbackToken, bool) {
	if s == nil || s.db == nil {
		return nil, false
	}
	var (
		userID string
		scopes string
		expMS  int64
		fail   string
		deg    string
	)
	err := s.db.QueryRow(
		`SELECT user_id, scopes, expires_at, failure, degrade
		 FROM callback_token WHERE token = ?`, token).
		Scan(&userID, &scopes, &expMS, &fail, &deg)
	if err != nil {
		return nil, false
	}
	if fail != "" {
		return nil, false
	}
	expiresAt := time.UnixMilli(expMS)
	if !now.Before(expiresAt) {
		return nil, false
	}
	var scopeList []string
	_ = json.Unmarshal([]byte(scopes), &scopeList)
	return &CallbackToken{
		Token:     token,
		UserID:    userID,
		Scopes:    scopeList,
		ExpiresAt: expiresAt,
		Failure:   AuthFailureReason(fail),
		Degrade:   DegradePolicy(deg),
	}, true
}

// MarkFailure records a failure reason for the token directly on disk
// (persistent tombstone). A token with a failure reason is never returned by
// Lookup (fail-closed). Unknown token → silently no-op (no panic).
func (s *CallbackTokenStore) MarkFailure(token string, r AuthFailureReason) {
	if s == nil || s.db == nil {
		return
	}
	_, _ = s.db.Exec(
		`UPDATE callback_token SET failure = ? WHERE token = ?`,
		string(r), token)
}

// Refresh extends the token's expiry on disk and clears any failure reason,
// restoring validity across restarts (续期落盘). Unknown token → no-op.
func (s *CallbackTokenStore) Refresh(token string, expiresAt time.Time) {
	if s == nil || s.db == nil {
		return
	}
	_, _ = s.db.Exec(
		`UPDATE callback_token SET expires_at = ?, failure = '' WHERE token = ?`,
		expiresAt.UnixMilli(), token)
}

// Revoke is an explicit tombstone: marks the token revoked (fail-closed deny).
func (s *CallbackTokenStore) Revoke(token string) {
	s.MarkFailure(token, ReasonRevoked)
}

// Close releases the underlying database.
func (s *CallbackTokenStore) Close() {
	if s != nil && s.db != nil {
		_ = s.db.Close()
	}
}
