package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"memex/internal/memex"
)

func usage() {
	fmt.Printf(`Usage: %s <command> [arguments]

Commands:
  init <name>                Create a new repository
  connect <path>            Connect to existing repository
  add <file>               Add a file to repository
  delete <id>              Delete a node
  edit                     Open editor for a new note
  link <src> <dst> <type>  Create a link between nodes
  links <id>               Show links for a node
  status                   Show repository status
  export <path>            Export repository to tar archive
  import <path>            Import repository from tar archive

Export options:
  --nodes <id1,id2,...>    Export specific nodes and their subgraph
  --depth <n>              Maximum depth for subgraph export

Import options:
  --on-conflict <strategy> How to handle ID conflicts (skip, replace, rename)
  --merge                  Merge with existing content
  --prefix <prefix>        Add prefix to imported node IDs

`, filepath.Base(os.Args[0]))
	os.Exit(1)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() < 1 {
		// No command provided, open editor for new note
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		if err := memex.EditCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cmd := flag.Arg(0)
	args := flag.Args()[1:]

	var err error
	switch cmd {
	case "init":
		err = memex.InitCommand(args...)

	case "connect":
		err = memex.ConnectCommand(args...)

	case "add":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.AddCommand(args...)

	case "delete":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.DeleteCommand(args...)

	case "edit":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.EditCommand(args...)

	case "link":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.LinkCommand(args...)

	case "links":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.LinksCommand(args...)

	case "status":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.StatusCommand(args...)

	case "export":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.ExportCommand(args...)

	case "import":
		// Try to open repository in current directory
		if err := memex.OpenRepository(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer memex.CloseRepository()

		err = memex.ImportCommand(args...)

	default:
		usage()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
