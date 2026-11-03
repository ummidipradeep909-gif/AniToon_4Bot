package moderation

import "testing"

func TestIsChatAdmin(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"creator", true},
		{"administrator", true},
		{"member", false},
		{"restricted", false},
		{"left", false},
		{"kicked", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsChatAdmin(tt.status); got != tt.want {
			t.Errorf("IsChatAdmin(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestIsProtectedTarget(t *testing.T) {
	if !IsProtectedTarget("creator") {
		t.Fatal("creator must be protected")
	}
	if !IsProtectedTarget("administrator") {
		t.Fatal("administrator must be protected")
	}
	if IsProtectedTarget("member") {
		t.Fatal("ordinary member must not be protected")
	}
}
