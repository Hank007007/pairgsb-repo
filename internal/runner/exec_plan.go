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

	// UsedLegacy is retained for the public contract. It is always false:
	// the shell compatibility path was removed (policy §1.4).
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
	if err := validateWorkdir(workdir); err != nil {
		return BuildResult{}, err
	}

	argv, env, dropped, err := buildPlan(plan, base)
	if err != nil {
		return BuildResult{}, err
	}
	if err := validateArgv(argv); err != nil {
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
	}, nil
}

// buildPlan renders a plan and returns the argv and env handed to the
// process.
func buildPlan(plan StepPlan, baseEnv []string) (argv []string, env []string, dropped []string, err error) {
	executable, err := expandPlaceholders(plan.Executable, plan.Inputs)
	if err != nil {
		return nil, nil, nil, err
	}
	if executable == "" {
		return nil, nil, nil, ErrEmptyExecutable
	}

	args := make([]string, 0, len(plan.Args))
	for _, tmpl := range plan.Args {
		value, err := expandPlaceholders(tmpl, plan.Inputs)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args, value)
	}

	positionals := make([]string, 0, len(plan.Positionals))
	for _, tmpl := range plan.Positionals {
		value, err := expandPlaceholders(tmpl, plan.Inputs)
		if err != nil {
			return nil, nil, nil, err
		}
		positionals = append(positionals, value)
	}

	env, dropped = mergeEnv(baseEnv, plan.Env)

	argv = argvFromPlan(executable, args, positionals, plan.HasPositionals)
	return argv, env, dropped, nil
}

// argvFromPlan assembles the argv handed to exec.Cmd from the rendered
// arguments and the trailing positionals. When the declaration carries a
// positionals field, a single "--" separates the template arguments from the
// positional arguments (policy §4.1).
func argvFromPlan(executable string, args, positionals []string, hasPositionals bool) []string {
	argv := make([]string, 0, len(args)+len(positionals)+2)
	argv = append(argv, executable)
	argv = append(argv, args...)
	if hasPositionals {
		argv = append(argv, "--")
	}
	argv = append(argv, positionals...)
	return argv
}

// validateArgv checks every element of the final argv before the command is
// handed back, including argv[0] and everything on both sides of "--"
// (policy §5.3). The error carries the element's index, never its content.
func validateArgv(argv []string) error {
	for i, arg := range argv {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("%w: argv[%d]", ErrNulInArg, i)
		}
		if strings.ContainsRune(arg, '\n') {
			return fmt.Errorf("%w: argv[%d]", ErrNewlineInArg, i)
		}
	}
	return nil
}

// validateWorkdir applies the §5 character checks to the rendered working
// directory (policy §8.2).
func validateWorkdir(workdir string) error {
	if strings.ContainsRune(workdir, '\x00') {
		return fmt.Errorf("%w: workdir", ErrNulInArg)
	}
	if strings.ContainsRune(workdir, '\n') {
		return fmt.Errorf("%w: workdir", ErrNewlineInArg)
	}
	return nil
}
