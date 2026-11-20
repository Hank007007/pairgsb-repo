// Command patchdemo runs the scenarios in testdata/scenarios.json through the
// patch layer and prints each document before and after.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"patchsieve/internal/patch"
	"patchsieve/internal/policy"
)

type scenarioFile struct {
	Scenarios []scenario `json:"scenarios"`
}

type scenario struct {
	Name      string           `json:"name"`
	Note      string           `json:"note"`
	Principal policy.Principal `json:"principal"`
	Document  json.RawMessage  `json:"document"`
	Patch     json.RawMessage  `json:"patch"`
}

func main() {
	path := flag.String("file", "testdata/scenarios.json", "scenario file")
	flag.Parse()

	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, "patchdemo:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file scenarioFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	for i, sc := range file.Scenarios {
		doc, err := patch.DecodeDocument(sc.Document)
		if err != nil {
			return fmt.Errorf("scenario %q: %w", sc.Name, err)
		}
		before, err := patch.EncodeDocument(doc.Value)
		if err != nil {
			return fmt.Errorf("scenario %q: %w", sc.Name, err)
		}
		ops, err := patch.ParseRequest(sc.Patch)
		if err != nil {
			return fmt.Errorf("scenario %q: %w", sc.Name, err)
		}

		result, applyErr := patch.Apply(doc.Value, ops, sc.Principal)
		after, err := patch.EncodeDocument(result)
		if err != nil {
			return fmt.Errorf("scenario %q: %w", sc.Name, err)
		}

		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("== %s ==\n", sc.Name)
		if sc.Note != "" {
			fmt.Printf("note:   %s\n", sc.Note)
		}
		fmt.Printf("before: %s\n", before)
		fmt.Printf("after:  %s\n", after)
		if applyErr != nil {
			fmt.Printf("error:  %s\n", patch.CodeOf(applyErr))
		}
	}
	return nil
}
