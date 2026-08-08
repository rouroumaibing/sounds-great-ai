package pack

import (
	"context"
	"errors"
	"sync"

	"sounds-great-ai/pkg/pack/orchestrator"
)

// shallowCopyOutput returns a shallow copy of o. The returned TaskOutput has
// its own Data map and Results slice header, so adding/removing keys or
// re-slicing on the copy does not affect src. Values inside Data are shared
// by reference — this is safe under the read-only contract documented on
// TaskOutput.Control (capabilities must not mutate upstream Data values).
//
// Used by ExecuteDispatch to inject upstream results into downstream
// subInput.Previous without risking pointer-shared mutation across
// concurrent dependents.
func (p *Pack) shallowCopyOutput(o *TaskOutput) *TaskOutput {
	if o == nil {
		return nil
	}
	cp := *o
	if o.Data != nil {
		cp.Data = make(map[string]any, len(o.Data))
		for k, v := range o.Data {
			cp.Data[k] = v
		}
	}
	if o.Results != nil {
		cp.Results = append([]any(nil), o.Results...)
	}
	return &cp
}

// ErrMaxDepthExceeded is returned by ExecuteDispatch when the recursion
// depth counter exceeds plan.MaxDepth. Primary recursion protection is
// proactive (breeds with dispatch_execute capability are blocked before
// Bark); this is secondary belt-and-suspenders.
var ErrMaxDepthExceeded = errors.New("pack: dispatch max depth exceeded")

// BreedExecutor is the interface dispatch_execute depends on. *Pack
// implements it; tests can inject mocks.
type BreedExecutor interface {
	ExecuteDispatch(ctx context.Context, plan orchestrator.DispatchPlan, subtasks []orchestrator.SubTask) (results map[string]*TaskOutput, entryErrors map[string]string, err error)
}

// dispatchDepthKey is the context key for recursion depth counting.
type dispatchDepthKey struct{}

// ExecuteDispatch runs the pending entries of plan concurrently, respecting
// DependsOn topological layering.
//
//   - err: non-nil only for whole-call failures (MaxDepth, context cancelled).
//   - entryErrors: per-entry failures; does NOT set err (degradable).
//   - results: keyed by SubTaskID (NOT BreedID) to avoid same-breed races.
func (p *Pack) ExecuteDispatch(ctx context.Context, plan orchestrator.DispatchPlan, subtasks []orchestrator.SubTask) (results map[string]*TaskOutput, entryErrors map[string]string, err error) {
	results = make(map[string]*TaskOutput)
	entryErrors = make(map[string]string)

	// Depth check (secondary guard)
	depth := 0
	if v := ctx.Value(dispatchDepthKey{}); v != nil {
		if d, ok := v.(int); ok {
			depth = d
		}
	}
	if depth > plan.MaxDepth {
		return nil, nil, ErrMaxDepthExceeded
	}

	// Build subtask lookup
	subtaskByID := make(map[string]orchestrator.SubTask, len(subtasks))
	for _, st := range subtasks {
		subtaskByID[st.ID] = st
	}

	// Filter pending entries + proactive recursion guard.
	// Breeds whose capability list contains "dispatch_execute" are blocked
	// before Bark to prevent runaway recursion.
	var pending []orchestrator.DispatchEntry
	for _, entry := range plan.Entries {
		if entry.Status != "pending" {
			continue
		}
		p.mu.RLock()
		_, breedExists := p.registry[entry.BreedID]
		p.mu.RUnlock()
		_ = breedExists // breed exists check; capabilities removed in variant format
		pending = append(pending, entry)
	}

	// Topological layering (Kahn) over pending entries' DependsOn.
	// Deps on non-pending entries (skipped/already-decided) are treated as satisfied.
	layers := topoLayers(pending)

	// Increment depth for any nested Bark (belt-and-suspenders)
	childCtx := context.WithValue(ctx, dispatchDepthKey{}, depth+1)

	for _, layer := range layers {
		var wg sync.WaitGroup
		var mu sync.Mutex // guards results + entryErrors
		for _, entry := range layer {
			wg.Add(1)
			go func(entry orchestrator.DispatchEntry) {
				defer wg.Done()
				runSingleEntry(childCtx, p, entry, subtaskByID, results, entryErrors, &mu)
			}(entry)
		}
		wg.Wait()
	}

	return results, entryErrors, nil
}

// runSingleEntry runs one entry's Bark and stores its result/error under lock.
func runSingleEntry(ctx context.Context, p *Pack, entry orchestrator.DispatchEntry, subtaskByID map[string]orchestrator.SubTask, results map[string]*TaskOutput, entryErrors map[string]string, mu *sync.Mutex) {
	st := subtaskByID[entry.SubTaskID]
	subInput := &TaskInput{
		Query:    st.Description,
		Previous: map[string]*TaskOutput{},
	}
	// Inject shallow-copied upstream results
	for _, depID := range entry.DependsOn {
		mu.Lock()
		upstream := results[depID]
		mu.Unlock()
		if upstream != nil {
			subInput.Previous[depID] = p.shallowCopyOutput(upstream)
		}
	}

	out, err := p.Bark(ctx, entry.BreedID, subInput)
	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		entryErrors[entry.SubTaskID] = err.Error()
		return
	}
	results[entry.SubTaskID] = out
}

// topoLayers groups entries into dependency-ordered layers using Kahn's
// algorithm. Entries in the same layer have no inter-dependencies and may
// run concurrently. Deps on entries NOT in the pending set are treated as
// already satisfied.
func topoLayers(entries []orchestrator.DispatchEntry) [][]orchestrator.DispatchEntry {
	pendingSet := make(map[string]bool, len(entries))
	for _, e := range entries {
		pendingSet[e.SubTaskID] = true
	}

	inDegree := make(map[string]int, len(entries))
	dependents := make(map[string][]orchestrator.DispatchEntry)
	for _, e := range entries {
		inDegree[e.SubTaskID] = 0
	}
	for _, e := range entries {
		for _, dep := range e.DependsOn {
			if pendingSet[dep] { // only count deps within pending set
				inDegree[e.SubTaskID]++
				dependents[dep] = append(dependents[dep], e)
			}
		}
	}

	var layers [][]orchestrator.DispatchEntry
	for {
		var layer []orchestrator.DispatchEntry
		for _, e := range entries {
			if inDegree[e.SubTaskID] == 0 {
				layer = append(layer, e)
				inDegree[e.SubTaskID] = -1 // mark processed
			}
		}
		if len(layer) == 0 {
			break
		}
		layers = append(layers, layer)
		for _, e := range layer {
			for _, dep := range dependents[e.SubTaskID] {
				inDegree[dep.SubTaskID]--
			}
		}
	}
	return layers
}
