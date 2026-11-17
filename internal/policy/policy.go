// Package policy defines the in-memory representation of the authorization
// table read from a sudopolicy configuration file.
package policy

import "strings"

// Tag names understood by the evaluator. Tags are written in a rule as a
// word followed by a colon, for example "NOPASSWD:".
const (
	// TagSetenv allows the caller's environment variables to be preserved
	// for the command.
	TagSetenv = "SETENV"
	// TagNoPasswd marks a rule that does not require a password.
	TagNoPasswd = "NOPASSWD"
	// TagPasswd explicitly states that a password is required. It is the
	// default behaviour of a rule.
	TagPasswd = "PASSWD"
)

// Entry is a single authorization rule.
//
// The zero value is not useful; use the parse package to build entries from a
// configuration file.
type Entry struct {
	// Line is the one-based line number the entry was read from.
	Line int

	// Negated reports whether the rule was written with a leading '!'.
	Negated bool

	// Users lists the user patterns the rule applies to. A pattern is a
	// user name, "*", or a group written as "%name".
	Users []string

	// Hosts lists the host patterns the rule applies to. A pattern is a
	// host name or "*".
	Hosts []string

	// Runas lists the target users the rule permits. The first element is
	// the default identity when the caller does not request one.
	Runas []string

	// Tags lists the upper-cased tag names attached to the rule.
	Tags []string

	// Command is the absolute path of the executable the rule governs.
	Command string

	// Args lists the argument patterns the command must carry. An empty
	// list means the command must be executed without arguments.
	Args []string
}

// HasTag reports whether the entry carries the named tag. Tag matching is
// case-insensitive.
func (e Entry) HasTag(name string) bool {
	for _, tag := range e.Tags {
		if strings.EqualFold(tag, name) {
			return true
		}
	}
	return false
}

// Result is the outcome of evaluating one request against a table.
//
// A result is meaningful only when Evaluate returns no error. When Allowed is
// false the Reason field explains why. NeedPassword reports that the matched
// rule requires a password, in which case the executor prompts the operator
// and retries the evaluation with HasPassword set.
type Result struct {
	// Allowed reports whether the command may run as described.
	Allowed bool

	// NeedPassword reports whether the matched rule requires a password.
	NeedPassword bool

	// RunAs is the identity the command must run as. It is empty when the
	// request was denied.
	RunAs string

	// KeepEnv lists the environment variables that survive into the child
	// process.
	KeepEnv []string

	// RuleLine is the one-based line number of the rule that decided the
	// outcome, or zero when no rule matched.
	RuleLine int

	// Reason is a short human readable explanation, set when Allowed is
	// false.
	Reason string
}

// Table is an ordered collection of rules. Order is significant: rules are
// evaluated in the order in which they appear in the configuration file.
type Table struct {
	Entries []Entry
}

// Request describes one command execution the executor wants to authorize.
type Request struct {
	// User is the operator invoking the command.
	User string

	// Groups lists the groups the operator belongs to. Group membership is
	// supplied by the executor and is not read from the policy file.
	Groups []string

	// Host is the name of the target host.
	Host string

	// Runas is the target user the caller asked for. An empty value means
	// the caller did not express a preference.
	Runas string

	// Argv is the full argument vector, with the executable path in
	// Argv[0] and the command arguments after it.
	Argv []string

	// KeepEnv lists the environment variables the caller wants to keep for
	// the command.
	KeepEnv []string

	// HasPassword reports whether the caller has already supplied a
	// password for this request.
	HasPassword bool
}
