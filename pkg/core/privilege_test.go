package core

import "testing"

func TestRequireNonRoot(t *testing.T) {
	if err := RequireNonRoot(0); err == nil {
		t.Fatal("root execution accepted")
	}
	if err := RequireNonRoot(1000); err != nil {
		t.Fatalf("ordinary uid rejected: %v", err)
	}
}
