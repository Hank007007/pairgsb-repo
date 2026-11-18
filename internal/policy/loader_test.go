package policy

import (
	"errors"
	"testing"
)

func sampleBundle(revision string) Bundle {
	return Bundle{
		Revision: revision,
		Rules: []Rule{
			{
				ID:       "allow-alice-read",
				Effect:   Allow,
				Tenant:   "tenant-a",
				Subject:  "user:alice",
				Action:   "read",
				Resource: "doc:1",
			},
		},
	}
}

func TestLoadAndCurrent(t *testing.T) {
	l := NewLoader()

	if _, ok := l.Revision(); ok {
		t.Fatal("empty loader reports a revision")
	}
	if _, err := l.Current(); !errors.Is(err, ErrNoBundle) {
		t.Fatalf("Current on empty loader = %v, want ErrNoBundle", err)
	}

	if err := l.Load(sampleBundle("42")); err != nil {
		t.Fatalf("Load: %v", err)
	}
	rev, ok := l.Revision()
	if !ok || rev != "42" {
		t.Fatalf("Revision = %q, %v; want 42, true", rev, ok)
	}

	cur, err := l.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if cur.Revision != "42" || len(cur.Rules) != 1 {
		t.Fatalf("Current = %+v", cur)
	}
}

func TestCurrentReturnsACopy(t *testing.T) {
	l := NewLoader()
	if err := l.Load(sampleBundle("7")); err != nil {
		t.Fatalf("Load: %v", err)
	}

	first, err := l.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	first.Rules[0].Effect = Deny
	first.Rules = append(first.Rules, Rule{ID: "extra"})

	second, err := l.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if second.Rules[0].Effect != Allow || len(second.Rules) != 1 {
		t.Fatalf("caller mutation leaked into the loader: %+v", second)
	}
}

func TestLoadRejectsOlderRevision(t *testing.T) {
	l := NewLoader()
	if err := l.Load(sampleBundle("12")); err != nil {
		t.Fatalf("Load 12: %v", err)
	}
	if err := l.Load(sampleBundle("11")); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Load 11 after 12 = %v, want ErrStaleRevision", err)
	}
	if err := l.Load(sampleBundle("13")); err != nil {
		t.Fatalf("Load 13: %v", err)
	}
	rev, _ := l.Revision()
	if rev != "13" {
		t.Fatalf("Revision = %q, want 13", rev)
	}
}

func TestLoadSameRevisionIsIdempotent(t *testing.T) {
	l := NewLoader()
	b := sampleBundle("20")
	if err := l.Load(b); err != nil {
		t.Fatalf("Load: %v", err)
	}
	b.Rules = nil
	if err := l.Load(b); err != nil {
		t.Fatalf("Load same revision: %v", err)
	}
	cur, err := l.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if len(cur.Rules) != 0 {
		t.Fatalf("same-revision load did not replace the bundle: %+v", cur)
	}
}

func TestBundleValidate(t *testing.T) {
	tests := []struct {
		name    string
		bundle  Bundle
		wantErr bool
	}{
		{
			name:    "valid",
			bundle:  sampleBundle("1"),
			wantErr: false,
		},
		{
			name:    "empty revision",
			bundle:  Bundle{Rules: sampleBundle("1").Rules},
			wantErr: true,
		},
		{
			name:    "non numeric revision",
			bundle:  Bundle{Revision: "v1", Rules: sampleBundle("1").Rules},
			wantErr: true,
		},
		{
			name: "duplicate rule id",
			bundle: Bundle{Revision: "1", Rules: []Rule{
				{ID: "r", Effect: Allow, Tenant: "t", Subject: "s", Action: "a", Resource: "x"},
				{ID: "r", Effect: Deny, Tenant: "t", Subject: "s", Action: "a", Resource: "y"},
			}},
			wantErr: true,
		},
		{
			name: "unknown effect",
			bundle: Bundle{Revision: "1", Rules: []Rule{
				{ID: "r", Effect: "maybe", Tenant: "t", Subject: "s", Action: "a", Resource: "x"},
			}},
			wantErr: true,
		},
		{
			name: "empty pattern",
			bundle: Bundle{Revision: "1", Rules: []Rule{
				{ID: "r", Effect: Allow, Tenant: "t", Subject: "", Action: "a", Resource: "x"},
			}},
			wantErr: true,
		},
		{
			name: "inverted interval",
			bundle: Bundle{Revision: "1", Rules: []Rule{
				{ID: "r", Effect: Allow, Tenant: "t", Subject: "s", Action: "a", Resource: "x",
					NotBefore: testClock, NotAfter: testClock.Add(-1)},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()
			if tt.wantErr && !errors.Is(err, ErrInvalidBundle) {
				t.Fatalf("Validate = %v, want ErrInvalidBundle", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate = %v, want nil", err)
			}
		})
	}
}

func TestReadBundle(t *testing.T) {
	data := []byte(`{
		"revision": "5",
		"rules": [
			{
				"id": "deny-secret",
				"effect": "deny",
				"tenant": "tenant-a",
				"subject": "user:*",
				"action": "read",
				"resource": "secret:*"
			}
		]
	}`)

	b, err := ReadBundle(data)
	if err != nil {
		t.Fatalf("ReadBundle: %v", err)
	}
	if b.Revision != "5" || len(b.Rules) != 1 {
		t.Fatalf("ReadBundle = %+v", b)
	}
	if b.Rules[0].Effect != Deny {
		t.Fatalf("effect = %q, want deny", b.Rules[0].Effect)
	}

	if _, err := ReadBundle([]byte(`{"revision":"5","rules":[],"extra":true}`)); !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("unknown field = %v, want ErrInvalidBundle", err)
	}
	if _, err := ReadBundle([]byte(`not json`)); !errors.Is(err, ErrInvalidBundle) {
		t.Fatalf("malformed json = %v, want ErrInvalidBundle", err)
	}
}

func TestLoadJSON(t *testing.T) {
	l := NewLoader()
	err := l.LoadJSON([]byte(`{
		"revision": "3",
		"rules": [
			{"id":"r1","effect":"allow","tenant":"tenant-a","subject":"user:alice","action":"read","resource":"doc:1"}
		]
	}`))
	if err != nil {
		t.Fatalf("LoadJSON: %v", err)
	}
	rev, _ := l.Revision()
	if rev != "3" {
		t.Fatalf("Revision = %q, want 3", rev)
	}
}

func TestBundleRuleLookup(t *testing.T) {
	b := sampleBundle("1")
	r, ok := b.Rule("allow-alice-read")
	if !ok || r.ID != "allow-alice-read" {
		t.Fatalf("Rule = %+v, %v", r, ok)
	}
	if _, ok := b.Rule("missing"); ok {
		t.Fatal("found a rule that does not exist")
	}
	var nilBundle *Bundle
	if got := nilBundle.Clone(); got != nil {
		t.Fatalf("nil Clone = %+v", got)
	}
}
