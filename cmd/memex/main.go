package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"memex/internal/memex"
)

func main() {
	// Subcommands
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	commitCmd := flag.NewFlagSet("commit", flag.ExitOnError)
	commitMsg := commitCmd.String("m", "", "Commit message")
	addCmd := flag.NewFlagSet("add", flag.ExitOnError)
	showCmd := flag.NewFlagSet("show", flag.ExitOnError)
	linkCmd := flag.NewFlagSet("link", flag.ExitOnError)
	linkType := linkCmd.String("type", "references", "Type of link")
	linkNote := linkCmd.String("note", "", "Note about the link")
	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	searchQuery := searchCmd.String("q", "", "Search query (key=value,...)")

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

	case "show":
		showCmd.Parse(os.Args[2:])
		if showCmd.NArg() != 1 {
			fmt.Fprintf(os.Stderr, "Usage: memex show <id>\n")
			os.Exit(1)
		}
		if err := memex.ShowCommand(showCmd.Arg(0)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "link":
		linkCmd.Parse(os.Args[2:])
		if linkCmd.NArg() != 2 {
			fmt.Fprintf(os.Stderr, "Usage: memex link [-type <type>] [-note <note>] <source-id> <target-id>\n")
			os.Exit(1)
		}
		if err := memex.LinkCommand(linkCmd.Arg(0), linkCmd.Arg(1), *linkType, *linkNote); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "search":
		searchCmd.Parse(os.Args[2:])
		query := make(map[string]any)
		if *searchQuery != "" {
			// Parse key=value pairs
			pairs := strings.Split(*searchQuery, ",")
			for _, pair := range pairs {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					query[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}
		}
		if err := memex.SearchCommand(query); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "status":
		if err := memex.StatusCommand(); err != nil {
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
		if len(os.Args) != 3 {
			fmt.Fprintf(os.Stderr, "Usage: memex restore <commit-hash>\n")
			os.Exit(1)
		}
		if err := memex.RestoreCommand(os.Args[2]); err != nil {
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
