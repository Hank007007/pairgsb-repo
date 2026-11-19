// Command demo builds an exec plan from a plugin manifest and, on request,
// runs it. It is the smallest end-to-end consumer of internal/runner:
//
//	go run ./cmd/demo -set greeting="hello world" -run
//
// With -twice the plan is built twice and the two argv/env pairs are compared,
// which is the reproducibility check described in docs/design.md.
//
// The bundled example manifest points at ./bin/echoargs. If that helper has
// not been built yet, demo builds it from ./cmd/echoargs first.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"argvsieve-exec-args/internal/plugin"
	"argvsieve-exec-args/internal/runner"
)

const helperPackage = "./cmd/echoargs"

type stringFlag []string

func (s *stringFlag) String() string { return strings.Join(*s, ",") }

func (s *stringFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "demo: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var sets stringFlag
	pluginPath := flag.String("plugin", "examples/plugins/demo/plugin.yaml", "path to a plugin manifest")
	stepName := flag.String("step", "", "step to build (default: the first declared step)")
	runIt := flag.Bool("run", false, "execute the prepared command")
	twice := flag.Bool("twice", false, "build the plan twice and compare the results")
	noBuild := flag.Bool("no-build", false, "never build the echoargs helper")
	flag.Var(&sets, "set", "input value as name=value; may be repeated")
	flag.Parse()

	inputs := map[string]string{}
	for _, entry := range sets {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || name == "" {
			return fmt.Errorf("invalid -set %q, want name=value", entry)
		}
		inputs[name] = value
	}

	p, err := plugin.LoadFile(*pluginPath)
	if err != nil {
		return err
	}

	name := *stepName
	if name == "" {
		names := p.StepNames()
		name = names[0]
	}
	step, ok := p.Step(name)
	if !ok {
		return fmt.Errorf("plugin %q has no step %q", p.Name, name)
	}

	if err := ensureHelper(step.Executable, *noBuild); err != nil {
		return err
	}

	plan := runner.StepPlan{
		PluginName:     p.Name,
		StepName:       name,
		Executable:     step.Executable,
		Args:           step.Args,
		Env:            step.Env,
		Workdir:        step.Workdir,
		Positionals:    step.Positionals,
		HasPositionals: step.HasPositionals,
		Inputs:         inputs,
	}

	result, err := runner.Build(plan)
	if err != nil {
		return err
	}

	fmt.Printf("plugin=%s step=%s\n", p.Name, name)
	fmt.Printf("argv (%d elements):\n", len(result.Argv))
	for i, arg := range result.Argv {
		fmt.Printf("  [%d] %q\n", i, arg)
	}
	fmt.Printf("env (%d entries):\n", len(result.Env))
	declaredKeys := map[string]bool{}
	for _, entry := range step.Env {
		key, _, _ := strings.Cut(entry, "=")
		declaredKeys[key] = true
	}
	for _, entry := range result.Env {
		key, value, _ := strings.Cut(entry, "=")
		if declaredKeys[key] {
			fmt.Printf("  %s=%s\n", key, value)
			continue
		}
		fmt.Printf("  %s\n", key)
	}
	if len(result.DroppedEnv) > 0 {
		fmt.Printf("dropped env keys: %s\n", strings.Join(result.DroppedEnv, ", "))
	}
	if result.UsedLegacy {
		fmt.Println("path: compatibility")
	} else {
		fmt.Println("path: argv")
	}

	if *twice {
		again, err := runner.Build(plan)
		if err != nil {
			return err
		}
		if err := compare(result, again); err != nil {
			return err
		}
		fmt.Println("reproducible: yes")
	}

	if !*runIt {
		return nil
	}

	// On Windows an executable built into the tree is named "*.exe", while
	// the manifest addresses it without the suffix. Resolve the file the
	// runner should start without touching argv.
	result.Cmd.Path = resolveExecutable(result.Cmd.Path)
	result.Cmd.Stdout = os.Stdout
	result.Cmd.Stderr = os.Stderr
	err = result.Cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		fmt.Println("exit status: 0")
	case errors.As(err, &exitErr):
		fmt.Printf("exit status: %d\n", exitErr.ExitCode())
	default:
		return err
	}
	return nil
}

// resolveExecutable appends the platform executable suffix when the declared
// path names a file that only exists with that suffix.
func resolveExecutable(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if _, err := os.Stat(path + ".exe"); err == nil {
		return path + ".exe"
	}
	return path
}

func compare(a, b runner.BuildResult) error {
	if len(a.Argv) != len(b.Argv) {
		return fmt.Errorf("argv length differs: %d vs %d", len(a.Argv), len(b.Argv))
	}
	for i := range a.Argv {
		if a.Argv[i] != b.Argv[i] {
			return fmt.Errorf("argv[%d] differs: %q vs %q", i, a.Argv[i], b.Argv[i])
		}
	}
	if len(a.Env) != len(b.Env) {
		return fmt.Errorf("env length differs: %d vs %d", len(a.Env), len(b.Env))
	}
	for i := range a.Env {
		if a.Env[i] != b.Env[i] {
			return fmt.Errorf("env[%d] differs: %q vs %q", i, a.Env[i], b.Env[i])
		}
	}
	return nil
}

// ensureHelper builds the bundled echoargs helper when the manifest points at
// a relative helper path that does not exist yet.
func ensureHelper(executable string, noBuild bool) error {
	if noBuild || !strings.HasPrefix(filepath.ToSlash(executable), "./bin/") {
		return nil
	}
	if _, err := os.Stat(executable); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		return err
	}
	out := executable
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(out), ".exe") {
		out += ".exe"
	}
	fmt.Printf("building helper %s\n", out)
	build := exec.Command("go", "build", "-o", out, helperPackage)
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	return build.Run()
}
