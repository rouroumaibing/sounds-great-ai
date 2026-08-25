package auth

import (
	"path/filepath"
	"testing"
	"time"
)

// F174 核心交付：回调 token 落 SQLite 后，进程重启（store 重开）不丢。
func TestCallbackTokenStore_PersistsAcrossRestart(t *testing.T) {
	now := time.Now()
	dbPath := filepath.Join(t.TempDir(), "cbauth.db")

	// 第一次生命周期：签发 + 落盘 + 关闭（模拟进程退出）。
	s1 := NewCallbackTokenStore(dbPath)
	if s1 == nil {
		t.Fatal("store must open")
	}
	good := NewCallbackToken("persist-good", "op-1", []string{"thread:write"}, now.Add(time.Hour))
	s1.Put(good)
	s1.Close()

	// 第二次生命周期：重新打开（模拟进程重启）。
	s2 := NewCallbackTokenStore(dbPath)
	if s2 == nil {
		t.Fatal("store must reopen after restart")
	}
	defer s2.Close()
	c, ok := s2.Lookup("persist-good", now)
	if !ok || c == nil {
		t.Fatal("token must survive restart")
	}
	if c.UserID != "op-1" || len(c.Scopes) != 1 || c.Scopes[0] != "thread:write" {
		t.Fatalf("token fields not persisted: %+v", c)
	}
	if c.Degrade != DegradeDeny {
		t.Fatalf("degrade policy not persisted: %q", c.Degrade)
	}
}

// F174 + F298：tombstone（failure reason）落盘后，重启仍 fail-closed 拒绝。
func TestCallbackTokenStore_TombstonePersists(t *testing.T) {
	now := time.Now()
	dbPath := filepath.Join(t.TempDir(), "cbauth.db")

	s1 := NewCallbackTokenStore(dbPath)
	tok := NewCallbackToken("doomed", "op-1", nil, now.Add(time.Hour))
	s1.Put(tok)
	s1.MarkFailure("doomed", ReasonRevoked) // 直接落盘 tombstone
	s1.Close()

	s2 := NewCallbackTokenStore(dbPath)
	defer s2.Close()
	if c, ok := s2.Lookup("doomed", now); ok || c != nil {
		t.Fatal("revoked token must stay rejected after restart")
	}
}

// F174：续期（Refresh）落盘后，重启仍有效（expire 被顺延）。
func TestCallbackTokenStore_RefreshPersists(t *testing.T) {
	now := time.Now()
	dbPath := filepath.Join(t.TempDir(), "cbauth.db")

	s1 := NewCallbackTokenStore(dbPath)
	// 签发一个已过期 token
	expired := NewCallbackToken("stale", "op-1", nil, now.Add(-time.Hour))
	s1.Put(expired)
	s1.Refresh("stale", now.Add(time.Hour)) // 续期落盘
	s1.Close()

	s2 := NewCallbackTokenStore(dbPath)
	defer s2.Close()
	c, ok := s2.Lookup("stale", now)
	if !ok || c == nil {
		t.Fatal("refreshed token must be valid after restart")
	}
	if !c.ExpiresAt.After(now) {
		t.Fatal("refreshed expiry not persisted")
	}
	if c.Failure != "" {
		t.Fatal("refresh must clear failure on disk")
	}
}

// F174：显式 Revoke 语义 = 持久 tombstone。
func TestCallbackTokenStore_RevokePersists(t *testing.T) {
	now := time.Now()
	dbPath := filepath.Join(t.TempDir(), "cbauth.db")

	s1 := NewCallbackTokenStore(dbPath)
	s1.Put(NewCallbackToken("rtok", "op-1", nil, now.Add(time.Hour)))
	s1.Revoke("rtok")
	s1.Close()

	s2 := NewCallbackTokenStore(dbPath)
	defer s2.Close()
	if c, ok := s2.Lookup("rtok", now); ok || c != nil {
		t.Fatal("revoked token must be rejected after restart")
	}
}

// F174 fail-closed：空路径 → nil store，Put/Lookup 不 panic 且拒绝一切。
func TestCallbackTokenStore_EmptyPathFailClosed(t *testing.T) {
	now := time.Now()
	s := NewCallbackTokenStore("")
	if s != nil {
		t.Fatal("empty path must yield nil store")
	}
	// 下面这些在 nil 上应安全 no-op/拒绝，不 panic。
	s.Put(NewCallbackToken("x", "u", nil, now.Add(time.Hour)))
	if c, ok := s.Lookup("x", now); ok || c != nil {
		t.Fatal("nil store must reject all lookups")
	}
}
