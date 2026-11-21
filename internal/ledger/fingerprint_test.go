package ledger

import "testing"

func TestComputeIsStableForIdenticalInput(t *testing.T) {
	in := FingerprintInput{RuleID: "go/sql-injection", File: "internal/db/query.go", Line: 42}
	if got, want := Compute(in), Compute(in); got != want {
		t.Fatalf("Compute is not deterministic: %s != %s", got, want)
	}
}

func TestComputeSeparatesRules(t *testing.T) {
	file := "internal/db/query.go"
	a := Compute(FingerprintInput{RuleID: "go/sql-injection", File: file, Line: 42})
	b := Compute(FingerprintInput{RuleID: "go/hardcoded-secret", File: file, Line: 42})
	if a == b {
		t.Fatal("different rules produced the same fingerprint")
	}
}

func TestComputeSeparatesFiles(t *testing.T) {
	a := Compute(FingerprintInput{RuleID: "go/sql-injection", File: "internal/db/query.go", Line: 42})
	b := Compute(FingerprintInput{RuleID: "go/sql-injection", File: "internal/db/exec.go", Line: 42})
	if a == b {
		t.Fatal("different files produced the same fingerprint")
	}
}

func TestComputeIsHexSHA256(t *testing.T) {
	fp := Compute(FingerprintInput{RuleID: "r", File: "a.go", Line: 1})
	if len(fp) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(fp))
	}
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("fingerprint contains non-lowercase-hex rune %q", c)
		}
	}
}
