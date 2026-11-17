// Command sudopolicy evaluates one command request against a policy file and
// prints the decision as a single line.
//
// Usage:
//
//	sudopolicy -policy policy/sudopolicy.conf -user ops -groups ops -host web1 \
//	    -runas root -env LANG -password -- /bin/ls -la
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"sudopolicy"
)

func main() {
	policyPath := flag.String("policy", "policy/sudopolicy.conf", "path to the policy file")
	user := flag.String("user", "", "operator user name")
	groups := flag.String("groups", "", "comma separated operator groups")
	host := flag.String("host", "", "target host name")
	runas := flag.String("runas", "", "requested target user")
	keepEnv := flag.String("env", "", "comma separated environment variables to keep")
	hasPassword := flag.Bool("password", false, "a password has been supplied")
	flag.Parse()

	argv := flag.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "sudopolicy: missing command")
		flag.Usage()
		os.Exit(2)
	}

	table, err := sudopolicy.Load(*policyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sudopolicy: %v\n", err)
		os.Exit(1)
	}

	request := sudopolicy.Request{
		User:        *user,
		Groups:      splitList(*groups),
		Host:        *host,
		Runas:       *runas,
		Argv:        argv,
		KeepEnv:     splitList(*keepEnv),
		HasPassword: *hasPassword,
	}

	result, err := sudopolicy.Evaluate(table, request)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sudopolicy: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(format(result))
	if !result.Allowed {
		os.Exit(1)
	}
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func format(result sudopolicy.Result) string {
	var b strings.Builder
	if result.Allowed {
		b.WriteString("allow")
	} else {
		b.WriteString("deny")
	}
	if result.Reason != "" {
		fmt.Fprintf(&b, " reason=%q", result.Reason)
	}
	if result.RunAs != "" {
		fmt.Fprintf(&b, " runas=%s", result.RunAs)
	}
	if result.NeedPassword {
		b.WriteString(" password=required")
	}
	if len(result.KeepEnv) > 0 {
		env := append([]string(nil), result.KeepEnv...)
		sort.Strings(env)
		fmt.Fprintf(&b, " env=%s", strings.Join(env, ","))
	}
	if result.RuleLine > 0 {
		fmt.Fprintf(&b, " line=%d", result.RuleLine)
	}
	return b.String()
}
