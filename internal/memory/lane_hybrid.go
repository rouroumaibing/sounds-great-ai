package memory

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// ---- lexical BM25-style scorer (hybrid RRF lexical half) ----

var queryTokenRe = regexp.MustCompile(`[a-zA-Z0-9]+|[\p{Han}]`)

func tokenizeQuery(s string) []string {
	low := strings.ToLower(s)
	return queryTokenRe.FindAllString(low, -1)
}

// lexicalScores returns entry_id → TF*IDF score over the visible corpus. CJK
// is scored per-character (weighted higher).
func lexicalScores(query string, entries []*LaneEntry) map[string]float64 {
	toks := tokenizeQuery(query)
	if len(toks) == 0 {
		return nil
	}
	n := float64(len(entries))
	df := map[string]int{}
	for _, e := range entries {
		seen := map[string]bool{}
		low := strings.ToLower(e.Content)
		for _, t := range toks {
			if !seen[t] && strings.Contains(low, t) {
				seen[t] = true
				df[t]++
			}
		}
	}
	score := map[string]float64{}
	for _, e := range entries {
		low := strings.ToLower(e.Content)
		var s float64
		for _, t := range toks {
			idf := math.Log((n + 1) / (float64(df[t]) + 1))
			w := 1.0
			if len([]rune(t)) == 1 { // CJK single char → up-weight
				w = 1.5
			}
			s += float64(strings.Count(low, t)) * idf * w
		}
		if s > 0 {
			score[e.ID] = s
		}
	}
	return score
}

// rrf fuses ranked ID lists with k=60. Ties share
// the average rank.
func rrf(lists ...[]string) map[string]float64 {
	const k = 60
	out := map[string]float64{}
	for _, list := range lists {
		for i, id := range list {
			out[id] += 1.0 / float64(k+i)
		}
	}
	return out
}

func rankIDs(hits []vecHit) []string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.id)
	}
	return ids
}

// ---- LaneRegistry vector API (Gap3 + P1 maturity) ----

// SetEmbedMode sets the embedding mode (off/shadow/on). off → no vectors used (lexical only); shadow → vectors
// are stored/refreshed but ranking stays lexical; on → hybrid RRF (default when
// an embedder is configured).
func (r *LaneRegistry) SetEmbedMode(mode string) {
	switch mode {
	case "off", "shadow", "on":
		r.embedMode = mode
	default:
		if r.vector != nil {
			r.embedMode = "on"
		} else {
			r.embedMode = "off"
		}
	}
}

// StoreVector records the whole-entry embedding for an entry.
func (r *LaneRegistry) StoreVector(id string, vec []float32) error {
	if r.vector == nil {
		return errVectorEmpty
	}
	return r.vector.upsert(id, vec)
}

// StorePassages chunks content and embeds each passage (passage-level recall).
func (r *LaneRegistry) StorePassages(id, content string, embed func(string) ([]float32, error)) error {
	if r.vector == nil || embed == nil {
		return errVectorEmpty
	}
	// Drop stale passages for this entry, then (re)embed current chunks.
	_, _ = r.vector.db.Exec("DELETE FROM lane_passage_vec WHERE entry_id = ?", id)
	for i, p := range splitPassages(content) {
		vec, err := embed(p)
		if err != nil {
			continue
		}
		if err := r.vector.upsertPassage(id, id+"#"+itoa(i), p, vec); err != nil {
			return err
		}
	}
	return nil
}

func splitPassages(s string) []string {
	raw := regexp.MustCompile(`[\n。！？.!?]+`).Split(s, -1)
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if len([]rune(p)) >= 4 {
			out = append(out, p)
		}
	}
	if len(out) == 0 && len([]rune(strings.TrimSpace(s))) >= 4 {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// HybridSearch returns approved entries ranked by hybrid RRF of (entry NN,
// passage NN, BM25 lexical). embedMode
// gates how NN contributes: off → lexical only; shadow → NN computed/stored
// but ranking stays lexical; on → full fusion. Returns (nil, false) when no
// hits or the vector store is unavailable and the query is non-lexical.
func (r *LaneRegistry) HybridSearch(query string, qVec []float32, topK int, operator string) ([]*LaneEntry, bool) {
	if topK <= 0 {
		topK = 10
	}
	// Corpus = visible approved truth.
	var corpus []*LaneEntry
	for _, t := range r.LaneTypes() {
		for _, e := range r.Lane(t).Truth() {
			if EntryVisible(e, operator) {
				corpus = append(corpus, e)
			}
		}
	}
	if len(corpus) == 0 {
		return nil, false
	}
	lexical := lexicalScores(query, corpus)
	useNN := r.vector != nil && qVec != nil && r.embedMode != "off"

	if !useNN && len(lexical) == 0 {
		return nil, false
	}

	var fused map[string]float64
	if useNN && r.embedMode == "on" {
		entryRank := rankIDs(r.vector.entrySim(qVec))
		passRank := rankIDs(r.vector.passageSim(qVec))
		lexRank := rankByScore(lexical)
		fused = rrf(entryRank, passRank, lexRank)
	} else {
		// off / shadow → lexical-only ranking (NN still refreshed on reindex).
		fused = rankScoreMap(lexical)
	}

	type scored struct {
		id string
		s  float64
	}
	order := make([]scored, 0, len(fused))
	for id, s := range fused {
		order = append(order, scored{id, s})
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].s > order[j].s })

	out := make([]*LaneEntry, 0, topK)
	for _, sc := range order {
		if laneType, ok := r.FindLaneOf(sc.id); ok {
			if e, ok := r.Lane(laneType).Get(sc.id); ok && EntryVisible(e, operator) {
				out = append(out, e)
			}
		}
		if len(out) >= topK {
			break
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func rankByScore(m map[string]float64) []string {
	type kv struct {
		id string
		s  float64
	}
	ks := make([]kv, 0, len(m))
	for id, s := range m {
		ks = append(ks, kv{id, s})
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].s > ks[j].s })
	ids := make([]string, len(ks))
	for i, k := range ks {
		ids[i] = k.id
	}
	return ids
}

func rankScoreMap(m map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for id, s := range m {
		out[id] = s
	}
	return out
}

// SemanticSearch returns the most similar approved entries to q by whole-entry
// cosine only (NN half of the hybrid). Kept for NN-only callers / tests.
func (r *LaneRegistry) SemanticSearch(q []float32, topK int, operator string) ([]*LaneEntry, bool) {
	if r.vector == nil {
		return nil, false
	}
	ids := rankIDs(r.vector.entrySim(q))
	if len(ids) == 0 {
		return nil, false
	}
	out := make([]*LaneEntry, 0, topK)
	for _, id := range ids {
		if laneType, ok := r.FindLaneOf(id); ok {
			if e, ok := r.Lane(laneType).Get(id); ok && EntryVisible(e, operator) {
				out = append(out, e)
			}
		}
		if topK > 0 && len(out) >= topK {
			break
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}
