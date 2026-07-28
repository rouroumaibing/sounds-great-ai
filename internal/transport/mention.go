package transport

import (
	"regexp"

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
