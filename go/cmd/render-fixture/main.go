package main

import (
	"encoding/json"
	"fmt"
	"os"

	"dump-todos-go/internal/export"
)

type fixture struct {
	Lists []export.List `json:"lists"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: render-fixture <fixture-path> [--incomplete]")
		os.Exit(1)
	}

	fixturePath := os.Args[1]
	incompleteOnly := false
	for _, arg := range os.Args[2:] {
		if arg == "--incomplete" {
			incompleteOnly = true
		}
	}

	data, err := os.ReadFile(fixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var parsed fixture
	if err := json.Unmarshal(data, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	output := export.RenderLists(parsed.Lists, incompleteOnly)
	fmt.Print(output)
	if output != "" && output[len(output)-1] != '\n' {
		fmt.Println()
	}
}
