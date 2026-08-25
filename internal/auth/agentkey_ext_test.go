package auth

import "testing"

// TestAgentKey_CrossInvocationWriteback verifies F178: a key issued in one
// "invocation" (registry instance) is durably written back and visible to a
// brand-new registry over the same store, including its thread/Memory
// back-reference metadata.
func TestAgentKey_CrossInvocationWriteback(t *testing.T) {
	store := NewInMemoryAgentKeyStore()
	r1 := NewAgentKeyRegistry(store)
	k, err := r1.Issue("dog-x", "key-1")
	if err != nil {
		t.Fatal(err)
	}
	k.Metadata = map[string]string{"thread_id": "t1", "memory_ref": "m1"}
	if err := store.Save(*k); err != nil {
		t.Fatal(err)
	}

	// A second invocation/process: brand-new registry over the SAME store.
	r2 := NewAgentKeyRegistry(store)
	ok, err := r2.Validate("key-1")
	if !ok || err != nil {
		t.Fatalf("key must validate across invocations (writeback persisted): ok=%v err=%v", ok, err)
	}
	loaded, err := store.Load("key-1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Metadata["thread_id"] != "t1" || loaded.Metadata["memory_ref"] != "m1" {
		t.Fatal("metadata not written back across invocations")
	}
}

// TestAgentKey_RevokeDenies verifies F178: after revoke, validation fails
// closed even from a different invocation instance sharing the store.
func TestAgentKey_RevokeDenies(t *testing.T) {
	store := NewInMemoryAgentKeyStore()
	r1 := NewAgentKeyRegistry(store)
	if _, err := r1.Issue("dog-x", "key-2"); err != nil {
		t.Fatal(err)
	}
	if err := r1.Revoke("key-2"); err != nil {
		t.Fatal(err)
	}

	r2 := NewAgentKeyRegistry(store)
	ok, err := r2.Validate("key-2")
	if ok {
		t.Fatal("revoked key must deny")
	}
	if err != ErrAgentKeyRevoked {
		t.Fatalf("expected ErrAgentKeyRevoked, got %v", err)
	}
}
