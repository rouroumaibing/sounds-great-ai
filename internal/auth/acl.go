package auth

import (
	"errors"
	"sync"
)

// Access is a thread permission level. Higher levels imply lower ones
// (admin > write > read). A Denied pseudo-level is never granted.
type Access string

const (
	// AccessRead allows viewing a thread.
	AccessRead Access = "read"
	// AccessWrite allows posting/acting on a thread.
	AccessWrite Access = "write"
	// AccessAdmin allows managing ACLs and membership.
	AccessAdmin Access = "admin"
)

// level ranks access for implication checks.
func (a Access) level() int {
	switch a {
	case AccessRead:
		return 1
	case AccessWrite:
		return 2
	case AccessAdmin:
		return 3
	default:
		return 0
	}
}

// ErrDenied is returned when an ACL check fails (fail-closed).
var ErrDenied = errors.New("auth: access denied (fail-closed)")

// ThreadACL enforces per-thread access. By default a thread has NO rules, which
// means EVERYONE is denied (fail-closed) until an owner grants access.
type ThreadACL struct {
	mu    sync.RWMutex
	rules map[string]map[string]Access // threadID -> userID -> access
}

// NewThreadACL creates an empty ACL.
func NewThreadACL() *ThreadACL {
	return &ThreadACL{rules: make(map[string]map[string]Access)}
}

// Grant gives userID access on threadID.
func (a *ThreadACL) Grant(threadID, userID string, access Access) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rules[threadID] == nil {
		a.rules[threadID] = make(map[string]Access)
	}
	a.rules[threadID][userID] = access
}

// Revoke removes userID's access on threadID.
func (a *ThreadACL) Revoke(threadID, userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if m := a.rules[threadID]; m != nil {
		delete(m, userID)
	}
}

// Check reports whether userID holds at least `needed` access on threadID.
// Fail-closed: an unknown thread, unknown user, or insufficient level denies.
func (a *ThreadACL) Check(threadID, userID string, needed Access) (bool, error) {
	a.mu.RLock()
	m, ok := a.rules[threadID]
	a.mu.RUnlock()
	if !ok {
		return false, ErrDenied // no rules => deny all
	}
	have, ok := m[userID]
	if !ok || have.level() < needed.level() {
		return false, ErrDenied
	}
	return true, nil
}

// AccessOf returns the access a user holds on a thread ("", false if none).
func (a *ThreadACL) AccessOf(threadID, userID string) (Access, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m, ok := a.rules[threadID]
	if !ok {
		return "", false
	}
	acc, ok := m[userID]
	return acc, ok
}
