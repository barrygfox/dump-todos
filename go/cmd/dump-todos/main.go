package main

import (
	"context"
	"fmt"
	"os"

	"dump-todos-go/internal/auth"
	"dump-todos-go/internal/config"
	"dump-todos-go/internal/export"
	"dump-todos-go/internal/graph"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	token, err := auth.AcquireToken(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Authenticated. Fetching tasks")
	if cfg.IncompleteOnly {
		fmt.Fprintf(os.Stderr, " (incomplete only)")
	}
	fmt.Fprintln(os.Stderr, "...")

	client := graph.NewClient(token)
	lists, err := client.Lists(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	content, err := export.Markdown(ctx, client, lists, cfg.IncompleteOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.OutputPath == "" {
		fmt.Print(content)
		if content != "" && content[len(content)-1] != '\n' {
			fmt.Println()
		}
		return
	}

	if err := os.WriteFile(cfg.OutputPath, []byte(content), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Done. Written to %s\n", cfg.OutputPath)
}
