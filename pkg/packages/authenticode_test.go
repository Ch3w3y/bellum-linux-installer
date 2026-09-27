package packages

import "testing"

func TestApprovedSignerMustBeLeafExactCN(t *testing.T) {
	valid := "Signer #0:\n Subject: C=US, O=Astarte, CN=ASTARTE INDUSTRIES INC.\nSigner #1:\n Subject: CN=ASTARTE INDUSTRIES INC.\n"
	if !hasApprovedLeafSubject(valid, "ASTARTE INDUSTRIES INC.") {
		t.Fatal("approved leaf signer rejected")
	}
	if hasApprovedLeafSubject(valid, "ASTARTE") {
		t.Fatal("partial signer subject accepted")
	}
	if hasApprovedLeafSubject("Signer #0:\n Subject: CN=OTHER INC.\n", "ASTARTE INDUSTRIES INC.") {
		t.Fatal("wrong signer accepted")
	}
}
