package main

import (
	"flag"
	"fmt"
	"os"

	"memex/internal/memex"
)

func main() {
	// Subcommands
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	commitCmd := flag.NewFlagSet("commit", flag.ExitOnError)
	commitMsg := commitCmd.String("m", "", "Commit message")
	restoreCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)

	// Parse command
	if len(os.Args) < 2 {
		// No command provided, default to edit
		if err := memex.EditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		if initCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex init <directory>\n")
			os.Exit(1)
		}
		if err := memex.InitCommand(initCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "add":
		addCmd.Parse(os.Args[2:])
		if addCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex add <file>\n")
			os.Exit(1)
		}
		if err := memex.AddCommand(addCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "commit":
		commitCmd.Parse(os.Args[2:])
		if *commitMsg == "" {
			fmt.Fprintf(os.Stderr, "Usage: memex commit -m \"commit message\"\n")
			os.Exit(1)
		}
		if err := memex.CommitCommand(*commitMsg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "log":
		if err := memex.LogCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "restore":
		restoreCmd.Parse(os.Args[2:])
		if restoreCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex restore <commit-hash>\n")
			os.Exit(1)
		}
		if err := memex.RestoreCommand(restoreCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		// Unknown command, default to edit
		if err := memex.EditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
