// Command condgrate evaluates one authorization request against a policy chain.
//
// Usage:
//
//	condgrate -chain examples/chain.json -request examples/request.json -directory examples/directory.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"condgrate/internal/attr"
	"condgrate/internal/engine"
	"condgrate/internal/policy"
)

type requestDoc struct {
	Action    string         `json:"action"`
	Principal map[string]any `json:"principal"`
	Resource  map[string]any `json:"resource"`
	Context   map[string]any `json:"context"`
}

type directoryDoc struct {
	Principal struct {
		Values map[string]any    `json:"values"`
		Types  map[string]string `json:"types"`
	} `json:"principal"`
	Resource struct {
		Values map[string]any    `json:"values"`
		Types  map[string]string `json:"types"`
	} `json:"resource"`
	Context struct {
		Values map[string]any    `json:"values"`
		Types  map[string]string `json:"types"`
	} `json:"context"`
}

func main() {
	chainPath := flag.String("chain", "", "path to the policy chain JSON")
	requestPath := flag.String("request", "", "path to the request JSON")
	dirPath := flag.String("directory", "", "optional path to the directory snapshot JSON")
	flag.Parse()

	if *chainPath == "" || *requestPath == "" {
		fmt.Fprintln(os.Stderr, "condgrate: -chain and -request are required")
		flag.Usage()
		os.Exit(2)
	}

	chain, err := policy.LoadChain(*chainPath)
	if err != nil {
		fail("load chain: %v", err)
	}

	req, err := readRequest(*requestPath)
	if err != nil {
		fail("read request: %v", err)
	}

	dir := attr.NewDirectory()
	if *dirPath != "" {
		if err := loadDirectory(dir, *dirPath); err != nil {
			fail("load directory: %v", err)
		}
	}

	eng := engine.New(chain, dir)
	resp := eng.Decide(engine.Request{
		Action:    req.Action,
		Principal: req.Principal,
		Resource:  req.Resource,
		Context:   req.Context,
	})

	fmt.Printf("effect:       %s\n", resp.Effect)
	fmt.Printf("reason:       %s\n", resp.Reason)
	fmt.Printf("matched:      %v\n", resp.Matched)
	fmt.Printf("obligations:  %v\n", resp.Obligations)
	fmt.Printf("indeterminate: %v\n", resp.Indeterminate)
}

func readRequest(path string) (requestDoc, error) {
	var doc requestDoc
	data, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	return doc, nil
}

func loadDirectory(dir *attr.Directory, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc directoryDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	dir.SetPrincipal(doc.Principal.Values, doc.Principal.Types)
	dir.SetResource(doc.Resource.Values, doc.Resource.Types)
	dir.SetContext(doc.Context.Values, doc.Context.Types)
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "condgrate: "+format+"\n", args...)
	os.Exit(1)
}
