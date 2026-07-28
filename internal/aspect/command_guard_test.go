package aspect

import (
	"testing"
)

func TestGuardCommandBlacklist(t *testing.T) {
	guard := NewCommandGuard()

	tests := []struct {
		cmd     string
		blocked bool
	}{
		{"rm -rf /", true},
		{"rm -rf /home", true},
		{"kill -9 1", true},
		{"kill -9 $PPID", true},
		{"drop database mydb", true},
		{"redis-cli flushall", true},
		{":(){ :|:& };:", true}, // fork bomb
		{"echo hello", false},
		{"ls -la", false},
		{"git status", false},
		{"go test ./...", false},
	}

	for _, tt := range tests {
		result := guard.GuardCommand(tt.cmd)
		if tt.blocked && result.Status != GuardStatusBlocked {
			t.Errorf("GuardCommand(%q) should be blocked, got %s: %s", tt.cmd, result.Status, result.Reason)
		}
		if !tt.blocked && result.Status == GuardStatusBlocked {
			t.Errorf("GuardCommand(%q) should not be blocked, got blocked: %s", tt.cmd, result.Reason)
		}
	}
}

func TestGuardCommandShellFeatures(t *testing.T) {
	guard := NewCommandGuard()

	tests := []string{
		"echo hello | cat",
		"echo hello; rm -rf /",
		"echo hello > /etc/passwd",
		"echo $(whoami)",
		"echo `whoami`",
		"echo $HOME",
		"echo ${HOME}",
		"cat file &",
	}

	for _, cmd := range tests {
		result := guard.GuardCommand(cmd)
		if result.Status != GuardStatusBlocked {
			t.Errorf("GuardCommand(%q) should be blocked for shell features, got %s", cmd, result.Status)
		}
	}
}

func TestGuardCommandHITL(t *testing.T) {
	guard := NewCommandGuard()

	tests := []string{
		"git push --force",
		"git push -f origin main",
		"go get github.com/some/new/dependency",
		"npm install",
	}

	for _, cmd := range tests {
		result := guard.GuardCommand(cmd)
		if result.Status != GuardStatusNeedsApproval {
			t.Errorf("GuardCommand(%q) should need approval, got %s", cmd, result.Status)
		}
	}
}

func TestGuardFilePath(t *testing.T) {
	guard := NewCommandGuard()

	tests := []struct {
		path     string
		op       FileOp
		expected GuardStatus
	}{
		{".env", FileOpWrite, GuardStatusNeedsApproval},
		{"config.json", FileOpWrite, GuardStatusNeedsApproval},
		{"go.mod", FileOpWrite, GuardStatusNeedsApproval},
		{"main.go", FileOpWrite, GuardStatusAllowed},
		{"main.go", FileOpRead, GuardStatusAllowed},
	}

	for _, tt := range tests {
		result := guard.GuardFilePath(tt.path, tt.op)
		if result.Status != tt.expected {
			t.Errorf("GuardFilePath(%q, %v) = %s, want %s: %s", tt.path, tt.op, result.Status, tt.expected, result.Reason)
		}
	}
}
