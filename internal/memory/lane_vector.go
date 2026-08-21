package memory

import (
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (shared with lane persister)
)

// vectorStore persists dense embeddings in a dedicated SQLite file (path +
// ".vec.db") keyed by entry_id (whole-entry vector) and passage_key (passage
// vectors). Similarity search is exact cosine computed in Go over the stored
// vectors. The hybrid RRF fusion that combines these
// vectors with BM25 lexical recall lives in lane_hybrid.go.
type vectorStore struct {
	db *sql.DB
}

func openVectorDB(path string) (*vectorStore, error) {
	if path == "" {
		return nil, errVectorEmpty
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+".vec.db")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(3)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lane_entry_vec (
		entry_id TEXT PRIMARY KEY, vec BLOB)`); err != nil {
		db.Close()
		return nil, err
	}
	// Passage-level vectors: an entry is chunked and
	// each chunk embedded, so a recall can match a sub-section, not just the
	// whole entry.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lane_passage_vec (
		passage_key TEXT PRIMARY KEY, entry_id TEXT, passage TEXT, vec BLOB)`); err != nil {
		db.Close()
		return nil, err
	}
	return &vectorStore{db: db}, nil
}

var errVectorEmpty = errors.New("memory vector: empty path")

func encodeVec(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func decodeVec(buf []byte) []float32 {
	if len(buf)%4 != 0 {
		return nil
	}
	n := len(buf) / 4
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec
}

func cosine(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

func (v *vectorStore) upsert(id string, vec []float32) error {
	_, err := v.db.Exec("INSERT OR REPLACE INTO lane_entry_vec (entry_id, vec) VALUES (?, ?)", id, encodeVec(vec))
	return err
}

func (v *vectorStore) upsertPassage(entryID, key, passage string, vec []float32) error {
	_, err := v.db.Exec("INSERT OR REPLACE INTO lane_passage_vec (passage_key, entry_id, passage, vec) VALUES (?, ?, ?, ?)", key, entryID, passage, encodeVec(vec))
	return err
}

// entrySim returns entry_id → best cosine to q (descending ranked list).
func (v *vectorStore) entrySim(q []float32) []vecHit {
	rows, err := v.db.Query("SELECT entry_id, vec FROM lane_entry_vec")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var hits []vecHit
	for rows.Next() {
		var id string
		var buf []byte
		if err := rows.Scan(&id, &buf); err != nil {
			return hits
		}
		vec := decodeVec(buf)
		if vec == nil {
			continue
		}
		hits = append(hits, vecHit{id: id, sim: cosine(q, vec)})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })
	return hits
}

// passageSim returns entry_id → best passage cosine to q (descending ranked).
func (v *vectorStore) passageSim(q []float32) []vecHit {
	rows, err := v.db.Query("SELECT entry_id, vec FROM lane_passage_vec")
	if err != nil {
		return nil
	}
	defer rows.Close()
	best := map[string]float32{}
	for rows.Next() {
		var id string
		var buf []byte
		if err := rows.Scan(&id, &buf); err != nil {
			return nil
		}
		vec := decodeVec(buf)
		if vec == nil {
			continue
		}
		s := cosine(q, vec)
		if _, ok := best[id]; !ok || s > best[id] {
			best[id] = s
		}
	}
	var hits []vecHit
	for id, s := range best {
		hits = append(hits, vecHit{id: id, sim: s})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].sim > hits[j].sim })
	return hits
}

type vecHit struct {
	id  string
	sim float32
}

func (v *vectorStore) close() { _ = v.db.Close() }
