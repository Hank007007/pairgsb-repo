// Package eval turns a policy table and a command request into a decision.
package eval

import (
	"errors"

	"sudopolicy/internal/match"
	"sudopolicy/internal/policy"
)

// ErrNoPolicy is returned when Evaluate is called without a table.
var ErrNoPolicy = errors.New("sudopolicy: no policy table")

// ErrEmptyArgv is returned when the request does not carry a command.
var ErrEmptyArgv = errors.New("sudopolicy: request has an empty argv")

// Evaluate applies the policy table to a request and returns the decision.
//
// Evaluation never reads the process environment or the operating system's
// user database; everything it needs is carried by the request and the table.
func Evaluate(table *policy.Table, request policy.Request) (policy.Result, error) {
	if table == nil {
		return policy.Result{}, ErrNoPolicy
	}
	if len(request.Argv) == 0 {
		return policy.Result{}, ErrEmptyArgv
	}

	for _, entry := range table.Entries {
		if result, ok := matchEntry(entry, request); ok {
			return result, nil
		}
	}

	return policy.Result{
		Allowed: false,
		Reason:  "no matching rule",
	}, nil
}

// matchEntry reports whether a single entry applies to the request and, when
// it does, the decision the entry produces.
func matchEntry(entry policy.Entry, request policy.Request) (policy.Result, bool) {
	if !anyUser(entry.Users, request) {
		return policy.Result{}, false
	}
	if !anyHost(entry.Hosts, request.Host) {
		return policy.Result{}, false
	}
	if !match.Command(entry.Command, request.Argv[0]) {
		return policy.Result{}, false
	}
	if !match.Args(entry.Args, request.Argv[1:]) {
		return policy.Result{}, false
	}

	runAs, ok := resolveRunAs(entry.Runas, request.Runas)
	if !ok {
		return policy.Result{}, false
	}

	keepEnv := request.KeepEnv

	result := policy.Result{
		Allowed:  true,
		RunAs:    runAs,
		KeepEnv:  keepEnv,
		RuleLine: entry.Line,
	}
	if !entry.HasTag(policy.TagNoPasswd) {
		result.NeedPassword = true
		if !request.HasPassword {
			result.Allowed = false
			result.RunAs = ""
			result.KeepEnv = nil
			result.Reason = "password required"
		}
	}
	return result, true
}

func anyUser(patterns []string, request policy.Request) bool {
	for _, pattern := range patterns {
		if match.User(pattern, request.User, request.Groups) {
			return true
		}
	}
	return false
}

func anyHost(patterns []string, host string) bool {
	for _, pattern := range patterns {
		if match.Host(pattern, host) {
			return true
		}
	}
	return false
}

// resolveRunAs picks the identity the command runs as. An empty request asks
// for the entry's default, which is its first listed target user.
func resolveRunAs(permitted []string, requested string) (string, bool) {
	if requested == "" {
		if len(permitted) == 0 {
			return "", false
		}
		return permitted[0], true
	}
	for _, name := range permitted {
		if name == requested {
			return requested, true
		}
	}
	return "", false
}
