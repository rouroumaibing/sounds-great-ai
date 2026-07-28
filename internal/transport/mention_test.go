package transport

import (
	"testing"

	"sounds-great-ai/pkg/pack"
)

func newMentionTestPack() *pack.Pack {
	p := pack.New("test")
	p.Register(&pack.BreedConfig{
		ID:           "xigou",
		Capabilities: []pack.CapabilityBinding{},
		Source:       pack.BreedSourceSystem,
	})
	p.Register(&pack.BreedConfig{
		ID:           "zhonghuatianyuanquan",
		Capabilities: []pack.CapabilityBinding{},
		Source:       pack.BreedSourceSystem,
	})
	p.Register(&pack.BreedConfig{
		ID:           "bianmu",
		Capabilities: []pack.CapabilityBinding{},
		Source:       pack.BreedSourceSystem,
	})
	return p
}

func TestParseMentionExact(t *testing.T) {
	p := newMentionTestPack()
	got := parseMention("@xigou 帮我搜索代码", p)
	if got != "xigou" {
		t.Errorf("got %q, want %q", got, "xigou")
	}
}

func TestParseMentionDefault(t *testing.T) {
	p := newMentionTestPack()
	got := parseMention("分析一下这个函数", p)
	if got != "bianmu" {
		t.Errorf("got %q, want %q", got, "bianmu")
	}
}

func TestParseMentionEmailNoMatch(t *testing.T) {
	p := newMentionTestPack()
	got := parseMention("我的邮箱是 test@xigou.com", p)
	if got != "bianmu" {
		t.Errorf("email should not match, got %q, want %q", got, "bianmu")
	}
}

func TestParseMentionUnknownBreedFallback(t *testing.T) {
	p := newMentionTestPack()
	got := parseMention("Hello @invalid_breed help me", p)
	if got != "bianmu" {
		t.Errorf("unknown breed should fallback, got %q, want %q", got, "bianmu")
	}
}

func TestParseMentionAtLineStart(t *testing.T) {
	p := newMentionTestPack()
	got := parseMention("@zhonghuatianyuanquan 检查安全", p)
	if got != "zhonghuatianyuanquan" {
		t.Errorf("line start match, got %q, want %q", got, "zhonghuatianyuanquan")
	}
}
