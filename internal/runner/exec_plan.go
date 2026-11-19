package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// StepPlan is everything the runner knows about one step at build time: the
// declaration supplied by the plugin, the working directory template, and the
// input values collected from the pipeline author.
//
// The zero value is usable; only Executable has to be set before calling
// Build. BaseEnv defaults to os.Environ() when left empty.
type StepPlan struct {
	// PluginName and StepName are used for audit messages only.
	PluginName string
	StepName   string

	// Executable is the program declared by the plugin.
	Executable string

	// Args holds the argument templates in declaration order.
	Args []string

	// Env holds "KEY=VALUE" entries declared by the plugin.
	Env []string

	// Workdir is the working directory template.
	Workdir string

	// Positionals holds the trailing argument templates.
	Positionals []string

	// HasPositionals reports whether the declaration carried a positionals
	// field. An explicitly empty list still counts.
	HasPositionals bool

	// Inputs maps placeholder names (the "xxx" in ${INPUT_xxx}) to values.
	Inputs map[string]string

	// BaseEnv is the environment inherited from the runner process. When
	// empty, os.Environ() is used.
	BaseEnv []string
}

// BuildResult is the outcome of Build. Cmd is ready to be handed to the
// runner, which starts it without further changes.
type BuildResult struct {
	// Cmd is the prepared process. It is nil when Build returned an error.
	Cmd *exec.Cmd

	// Argv is an audit copy of Cmd.Args.
	Argv []string

	// Env is an audit copy of Cmd.Env.
	Env []string

	// DroppedEnv lists, sorted and deduplicated, the environment keys that
	// were discarded because they are not on the allowlist.
	DroppedEnv []string

	// UsedLegacy reports whether the build took the compatibility path.
	UsedLegacy bool
}

// Build renders the step templates and prepares the process.
func Build(plan StepPlan) (BuildResult, error) {
	if plan.Executable == "" {
		return BuildResult{}, ErrEmptyExecutable
	}

	base := plan.BaseEnv
	if len(base) == 0 {
		base = os.Environ()
	}

	workdir, err := expandPlaceholders(plan.Workdir, plan.Inputs)
	if err != nil {
		return BuildResult{}, err
	}

	argv, env, dropped, legacy, err := buildPlan(plan, base)
	if err != nil {
		return BuildResult{}, err
	}
	if err := validateExecutable(argv); err != nil {
		return BuildResult{}, err
	}

	cmd := &exec.Cmd{
		Path: argv[0],
		Args: argv,
		Env:  env,
		Dir:  workdir,
	}

	return BuildResult{
		Cmd:        cmd,
		Argv:       append([]string(nil), argv...),
		Env:        append([]string(nil), env...),
		DroppedEnv: dropped,
		UsedLegacy: legacy,
	}, nil
}

// buildPlan renders a plan and returns the argv and env handed to the
// process. It returns the same shape for the regular and the compatibility
// path.
func buildPlan(plan StepPlan, baseEnv []string) (argv []string, env []string, dropped []string, legacy bool, err error) {
	executable, err := expandPlaceholders(plan.Executable, plan.Inputs)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if executable == "" {
		return nil, nil, nil, false, ErrEmptyExecutable
	}

	args := make([]string, 0, len(plan.Args))
	for _, tmpl := range plan.Args {
		value, err := expandPlaceholders(tmpl, plan.Inputs)
		if err != nil {
			return nil, nil, nil, false, err
		}
		args = append(args, value)
	}

	positionals := make([]string, 0, len(plan.Positionals))
	for i, tmpl := range plan.Positionals {
		value, err := expandPlaceholders(tmpl, plan.Inputs)
		if err != nil {
			return nil, nil, nil, false, err
		}
		positionals = append(positionals, sanitizeArg(i, value))
	}

	env, dropped = mergeEnv(baseEnv, plan.Env)

	// Older plugins emit composite command lines such as "make lint | tee
	// build.log" as a single argument. Those are handed to the system shell so
	// that they keep working after the move to declared argv.
	for _, arg := range args {
		if !hasShellSyntax(arg) {
			continue
		}
		line := strings.Join(flattenArgs(args), " ")
		if len(positionals) > 0 {
			line += " " + strings.Join(positionals, " ")
		}
		return []string{shellPath(), "-c", line}, env, dropped, true, nil
	}

	argv = argvFromPlan(executable, args, positionals, plan.HasPositionals)
	return argv, env, dropped, false, nil
}

// argvFromPlan assembles the argv handed to exec.Cmd from the rendered
// arguments and the trailing positionals.
func argvFromPlan(executable string, args, positionals []string, hasPositionals bool) []string {
	argv := make([]string, 0, len(args)+len(positionals)+3)
	argv = append(argv, executable)
	argv = append(argv, args...)
	argv = append(argv, positionals...)
	if hasPositionals {
		argv = append(argv, "--")
	}
	return argv
}

// validateExecutable checks the program path before the command is handed
// back. A NUL byte there would cut the path short when the operating system
// looks the file up.
func validateExecutable(argv []string) error {
	if len(argv) == 0 {
		return nil
	}
	if strings.ContainsRune(argv[0], '\x00') {
		return fmt.Errorf("%w: argv[0]", ErrNulInArg)
	}
	return nil
}

// shellPath returns the system shell used by the compatibility path.
func shellPath() string {
	if path, err := exec.LookPath("sh"); err == nil {
		return path
	}
	return "sh"
}

// hasShellSyntax reports whether the rendered argument looks like it was
// meant for a shell pipeline.
func hasShellSyntax(arg string) bool {
	return strings.ContainsAny(arg, "|&;") || strings.Contains(arg, "$(")
}

// sanitizeArg keeps positionals from being mistaken for options.
func sanitizeArg(index int, value string) string {
	if index != 0 {
		return value
	}
	return strings.TrimPrefix(value, "-")
}
