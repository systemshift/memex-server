package main

import (
	"flag"
	"fmt"
	"os"

	"memex/internal/memex"
)

func showHelp() {
	fmt.Println(`memex - A personal knowledge graph

Usage:
  memex [command] [arguments]

Commands:
  init <name>     Create a new repository
  connect <path>  Connect to an existing repository
  status          Show repository status
  add <file>      Add a file to the repository
  delete <id>     Delete an object
  link <src> <dst> <type> [note]  Create a link between objects
  links <id>      Show links for an object
  help            Show this help message

When no command is provided and a repository is connected:
  Opens an editor to create a new note

Examples:
  memex init my_repo              Create a new repository
  memex connect my_repo.mx        Connect to existing repository
  memex status                    Show repository status
  memex add document.txt          Add a file
  memex link abc123 def456 ref    Create a reference link
  memex links abc123              Show links for an object

For more information, visit: https://github.com/your/memex`)
	os.Exit(0)
}

func main() {
	// Handle help flags
	flag.Usage = showHelp
	help := flag.Bool("help", false, "Show help message")
	h := flag.Bool("h", false, "Show help message")
	flag.Parse()

	if *help || *h {
		showHelp()
	}

	args := flag.Args()

	// Handle help command
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help") {
		showHelp()
	}

	// Handle init and connect commands first
	if len(args) > 0 {
		switch args[0] {
		case "init":
			if len(args) != 2 {
				fmt.Println("Error: Repository name required")
				showHelp()
			}
			if err := memex.InitCommand(args[1]); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return

		case "connect":
			if len(args) != 2 {
				fmt.Println("Error: Repository path required")
				showHelp()
			}
			if err := memex.ConnectCommand(args[1]); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	// For all other commands, need a connected repo
	if err := memex.OpenRepository(); err != nil {
		fmt.Printf("Error: %v\n", err)
		showHelp()
	}

	defer memex.CloseRepository()

	// If no command provided, open editor
	if len(args) == 0 {
		if err := memex.EditCommand(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Execute command
	cmd := args[0]
	args = args[1:]

	var err error
	switch cmd {
	case "status":
		err = memex.StatusCommand()

	case "add":
		if len(args) != 1 {
			fmt.Println("Error: File path required")
			showHelp()
		}
		err = memex.AddCommand(args[0])

	case "delete":
		if len(args) != 1 {
			fmt.Println("Error: ID required")
			showHelp()
		}
		err = memex.DeleteCommand(args[0])

	case "link":
		if len(args) < 3 {
			fmt.Println("Error: Source, target, and link type required")
			showHelp()
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
		}
		err = memex.LinkCommand(args[0], args[1], args[2], note)

	case "links":
		if len(args) != 1 {
			fmt.Println("Error: ID required")
			showHelp()
		}
		err = memex.LinksCommand(args[0])

	default:
		fmt.Printf("Error: Unknown command: %s\n", cmd)
		showHelp()
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
