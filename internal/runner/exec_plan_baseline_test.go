package runner_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"argvsieve-exec-args/internal/plugin"
	"argvsieve-exec-args/internal/runner"
)

// baseEnv is a small, deterministic stand-in for the runner's environment so
// that assertions on the resulting env do not depend on the host.
var baseEnv = []string{
	"CI=true",
	"CI_PIPELINE_ID=42",
	"HOME=/home/ci",
	"LANG=en_US.UTF-8",
}

func loadStep(t *testing.T, dir, step string) plugin.Step {
	t.Helper()
	p, err := plugin.LoadFile(filepath.Join("testdata", dir, "plugin.yaml"))
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	s, ok := p.Step(step)
	if !ok {
		t.Fatalf("plugin %s has no step %q", dir, step)
	}
	return s
}

func planFor(s plugin.Step, inputs map[string]string) runner.StepPlan {
	return runner.StepPlan{
		Executable:     s.Executable,
		Args:           s.Args,
		Env:            s.Env,
		Workdir:        s.Workdir,
		Positionals:    s.Positionals,
		HasPositionals: s.HasPositionals,
		Inputs:         inputs,
		BaseEnv:        baseEnv,
	}
}

func TestBuildBasicStep(t *testing.T) {
	step := loadStep(t, "basic", "build")
	plan := planFor(step, map[string]string{
		"OUT":    "dist",
		"LABEL":  "nightly",
		"TARGET": "linux",
	})

	got, err := runner.Build(plan)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Cmd == nil {
		t.Fatal("Cmd is nil on success")
	}
	if got.Argv[0] != "/usr/local/bin/builder" {
		t.Errorf("argv[0] = %q, want the declared executable", got.Argv[0])
	}
	for _, arg := range got.Argv {
		if strings.Contains(arg, "${") {
			t.Errorf("argv element %q still contains a placeholder", arg)
		}
	}
	joined := strings.Join(got.Argv, "\x00")
	for _, want := range []string{"--out", "dist", "--label", "nightly", "linux"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv %q is missing %q", got.Argv, want)
		}
	}
	if got.UsedLegacy {
		t.Errorf("UsedLegacy = true for a plain argv declaration")
	}
}

func TestBuildValuesWithSpacesStaySingleElement(t *testing.T) {
	step := loadStep(t, "basic", "build")
	plan := planFor(step, map[string]string{
		"OUT":    "build dir",
		"LABEL":  "release candidate",
		"TARGET": "linux amd64",
	})

	got, err := runner.Build(plan)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	count := 0
	for _, arg := range got.Argv {
		if arg == "build dir" || arg == "release candidate" || arg == "linux amd64" {
			count++
		}
	}
	if count != 3 {
		t.Errorf("argv = %q, want the three spaced values as single elements", got.Argv)
	}
}

func TestBuildEnvCarriesDeclaredStepID(t *testing.T) {
	step := loadStep(t, "basic", "build")
	got, err := runner.Build(planFor(step, map[string]string{
		"OUT":    "dist",
		"LABEL":  "nightly",
		"TARGET": "linux",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	want := "CI_STEP_ID=build"
	for _, entry := range got.Env {
		if entry == want {
			return
		}
	}
	t.Errorf("env = %q, want entry %q", got.Env, want)
}

func TestBuildIsReproducible(t *testing.T) {
	step := loadStep(t, "basic", "package")
	plan := planFor(step, map[string]string{"ARCHIVE": "out/pkg.tar.gz"})

	first, err := runner.Build(plan)
	if err != nil {
		t.Fatalf("first Build: %v", err)
	}
	second, err := runner.Build(plan)
	if err != nil {
		t.Fatalf("second Build: %v", err)
	}

	if strings.Join(first.Argv, "\x00") != strings.Join(second.Argv, "\x00") {
		t.Errorf("argv differs between builds:\n%q\n%q", first.Argv, second.Argv)
	}
	if strings.Join(first.Env, "\n") != strings.Join(second.Env, "\n") {
		t.Errorf("env differs between builds:\n%q\n%q", first.Env, second.Env)
	}
}

func TestBuildLegacyManifest(t *testing.T) {
	step := loadStep(t, "legacy", "report")
	got, err := runner.Build(planFor(step, nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got.Cmd == nil || len(got.Argv) == 0 {
		t.Fatal("Build returned no command")
	}
	if !strings.Contains(strings.Join(got.Env, "\n"), "CI_STEP_ID=report") {
		t.Errorf("env = %q, want the declared step id", got.Env)
	}
}

func TestBuildEmptyExecutable(t *testing.T) {
	_, err := runner.Build(runner.StepPlan{BaseEnv: baseEnv})
	if !errors.Is(err, runner.ErrEmptyExecutable) {
		t.Fatalf("err = %v, want ErrEmptyExecutable", err)
	}
}

func TestBuildUnknownPlaceholder(t *testing.T) {
	plan := runner.StepPlan{
		Executable: "/usr/local/bin/builder",
		Args:       []string{"--out", "${INPUT_MISSING}"},
		Inputs:     map[string]string{"OUT": "dist"},
		BaseEnv:    baseEnv,
	}
	if _, err := runner.Build(plan); !errors.Is(err, runner.ErrUnknownPlaceholder) {
		t.Fatalf("err = %v, want ErrUnknownPlaceholder", err)
	}
}
