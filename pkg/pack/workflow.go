package pack

import (
	"context"
	"fmt"
	"sync"
)

// Bark 执行指定 breed 的 workflow DAG
func (p *Pack) Bark(ctx context.Context, breedID string, input *TaskInput) (*TaskOutput, error) {
	p.mu.RLock()
	breed, ok := p.registry[breedID]
	p.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("breed %q not found", breedID)
	}
	input.Breed = breed
	return p.executeWorkflow(ctx, &breed.Workflow, breed.Capabilities, input)
}

// executeWorkflow 按拓扑排序执行 workflow，同层并行，跨层串行
func (p *Pack) executeWorkflow(ctx context.Context, wf *WorkflowConfig, bindings []CapabilityBinding, input *TaskInput) (*TaskOutput, error) {
	// 1. 拓扑排序 + 循环检测
	layers, err := topologicalSort(wf.Steps)
	if err != nil {
		return nil, err
	}

	// 构建 capability_ref -> config 映射
	configMap := make(map[string]map[string]any)
	for _, binding := range bindings {
		key := fmt.Sprintf("%s:%s", binding.Name, binding.Version)
		configMap[key] = binding.Config
	}

	// 2. 按层执行，同层并行
	stepResults := make(map[string]*TaskOutput)
	for _, layer := range layers {
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error

		for _, step := range layer {
			wg.Add(1)
			go func(s WorkflowStep) {
				defer wg.Done()

				p.mu.RLock()
				cap, ok := p.capabilities[s.CapabilityRef]
				p.mu.RUnlock()
				if !ok {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("capability %q not found", s.CapabilityRef)
					}
					mu.Unlock()
					return
				}

				stepInput := *input
				stepInput.Previous = make(map[string]*TaskOutput)
				mu.Lock()
				for _, dep := range s.Depends {
					stepInput.Previous[dep] = stepResults[dep]
				}
				mu.Unlock()
				stepInput.CapabilityConfig = configMap[s.CapabilityRef]

				out, err := cap.Run(ctx, &stepInput)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				stepResults[s.ID] = out
				mu.Unlock()
			}(step)
		}
		wg.Wait()
		if firstErr != nil {
			return nil, firstErr
		}
	}

	// 3. 构建最终结果
	final := &TaskOutput{Data: make(map[string]any)}
	final.Data["steps"] = stepResults
	return final, nil
}

// topologicalSort 使用 Kahn 算法分层拓扑排序，检测循环依赖
func topologicalSort(steps []WorkflowStep) ([][]WorkflowStep, error) {
	stepMap := make(map[string]WorkflowStep)
	for _, step := range steps {
		stepMap[step.ID] = step
	}

	// 校验所有依赖存在
	for _, step := range steps {
		for _, dep := range step.Depends {
			if _, ok := stepMap[dep]; !ok {
				return nil, fmt.Errorf("step %q depends on unknown step %q", step.ID, dep)
			}
		}
	}

	var layers [][]WorkflowStep
	processed := make(map[string]bool)

	for len(processed) < len(steps) {
		var layer []WorkflowStep
		for _, step := range steps {
			if processed[step.ID] {
				continue
			}
			allDepsProcessed := true
			for _, dep := range step.Depends {
				if !processed[dep] {
					allDepsProcessed = false
					break
				}
			}
			if allDepsProcessed {
				layer = append(layer, step)
			}
		}
		if len(layer) == 0 {
			return nil, fmt.Errorf("cycle detected in workflow steps")
		}
		for _, step := range layer {
			processed[step.ID] = true
		}
		layers = append(layers, layer)
	}

	return layers, nil
}
