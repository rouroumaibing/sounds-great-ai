package transport

import (
	"sounds-great-ai/internal/telemetry"
)

// buildHandoffSourceNotice returns a short system note telling the next dog who
// handed it the ball (G7 step 1: context-transport.a2aFrom / a2aTriggerMessageId
// parity). The receiving dog otherwise has no idea it was @-summoned mid-chain.
func buildHandoffSourceNotice(fromBreed string) string {
	if fromBreed == "" {
		return ""
	}
	return "[系统] 你被 @" + fromBreed + " 通过交接(handoff)调用，请基于其输出继续推进任务。若需要原始请求上下文，请向上追溯对话历史。"
}

// scrubHandoffContext removes sensitive payloads from a handoff artifact before
// it crosses the breed boundary (G7 step 2). It reuses telemetry.RedactorInstance
// (the same HMAC pseudonymizer used for span attributes) plus provider-token
// pattern masking, so credentials never leak verbatim into the next dog's prompt.
func scrubHandoffContext(artifact string) string {
	return telemetry.RedactSecrets(artifact)
}
