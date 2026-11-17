// Command gqladmit is a small front door for a GraphQL gateway. It reads
// queries and emits the admission decision that the gateway would apply.
//
// Usage:
//
//	gqladmit -tenant acme -max-depth 8 -max-fields 200 < query.graphql
//	echo '{ viewer { id } }' | gqladmit -tenant acme
//
// The exit status is 0 when the query is admitted and 1 when it is rejected.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gqladmit/internal/admit"
	"gqladmit/internal/policy"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "gqladmit: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gqladmit", flag.ContinueOnError)
	fs.SetOutput(stderr)

	tenant := fs.String("tenant", "public", "tenant the request is made for")
	maxDepth := fs.Int("max-depth", 8, "maximum selection depth")
	maxFields := fs.Int("max-fields", 200, "maximum number of selected fields")
	restricted := fs.String("restricted", "userSecret,billingInfo", "comma separated list of restricted fields")
	granted := fs.String("granted", "", "comma separated list of fields granted to the tenant")
	file := fs.String("file", "", "read the query from this file instead of stdin")

	if err := fs.Parse(args); err != nil {
		return err
	}

	query := strings.Join(fs.Args(), " ")
	if *file != "" {
		data, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		query = string(data)
	} else if query == "" {
		data, err := io.ReadAll(bufio.NewReader(stdin))
		if err != nil {
			return err
		}
		query = string(data)
	}

	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("no query provided")
	}

	var grantFields []string
	if *granted != "" {
		grantFields = splitList(*granted)
	}

	engine := admit.New(
		admit.Limits{MaxDepth: *maxDepth, MaxFields: *maxFields},
		policy.NewIntrospectionGuard(),
		policy.NewAuthorizer(splitList(*restricted), map[string][]string{*tenant: grantFields}),
	)

	decision, err := engine.Admit(*tenant, query)
	if err != nil {
		fmt.Fprintf(stdout, "rejected: %v\n", err)
		return fmt.Errorf("request rejected")
	}

	fmt.Fprintf(stdout, "admitted: %s depth=%d fields=%d\n",
		decision.Operation, decision.Depth, decision.Fields)
	return nil
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
