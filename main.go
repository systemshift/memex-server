package main

import (
	"flag"
	"fmt"
	"os"
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
		if err := editCommand(); err != nil {
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
		if err := initCommand(initCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "add":
		addCmd.Parse(os.Args[2:])
		if addCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex add <file>\n")
			os.Exit(1)
		}
		if err := addCommand(addCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "commit":
		commitCmd.Parse(os.Args[2:])
		if *commitMsg == "" {
			fmt.Fprintf(os.Stderr, "Usage: memex commit -m \"commit message\"\n")
			os.Exit(1)
		}
		if err := commitCommand(*commitMsg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "log":
		if err := logCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "restore":
		restoreCmd.Parse(os.Args[2:])
		if restoreCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex restore <commit-hash>\n")
			os.Exit(1)
		}
		if err := restoreCommand(restoreCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		// Unknown command, default to edit
		if err := editCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
