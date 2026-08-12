package settings

import "fmt"

// ValidCLIClientIDs is the set of client identifiers that support the CLI
// invocation model. Per decision D5 these are exactly the clients that can be
// driven through a local CLI binary: claude / codex / gemini / opencode / kimi.
//
// A member's (or account's) clientId must be one of these, OR empty. An empty
// clientId means a generic api_key account (see ValidateClientID and D5).
var ValidCLIClientIDs = map[string]bool{
	"claude":   true,
	"codex":    true,
	"gemini":   true,
	"opencode": true,
	"kimi":     true,
}

// OAuthClientRefs are the built-in CLI OAuth identifiers. A member bound to one
// of these refers to a CLI OAuth login rather than a catalog account, so it is
// accepted without a catalog lookup.
var OAuthClientRefs = map[string]bool{
	"claude":   true,
	"codex":    true,
	"gemini":   true,
	"opencode": true,
	"kimi":     true,
}

// ValidateClientID returns false if clientID is non-empty and not in the
// allowed CLI set. An empty clientId is always accepted (generic api_key
// account, per D5).
func ValidateClientID(clientID string) bool {
	if clientID == "" {
		return true
	}
	return ValidCLIClientIDs[clientID]
}

// ValidateAccountRef validates a member's account_ref:
//   - empty             → unbind, allowed
//   - built-in OAuth ref → allowed (CLI OAuth, not a catalog account)
//   - otherwise         → must reference an existing account ID in the store.
//
// The lookup uses the same store instance that account writes go through, so it
// can never resolve to a different data root — this avoids the clowder #1303
// "account written under A, validated under B" split.
func ValidateAccountRef(store SettingsStore, ref string) error {
	if ref == "" || OAuthClientRefs[ref] {
		return nil
	}
	accounts, err := store.ListAccounts()
	if err != nil {
		return err
	}
	for _, a := range accounts {
		if a.ID == ref {
			return nil
		}
	}
	return fmt.Errorf("account_ref %q not found", ref)
}
