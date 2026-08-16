package transport

// buildHandoffSourceNotice returns a short system note telling the next dog who
// handed it the ball (G7 step 1 / context-transport.a2aFrom parity). The
// receiving dog otherwise has no idea it was @-summoned mid-chain. It is one
// layer of the G12 enriched handoff context (see buildEnrichedHandoffContext).
func buildHandoffSourceNotice(fromBreed string) string {
	if fromBreed == "" {
		return ""
	}
	return "[系统] 你被 @" + fromBreed + " 通过交接(handoff)调用，请基于其输出继续推进任务。若需要原始请求上下文，请向上追溯对话历史。"
}
