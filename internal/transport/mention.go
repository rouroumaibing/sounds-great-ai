package transport

import (
	"regexp"
	"strings"

	"sounds-great-ai/pkg/pack"
)

var mentionRegex = regexp.MustCompile(`(?:^|\s)@([a-zA-Z0-9_-]+)`)

// parseMention 从消息中提取 @breedID，未匹配或未注册则返回 "bianmu"
func parseMention(msg string, p *pack.Pack) string {
	matches := mentionRegex.FindAllStringSubmatch(msg, -1)
	for _, m := range matches {
		breedID := m[1]
		if p.HasBreed(breedID) {
			return breedID
		}
	}
	return "bianmu"
}

// isLeaderMention returns true if the message starts with one of the leader's
// mention patterns (e.g. "@leader do something"). This ensures leader messages
// are attributed as human user messages (catId=null) rather than breed-routed.
func isLeaderMention(text string, patterns []string) bool {
	trimmed := strings.TrimSpace(text)
	for _, p := range patterns {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}
