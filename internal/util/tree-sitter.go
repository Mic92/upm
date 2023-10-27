package util

import (
	"context"
	"fmt"
	"os"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func QueryImportsFromTreeSitter(path string, lang *sitter.Language, queryImports string) ([]string, error) {
	importPaths := []string{}

	contents, err := os.ReadFile(path)
	if err != nil {
		return importPaths, fmt.Errorf("failed to read file: %w", err)
	}

	query, err := sitter.NewQuery([]byte(queryImports), lang)
	if err != nil {
		return importPaths, fmt.Errorf("failed to create query: %w", err)
	}

	qc := sitter.NewQueryCursor()

	node, err := sitter.ParseCtx(context.Background(), contents, lang)
	if err != nil {
		return importPaths, fmt.Errorf("failed to parse file: %w", err)
	}

	qc.Exec(query, node)

	for {
		match, ok := qc.NextMatch()
		if !ok {
			break
		}

		match = qc.FilterPredicates(match, contents)
		if len(match.Captures) == 0 {
			continue
		}

		var importPath string
		for _, capture := range match.Captures {
			if query.CaptureNameForId(capture.Index) == "import" {
				importPath = capture.Node.Content(contents)
				break
			}
		}

		if importPath == "" {
			// shouldn't happen, with the way the query is written and handled.
			return importPaths, fmt.Errorf("empty import path")
		}

		importPaths = append(importPaths, strings.Trim(importPath, "'\"`"))
	}

	return importPaths, nil
}
