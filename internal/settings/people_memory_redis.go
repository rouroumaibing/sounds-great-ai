package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisPeopleMemoryStore is the operator-partitioned
// store. It activates only when a Redis URL is configured (SG_REDIS_URL); the
// file store remains the zero-dependency default. Every operator's document is
// a JSON blob at pm:{op}:doc; the deferred-receipt dual path uses atomic Lua
// (pm:{op}:rcpt:{id} hash + pm:{op}:ready zset) — the exact high-contention
// region guarded with Lua. Lower-contention propose/approve/recall paths
// reuse the shared document methods and persist the operator doc atomically.
//
// Receipts are stored as flat hashes (not JSON blobs) so the Lua scripts need
// no cjson and run identically under real Redis and miniredis in tests.

const redisPeoplePrefix = "pm:"

// Lua scripts guard the deferred-receipt lifecycle atomically (mirrors the
// DEFERRED_RECEIPT_STAGE_LUA / CLAIM / WITHDRAW / FORGET). They avoid cjson.
var (
	luaStageReceipt = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then return 'EXISTS' end
redis.call('HSET', KEYS[1], 'receipt_id', ARGV[1], 'owner_user_id', ARGV[2], 'subject', ARGV[3], 'person_id', ARGV[4], 'created_at', ARGV[5], 'requester_dog', ARGV[6], 'claimed_at', '0', 'claim_id', '', 'withdrawn', '0')
redis.call('ZADD', KEYS[2], ARGV[5], ARGV[1])
return 'CREATED'`)

	luaClaimReceipt = redis.NewScript(`
local claimed = redis.call('HGET', KEYS[1], 'claimed_at')
if claimed == false then return 'NOT_FOUND' end
if claimed ~= '0' then return 'ALREADY_CLAIMED' end
if redis.call('HGET', KEYS[1], 'withdrawn') == '1' then return 'WITHDRAWN' end
redis.call('HSET', KEYS[1], 'claimed_at', ARGV[2], 'claim_id', ARGV[3])
redis.call('ZREM', KEYS[2], ARGV[1])
return 'CLAIMED'`)

	luaWithdrawReceipt = redis.NewScript(`
local claimed = redis.call('HGET', KEYS[1], 'claimed_at')
if claimed == false then return 'NOT_FOUND' end
if claimed ~= '0' then return 'ALREADY_CLAIMED' end
redis.call('HSET', KEYS[1], 'withdrawn', '1')
redis.call('ZREM', KEYS[2], ARGV[1])
return 'WITHDRAWN'`)

	luaForgetReceipt = redis.NewScript(`
local claimed = redis.call('HGET', KEYS[1], 'claimed_at')
if claimed == false then return 'NOT_FOUND' end
if claimed ~= '0' then return 'ALREADY_CLAIMED' end
redis.call('DEL', KEYS[1])
redis.call('DEL', KEYS[3])
redis.call('ZREM', KEYS[2], ARGV[1])
return 'FORGOTTEN'`)
)

// RedisPeopleMemoryStore implements PeopleMemoryStore over Redis.
type RedisPeopleMemoryStore struct {
	client *redis.Client
	mu     sync.Mutex
	cache  map[string]*peopleMemoryDocument // operatorID -> loaded doc (in-process cache)
	// drillBudgets is the ephemeral (operator, turn) -> spend map for on-demand
	// drill budgeting. Never persisted (see drillTurnBudget).
	drillBudgets map[string]*drillTurnBudget
}

// NewRedisPeopleMemoryStore builds a Redis-backed store.
func NewRedisPeopleMemoryStore(client *redis.Client) *RedisPeopleMemoryStore {
	return &RedisPeopleMemoryStore{client: client, cache: map[string]*peopleMemoryDocument{}, drillBudgets: map[string]*drillTurnBudget{}}
}

func (s *RedisPeopleMemoryStore) docKey(op string) string    { return redisPeoplePrefix + op + ":doc" }
func (s *RedisPeopleMemoryStore) rcptKey(op, id string) string { return redisPeoplePrefix + op + ":rcpt:" + id }
func (s *RedisPeopleMemoryStore) coordsKey(op, id string) string { return redisPeoplePrefix + op + ":rcpt:" + id + ":coords" }
func (s *RedisPeopleMemoryStore) readyKey(op string) string { return redisPeoplePrefix + op + ":ready" }

// opDoc loads (and caches) the operator doc from Redis, creating an empty one if
// absent. Caller MUST hold s.mu.
func (s *RedisPeopleMemoryStore) opDoc(op string) *peopleMemoryDocument {
	if op == "" {
		op = "operator"
	}
	if d, ok := s.cache[op]; ok {
		return d
	}
	raw, err := s.client.Get(context.Background(), s.docKey(op)).Result()
	d := newPeopleMemoryDocument()
	if err == nil && raw != "" {
		if jsonErr := json.Unmarshal([]byte(raw), d); jsonErr != nil {
			d = newPeopleMemoryDocument()
		}
	}
	s.cache[op] = d
	return d
}

func (s *RedisPeopleMemoryStore) persist(op string) error {
	d := s.cache[op]
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal people-memory doc: %w", err)
	}
	return s.client.Set(context.Background(), s.docKey(op), data, 0).Err()
}

// ListOperators finds every operator that holds data. An operator may have only
// deferred receipts (no doc written yet) — the daily clerk must still find them
// to promote ready receipts — so we scan both pm:*:doc and pm:*:ready keys.
func (s *RedisPeopleMemoryStore) ListOperators() ([]string, error) {
	seen := map[string]struct{}{}
	for _, pattern := range []string{redisPeoplePrefix + "*:doc", redisPeoplePrefix + "*:ready"} {
		iter := s.client.Scan(context.Background(), 0, pattern, 0).Iterator()
		for iter.Next(context.Background()) {
			key := iter.Val()
			op := strings.TrimPrefix(key, redisPeoplePrefix)
			op = strings.TrimSuffix(op, ":doc")
			op = strings.TrimSuffix(op, ":ready")
			if op != "" {
				seen[op] = struct{}{}
			}
		}
		if err := iter.Err(); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(seen))
	for op := range seen {
		out = append(out, op)
	}
	return out, nil
}

// ---- Dual-path deferred receipts (Lua-guarded) ----

func (s *RedisPeopleMemoryStore) DeferReceipt(operatorID, requesterDog, subject, personID string, coords []SourceRef) (*DeferredPersonMemoryReceipt, error) {
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("subject is required")
	}
	now := time.Now().UnixMilli()
	r := &DeferredPersonMemoryReceipt{
		ReceiptID:    "rcpt-" + newShortID(),
		OwnerUserID:  operatorID,
		RequesterDog: requesterDog,
		Subject:      subject,
		PersonID:     personID,
		SourceCoords: coords,
		CreatedAt:    now,
	}
	res, err := luaStageReceipt.Run(context.Background(), s.client,
		[]string{s.rcptKey(operatorID, r.ReceiptID), s.readyKey(operatorID)},
		r.ReceiptID, operatorID, subject, personID, strconv.FormatInt(now, 10), requesterDog).Text()
	if err != nil {
		return nil, err
	}
	if res == "EXISTS" {
		return nil, fmt.Errorf("receipt %q already exists", r.ReceiptID)
	}
	// Persist the exact source coordinates separately (Lua stays cjson-free).
	if len(coords) > 0 {
		if cb, jerr := json.Marshal(coords); jerr == nil {
			_ = s.client.Set(context.Background(), s.coordsKey(operatorID, r.ReceiptID), cb, 0).Err()
		}
	}
	return r, nil
}

func (s *RedisPeopleMemoryStore) ListReadyDeferred(operatorID string) ([]*DeferredPersonMemoryReceipt, error) {
	ids, err := s.client.ZRange(context.Background(), s.readyKey(operatorID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*DeferredPersonMemoryReceipt, 0, len(ids))
	for _, id := range ids {
		r, ok := s.readReceipt(operatorID, id)
		if !ok {
			continue
		}
		if !r.Withdrawn && r.ClaimedAt == 0 {
			out = append(out, r)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out, nil
}

func (s *RedisPeopleMemoryStore) readReceipt(operatorID, id string) (*DeferredPersonMemoryReceipt, bool) {
	m, err := s.client.HGetAll(context.Background(), s.rcptKey(operatorID, id)).Result()
	if err != nil || len(m) == 0 {
		return nil, false
	}
	r := &DeferredPersonMemoryReceipt{
		ReceiptID:    m["receipt_id"],
		OwnerUserID:  m["owner_user_id"],
		Subject:      m["subject"],
		PersonID:     m["person_id"],
		RequesterDog: m["requester_dog"],
	}
	if v, e := strconv.ParseInt(m["created_at"], 10, 64); e == nil {
		r.CreatedAt = v
	}
	if v, e := strconv.ParseInt(m["claimed_at"], 10, 64); e == nil {
		r.ClaimedAt = v
	}
	r.ClaimID = m["claim_id"]
	r.Withdrawn = m["withdrawn"] == "1"
	if cb, e := s.client.Get(context.Background(), s.coordsKey(operatorID, id)).Result(); e == nil && cb != "" {
		_ = json.Unmarshal([]byte(cb), &r.SourceCoords)
	}
	return r, true
}

func (s *RedisPeopleMemoryStore) ClaimDeferredReceipt(operatorID, receiptID, requesterDog string) (*CaptureCandidate, error) {
	claimID := "cand-" + newShortID()
	res, err := luaClaimReceipt.Run(context.Background(), s.client,
		[]string{s.rcptKey(operatorID, receiptID), s.readyKey(operatorID)},
		receiptID, time.Now().UnixMilli(), claimID).Text()
	if err != nil {
		return nil, err
	}
	switch res {
	case "NOT_FOUND":
		return nil, fmt.Errorf("receipt %q not found", receiptID)
	case "WITHDRAWN":
		return nil, fmt.Errorf("receipt %q was withdrawn", receiptID)
	case "ALREADY_CLAIMED":
		return nil, fmt.Errorf("receipt %q already claimed", receiptID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.opDoc(operatorID)
	r, ok := s.readReceipt(operatorID, receiptID)
	if !ok {
		return nil, fmt.Errorf("receipt %q vanished after claim", receiptID)
	}
	now := time.Now().UnixMilli()
	c := &CaptureCandidate{
		CandidateID:       claimID,
		RequesterDog:      requesterDog,
		SourceMessageRef:  firstOrEmptySource(r.SourceCoords),
		TargetPersonID:    r.PersonID,
		State:             CandPendingApproval,
		PresentedAt:       now,
		CreatedAt:         now,
		DeferredReceiptID: r.ReceiptID,
	}
	d.Candidates[c.CandidateID] = c
	d.reindexPending()
	if err := s.persist(operatorID); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *RedisPeopleMemoryStore) WithdrawReceipt(operatorID, receiptID string) error {
	res, err := luaWithdrawReceipt.Run(context.Background(), s.client,
		[]string{s.rcptKey(operatorID, receiptID), s.readyKey(operatorID)}, receiptID).Text()
	if err != nil {
		return err
	}
	switch res {
	case "NOT_FOUND":
		return fmt.Errorf("receipt %q not found", receiptID)
	case "ALREADY_CLAIMED":
		return fmt.Errorf("receipt %q already claimed into a candidate; withdraw the candidate instead", receiptID)
	}
	return nil
}

func (s *RedisPeopleMemoryStore) ForgetReceipt(operatorID, receiptID string) error {
	res, err := luaForgetReceipt.Run(context.Background(), s.client,
		[]string{s.rcptKey(operatorID, receiptID), s.readyKey(operatorID), s.coordsKey(operatorID, receiptID)},
		receiptID).Text()
	if err != nil {
		return err
	}
	switch res {
	case "NOT_FOUND":
		return fmt.Errorf("receipt %q not found", receiptID)
	case "ALREADY_CLAIMED":
		return fmt.Errorf("receipt %q already claimed; forget the candidate instead", receiptID)
	}
	return nil
}

// ReserveDeferredReceipt marks a receipt claimed (claimed_at + claim_id) without
// creating a staged candidate, and removes it from the ready queue. Mirrors the
// file store's reserveDeferredReceipt; the authoritative receipt state lives in
// the receipt hash (HSet) and the ready zset, consistent with the coords writes.
func (s *RedisPeopleMemoryStore) ReserveDeferredReceipt(operatorID, receiptID, by string) error {
	r, ok := s.readReceipt(operatorID, receiptID)
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	if r.Withdrawn {
		return fmt.Errorf("receipt %q was withdrawn", receiptID)
	}
	if r.ClaimedAt != 0 {
		return fmt.Errorf("receipt %q already claimed", receiptID)
	}
	now := time.Now().UnixMilli()
	if err := s.client.HSet(context.Background(), s.rcptKey(operatorID, receiptID),
		"claimed_at", strconv.FormatInt(now, 10), "claim_id", "reserved:"+by).Err(); err != nil {
		return err
	}
	if err := s.client.ZRem(context.Background(), s.readyKey(operatorID), receiptID).Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.cache[operatorID]; ok {
		if cr, ok := d.Receipts[receiptID]; ok {
			cr.ClaimedAt = now
			cr.ClaimID = "reserved:" + by
		}
	}
	return nil
}

// ReleaseDeferredReceipt clears a reservation so the receipt becomes ready again.
func (s *RedisPeopleMemoryStore) ReleaseDeferredReceipt(operatorID, receiptID string) error {
	r, ok := s.readReceipt(operatorID, receiptID)
	if !ok {
		return fmt.Errorf("receipt %q not found", receiptID)
	}
	if err := s.client.HSet(context.Background(), s.rcptKey(operatorID, receiptID),
		"claimed_at", "0", "claim_id", "").Err(); err != nil {
		return err
	}
	if !r.Withdrawn {
		if err := s.client.ZAdd(context.Background(), s.readyKey(operatorID),
			redis.Z{Score: float64(time.Now().UnixMilli()), Member: receiptID}).Err(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.cache[operatorID]; ok {
		if cr, ok := d.Receipts[receiptID]; ok {
			cr.ClaimedAt = 0
			cr.ClaimID = ""
		}
	}
	return nil
}
