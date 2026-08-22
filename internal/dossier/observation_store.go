package dossier

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteObservationStore implements ObservationStore backed by SQLite.
// Observations are permanent team state (TTL=0 semantics — no cleanup), so
// they live in the main SG database alongside threads/messages rather than
// in an ephemeral store.
type sqliteObservationStore struct {
	db *sql.DB
}

// NewSQLiteObservationStore opens (creating if needed) the observations
// schema in the SQLite database at path.
func NewSQLiteObservationStore(path string) (ObservationStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sqlite dir %q: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS dossier_observations (
		id TEXT PRIMARY KEY,
		dog_id TEXT NOT NULL,
		content TEXT NOT NULL,
		provenance_type TEXT NOT NULL,
		provenance_author TEXT NOT NULL,
		provenance_date TEXT NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_dossier_obs_dog ON dossier_observations(dog_id, created_at);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create dossier_observations schema: %w", err)
	}
	return &sqliteObservationStore{db: db}, nil
}

// Add records one observation with operator provenance.
func (s *sqliteObservationStore) Add(input AddObservationInput) (Observation, error) {
	now := time.Now()
	obs := Observation{
		ID:      "obs_" + now.Format("20060102150405") + "_" + fmt.Sprintf("%d", now.UnixNano()%1e9),
		DogID:   input.DogID,
		Content: input.Content,
		Provenance: ObservationProvenance{
			Type:   "operator",
			Author: input.Author,
			Date:   now.Format("2006-01-02"),
		},
		CreatedAt: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO dossier_observations (id, dog_id, content, provenance_type, provenance_author, provenance_date, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		obs.ID, obs.DogID, obs.Content, obs.Provenance.Type, obs.Provenance.Author, obs.Provenance.Date, obs.CreatedAt.UnixMilli(),
	)
	if err != nil {
		return Observation{}, fmt.Errorf("insert observation: %w", err)
	}
	return obs, nil
}

// List returns observations for one dog, newest first.
func (s *sqliteObservationStore) List(dogID string, limit int) ([]Observation, error) {
	limit = clampLimit(limit)
	rows, err := s.db.Query(
		`SELECT id, dog_id, content, provenance_type, provenance_author, provenance_date, created_at
		 FROM dossier_observations WHERE dog_id = ? ORDER BY created_at DESC LIMIT ?`, dogID, limit)
	if err != nil {
		return nil, fmt.Errorf("list observations: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// ListAll returns observations grouped by dog, newest first per dog.
func (s *sqliteObservationStore) ListAll(limit int) (map[string][]Observation, error) {
	limit = clampLimit(limit)
	rows, err := s.db.Query(
		`SELECT id, dog_id, content, provenance_type, provenance_author, provenance_date, created_at
		 FROM dossier_observations ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all observations: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string][]Observation)
	counts := make(map[string]int)
	for rows.Next() {
		obs, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		if counts[obs.DogID] >= limit {
			continue
		}
		counts[obs.DogID]++
		grouped[obs.DogID] = append(grouped[obs.DogID], obs)
	}
	return grouped, rows.Err()
}

// Get returns a single observation.
func (s *sqliteObservationStore) Get(id string) (Observation, bool, error) {
	rows, err := s.db.Query(
		`SELECT id, dog_id, content, provenance_type, provenance_author, provenance_date, created_at
		 FROM dossier_observations WHERE id = ?`, id)
	if err != nil {
		return Observation{}, false, fmt.Errorf("get observation: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return Observation{}, false, rows.Err()
	}
	obs, err := scanObservation(rows)
	return obs, err == nil, err
}

func scanObservation(rows interface{ Scan(dest ...any) error }) (Observation, error) {
	var obs Observation
	var createdMs int64
	if err := rows.Scan(&obs.ID, &obs.DogID, &obs.Content, &obs.Provenance.Type, &obs.Provenance.Author, &obs.Provenance.Date, &createdMs); err != nil {
		return Observation{}, fmt.Errorf("scan observation: %w", err)
	}
	obs.CreatedAt = time.UnixMilli(createdMs)
	return obs, nil
}

func scanObservations(rows *sql.Rows) ([]Observation, error) {
	var out []Observation
	for rows.Next() {
		obs, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, rows.Err()
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > 200 {
		return 100
	}
	return limit
}
