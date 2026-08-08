package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"sounds-great-ai/internal/memory"
	"sounds-great-ai/internal/ragstore"
	"sounds-great-ai/internal/settings"
	"sounds-great-ai/internal/threadstore"
)

// RunVectorStoreContract runs the contract test suite for any ragstore.VectorStore
// implementation. It verifies Upsert+Search, GetByID, ListAll, Delete, DropAll, and Close.
func RunVectorStoreContract(t *testing.T, store ragstore.VectorStore) {
	ctx := context.Background()

	// --- Upsert + Search ---
	docs := []*schema.Document{
		{ID: "vc-1", Content: "hello world"},
		{ID: "vc-2", Content: "foo bar"},
	}
	if err := store.Upsert(ctx, docs); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	results, err := store.Search(ctx, "hello", ragstore.SearchOpts{TopK: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Search: expected at least 1 result, got 0")
	}

	// --- GetByID ---
	doc, err := store.GetByID(ctx, "vc-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if doc.ID != "vc-1" {
		t.Fatalf("GetByID: want vc-1, got %s", doc.ID)
	}

	// --- ListAll ---
	all, err := store.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListAll: want 2, got %d", len(all))
	}

	// --- Delete ---
	if err := store.Delete(ctx, []string{"vc-1"}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	all, _ = store.ListAll(ctx)
	if len(all) != 1 {
		t.Fatalf("after Delete: want 1, got %d", len(all))
	}
	if all[0].ID != "vc-2" {
		t.Fatalf("after Delete: want vc-2, got %s", all[0].ID)
	}

	// --- DropAll ---
	if err := store.DropAll(ctx); err != nil {
		t.Fatalf("DropAll: %v", err)
	}
	all, _ = store.ListAll(ctx)
	if len(all) != 0 {
		t.Fatalf("after DropAll: want 0, got %d", len(all))
	}

	// --- Close ---
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// RunThreadStoreContract runs the contract test suite for any threadstore.ThreadStore
// implementation. It verifies CreateThread+ListThreads, UpdateTitle, DeleteThread,
// DeleteThread-not-found, CreateSession+ListSessions, and UnsealSession.
func RunThreadStoreContract(t *testing.T, store threadstore.ThreadStore) {
	// --- CreateThread + ListThreads ---
	thread, err := store.CreateThread("Test Thread")
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("CreateThread: empty ID")
	}
	if thread.Title != "Test Thread" {
		t.Fatalf("CreateThread: want title 'Test Thread', got %q", thread.Title)
	}

	threads, err := store.ListThreads()
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("ListThreads: want 1, got %d", len(threads))
	}

	// --- UpdateTitle ---
	if err := store.UpdateTitle(thread.ID, "Updated Title"); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}
	threads, _ = store.ListThreads()
	if threads[0].Title != "Updated Title" {
		t.Fatalf("UpdateTitle: want 'Updated Title', got %q", threads[0].Title)
	}

	// --- CreateSession + ListSessions ---
	sess, err := store.CreateSession(thread.ID, "bianmu")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("CreateSession: empty ID")
	}
	if sess.ThreadID != thread.ID {
		t.Fatalf("CreateSession: want threadID %s, got %s", thread.ID, sess.ThreadID)
	}
	if sess.BreedID != "bianmu" {
		t.Fatalf("CreateSession: want breedID bianmu, got %s", sess.BreedID)
	}

	sessions, err := store.ListSessions(thread.ID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("ListSessions: want 1, got %d", len(sessions))
	}

	// --- UnsealSession ---
	if err := store.UnsealSession(sess.ID); err != nil {
		t.Fatalf("UnsealSession: %v", err)
	}
	sessions, _ = store.ListSessions(thread.ID)
	if sessions[0].Status != "active" {
		t.Fatalf("UnsealSession: want status 'active', got %q", sessions[0].Status)
	}

	// --- DeleteThread ---
	if err := store.DeleteThread(thread.ID); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}
	threads, _ = store.ListThreads()
	if len(threads) != 0 {
		t.Fatalf("after DeleteThread: want 0, got %d", len(threads))
	}

	// --- DeleteThread not-found ---
	if err := store.DeleteThread("nonexistent"); err == nil {
		t.Fatal("DeleteThread: expected error for nonexistent thread, got nil")
	}
}

// RunMessageStoreContract runs the contract test suite for any threadstore.MessageStore
// implementation. It verifies Append+GetByThread, Append-empty-threadID-fails,
// GetByThread-empty, and GetByThreadBefore.
func RunMessageStoreContract(t *testing.T, store threadstore.MessageStore) {
	now := time.Now()

	// --- Append empty threadID fails ---
	if err := store.Append(&threadstore.Message{Role: "user", Content: "no thread"}); err == nil {
		t.Fatal("Append: expected error for empty ThreadID, got nil")
	}

	// --- Append + GetByThread ---
	msg1 := &threadstore.Message{
		ThreadID:  "mc-1",
		Role:      "user",
		Content:   "hello",
		Timestamp: now,
	}
	if err := store.Append(msg1); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if msg1.ID == "" {
		t.Fatal("Append: expected auto-generated ID")
	}

	msg2 := &threadstore.Message{
		ThreadID:  "mc-1",
		Role:      "assistant",
		Content:   "world",
		Sender:    "bianmu",
		Timestamp: now.Add(time.Second),
	}
	if err := store.Append(msg2); err != nil {
		t.Fatalf("Append: %v", err)
	}

	msgs, err := store.GetByThread("mc-1", 0)
	if err != nil {
		t.Fatalf("GetByThread: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("GetByThread: want 2, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Fatalf("GetByThread: want first 'hello', got %q", msgs[0].Content)
	}

	// --- GetByThread-empty ---
	empty, err := store.GetByThread("nonexistent", 0)
	if err != nil {
		t.Fatalf("GetByThread empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("GetByThread empty: want 0, got %d", len(empty))
	}

	// --- GetByThreadBefore ---
	before, err := store.GetByThreadBefore("mc-1", msg2.Timestamp, msg2.ID, 10)
	if err != nil {
		t.Fatalf("GetByThreadBefore: %v", err)
	}
	if len(before) != 1 {
		t.Fatalf("GetByThreadBefore: want 1 (only msg1), got %d", len(before))
	}
	if before[0].Content != "hello" {
		t.Fatalf("GetByThreadBefore: want 'hello', got %q", before[0].Content)
	}
}

// RunSettingsStoreContract runs the contract test suite for any settings.SettingsStore
// implementation. It verifies CreateMember+ListMembers, UpdateMember, DeleteMember,
// DeleteMember-not-found, CreateAccount+ListAccounts, DeleteAccount, ListConfig, UpdateConfig.
func RunSettingsStoreContract(t *testing.T, store settings.SettingsStore) {
	// --- CreateMember + ListMembers ---
	member, err := store.CreateMember("bianmu", "Border Collie", "router", true)
	if err != nil {
		t.Fatalf("CreateMember: %v", err)
	}
	if member.ID == "" {
		t.Fatal("CreateMember: empty ID")
	}
	if member.BreedID != "bianmu" {
		t.Fatalf("CreateMember: want breedID bianmu, got %s", member.BreedID)
	}

	members, err := store.ListMembers()
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("ListMembers: want 1, got %d", len(members))
	}

	// --- UpdateMember ---
	if err := store.UpdateMember(member.ID, map[string]any{"display_name": "Updated Name"}); err != nil {
		t.Fatalf("UpdateMember: %v", err)
	}
	members, _ = store.ListMembers()
	if members[0].DisplayName != "Updated Name" {
		t.Fatalf("UpdateMember: want 'Updated Name', got %q", members[0].DisplayName)
	}

	// --- DeleteMember ---
	if err := store.DeleteMember(member.ID); err != nil {
		t.Fatalf("DeleteMember: %v", err)
	}
	members, _ = store.ListMembers()
	if len(members) != 0 {
		t.Fatalf("after DeleteMember: want 0, got %d", len(members))
	}

	// --- DeleteMember not-found ---
	if err := store.DeleteMember("nonexistent"); err == nil {
		t.Fatal("DeleteMember: expected error for nonexistent member, got nil")
	}

	// --- CreateAccount + ListAccounts ---
	account, err := store.CreateAccount("anthropic", "sk-test-key")
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if account.ID == "" {
		t.Fatal("CreateAccount: empty ID")
	}
	if account.Provider != "anthropic" {
		t.Fatalf("CreateAccount: want provider anthropic, got %s", account.Provider)
	}

	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("ListAccounts: want 1, got %d", len(accounts))
	}

	// --- DeleteAccount ---
	if err := store.DeleteAccount(account.ID); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	accounts, _ = store.ListAccounts()
	if len(accounts) != 0 {
		t.Fatalf("after DeleteAccount: want 0, got %d", len(accounts))
	}

	// --- ListConfig ---
	configs, err := store.ListConfig()
	if err != nil {
		t.Fatalf("ListConfig: %v", err)
	}
	if len(configs) == 0 {
		t.Fatal("ListConfig: expected at least 1 config, got 0")
	}

	// --- UpdateConfig ---
	firstKey := configs[0].Key
	if err := store.UpdateConfig(firstKey, "updated-value"); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	configs, _ = store.ListConfig()
	for _, c := range configs {
		if c.Key == firstKey {
			if c.Value != "updated-value" {
				t.Fatalf("UpdateConfig: want 'updated-value', got %q", c.Value)
			}
		}
	}
}

// RunEvidenceStoreContract runs the contract test suite for any memory.EvidenceStore
// implementation. It verifies AddEvidence+ListEvidence and that AddEvidence with an
// empty type string defaults to "evidence".
func RunEvidenceStoreContract(t *testing.T, store memory.EvidenceStore) {
	// --- AddEvidence + ListEvidence ---
	rec, err := store.AddEvidence("thread-1", "bug", "Nil pointer", "Found nil pointer in line 42", []string{"bug", "nil"})
	if err != nil {
		t.Fatalf("AddEvidence: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("AddEvidence: empty ID")
	}
	if rec.Type != "bug" {
		t.Fatalf("AddEvidence: want type 'bug', got %q", rec.Type)
	}

	records, err := store.ListEvidence()
	if err != nil {
		t.Fatalf("ListEvidence: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("ListEvidence: want 1, got %d", len(records))
	}

	// --- AddEvidence empty type defaults to "evidence" ---
	rec2, err := store.AddEvidence("thread-2", "", "Default type", "Some content", nil)
	if err != nil {
		t.Fatalf("AddEvidence empty type: %v", err)
	}
	if rec2.Type != "evidence" {
		t.Fatalf("AddEvidence empty type: want 'evidence', got %q", rec2.Type)
	}
}

// RunCredentialStoreContract runs the contract test suite for any settings.CredentialStore
// implementation. It verifies Set+Get, Has, and Delete.
func RunCredentialStoreContract(t *testing.T, store settings.CredentialStore) {
	// --- Set + Get ---
	if err := store.Set("anthropic", "sk-ant-xxx"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := store.Get("anthropic")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "sk-ant-xxx" {
		t.Fatalf("Get: want sk-ant-xxx, got %s", val)
	}

	// --- Has ---
	if !store.Has("anthropic") {
		t.Fatal("Has: expected true for 'anthropic'")
	}
	if store.Has("openai") {
		t.Fatal("Has: expected false for unset 'openai'")
	}

	// --- Delete ---
	if err := store.Delete("anthropic"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Has("anthropic") {
		t.Fatal("Has: expected false after Delete")
	}
	_, err = store.Get("anthropic")
	if err == nil {
		t.Fatal("Get: expected error after Delete, got nil")
	}
}
