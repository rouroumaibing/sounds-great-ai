package router

import "strings"

func (e *RoutingEngine) matchRules(task string) *RoutingDecision {
	taskLower := strings.ToLower(task)
	for _, rule := range e.rules {
		if strings.Contains(taskLower, rule.TaskType) {
			steps := make([]RoutingStep, 0)
			if len(rule.Flow) > 0 {
				for i, role := range rule.Flow {
					dep := []int{}
					if i > 0 {
						dep = []int{i - 1}
					}
					steps = append(steps, RoutingStep{Role: role, Skills: rule.Skills, DependsOn: dep})
				}
			} else {
				for _, role := range rule.AssignRoles {
					steps = append(steps, RoutingStep{Role: role, Skills: rule.Skills})
				}
			}
			return &RoutingDecision{Plan: steps, Reason: "rule: " + rule.TaskType}
		}
	}
	return nil
}
