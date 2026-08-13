package settings

import (
	"testing"

	"sounds-great-ai/pkg/pack"
)

func TestCredentialReady(t *testing.T) {
	mem := NewMemoryCredentialStore()
	mem.Set("acct_api", "secret")

	// api_key with credential present → ready
	bAPI := &pack.BreedConfig{ID: "b_api", Variants: []pack.Variant{{ID: "default", AccountRef: "acct_api"}}}
	acctAPI := &Account{ID: "acct_api", AuthType: "api_key"}
	if !CredentialReady(bAPI, acctAPI, mem) {
		t.Error("api_key with credential should be ready")
	}

	// api_key without credential → not ready
	memEmpty := NewMemoryCredentialStore()
	if CredentialReady(bAPI, acctAPI, memEmpty) {
		t.Error("api_key without credential should not be ready")
	}

	// oauth with existing binary ("sh" exists on PATH) → ready
	bOAuth := &pack.BreedConfig{ID: "b_oauth", Variants: []pack.Variant{{ID: "default", AccountRef: "acct_oauth"}}}
	acctOAuth := &Account{ID: "acct_oauth", AuthType: "oauth", ClientID: "sh"}
	if !CredentialReady(bOAuth, acctOAuth, mem) {
		t.Error("oauth with existing binary should be ready")
	}

	// oauth with absent binary → not ready
	bOAuthBad := &pack.BreedConfig{ID: "b_oauth_bad", Variants: []pack.Variant{{ID: "default", AccountRef: "acct_oauth_bad"}}}
	acctOAuthBad := &Account{ID: "acct_oauth_bad", AuthType: "oauth", ClientID: "this-binary-does-not-exist-xyz-123"}
	if CredentialReady(bOAuthBad, acctOAuthBad, mem) {
		t.Error("oauth without binary should not be ready")
	}

	// no bound account → not ready
	bNoRef := &pack.BreedConfig{ID: "b_noref", Variants: []pack.Variant{{ID: "default"}}}
	if CredentialReady(bNoRef, acctAPI, mem) {
		t.Error("no account ref should not be ready")
	}

	// nil breed / nil account → not ready
	if CredentialReady(nil, acctAPI, mem) {
		t.Error("nil breed should not be ready")
	}
	if CredentialReady(bAPI, nil, mem) {
		t.Error("nil account should not be ready")
	}

	// unknown auth type → not ready
	acctUnknown := &Account{ID: "acct_api", AuthType: "magic"}
	if CredentialReady(bAPI, acctUnknown, mem) {
		t.Error("unknown auth type should not be ready")
	}
}
