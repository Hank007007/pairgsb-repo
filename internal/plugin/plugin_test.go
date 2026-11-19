package plugin_test

import (
	"path/filepath"
	"testing"

	"argvsieve-exec-args/internal/plugin"
)

func TestLoadDemoManifest(t *testing.T) {
	p, err := plugin.LoadFile(filepath.Join("..", "..", "examples", "plugins", "demo", "plugin.yaml"))
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if p.Name != "demo" {
		t.Errorf("Name = %q, want demo", p.Name)
	}
	names := p.StepNames()
	if len(names) != 2 || names[0] != "greet" || names[1] != "legacy-report" {
		t.Fatalf("StepNames = %q, want [greet legacy-report] in manifest order", names)
	}

	greet, ok := p.Step("greet")
	if !ok {
		t.Fatal("missing step greet")
	}
	if greet.Executable != "./bin/echoargs" {
		t.Errorf("Executable = %q", greet.Executable)
	}
	if len(greet.Args) != 3 || greet.Args[0] != "--mode=greet" {
		t.Errorf("Args = %q", greet.Args)
	}
	if len(greet.Env) != 3 || greet.Env[1] != "LANG=C.UTF-8" {
		t.Errorf("Env = %q", greet.Env)
	}
	if !greet.HasPositionals || len(greet.Positionals) != 2 {
		t.Errorf("Positionals = %q, HasPositionals = %t", greet.Positionals, greet.HasPositionals)
	}

	legacy, ok := p.Step("legacy-report")
	if !ok {
		t.Fatal("missing step legacy-report")
	}
	if legacy.HasPositionals {
		t.Errorf("HasPositionals = true for a step without a positionals field")
	}
}

func TestParseRejectsStepWithoutExecutable(t *testing.T) {
	_, err := plugin.Parse([]byte("name: broken\nsteps:\n  x:\n    args:\n      - a\n"))
	if err == nil {
		t.Fatal("Parse accepted a step without an executable")
	}
}

func TestParseQuotedScalars(t *testing.T) {
	p, err := plugin.Parse([]byte("name: q\nsteps:\n  s:\n    executable: \"/bin/x\"\n    args:\n      - \"--a=b c\"\n      - '--d=it''s'\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	s, _ := p.Step("s")
	if s.Executable != "/bin/x" {
		t.Errorf("Executable = %q", s.Executable)
	}
	if len(s.Args) != 2 || s.Args[0] != "--a=b c" {
		t.Errorf("Args = %q", s.Args)
	}
	if s.Args[1] != "--d=it's" {
		t.Errorf("Args[1] = %q", s.Args[1])
	}
}
