package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/systemshift/memex/internal/memex"
)

var (
	showVersion = flag.Bool("version", false, "Show version information")
)

func printVersion() {
	fmt.Println(memex.BuildInfo())
	os.Exit(0)
}

func usage() {
	fmt.Printf(`Usage: %s <command> [arguments]

Built-in Commands:
  init <name>                Create a new repository
  connect <path>            Connect to existing repository
  add <file>               Add a file to repository
  delete <id>              Delete a node
  edit                     Open editor for a new note
  link <src> <dst> <type>  Create a link between nodes
  links <id>               Show links for a node
  status                   Show repository status
  version                  Show version information
  export <path>            Export repository to tar archive
  import <path>            Import repository from tar archive

Module Commands:
  module install <source>  Install a module from a source
  module remove <id>       Remove a module
  module list              List installed modules
  <module-id> <command>    Execute a module command

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
	// Import flags
	importFlags := flag.NewFlagSet("import", flag.ExitOnError)
	onConflict := importFlags.String("on-conflict", "skip", "How to handle ID conflicts (skip, replace, rename)")
	merge := importFlags.Bool("merge", false, "Merge with existing content")
	prefix := importFlags.String("prefix", "", "Add prefix to imported node IDs")

	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		printVersion()
	}

	cmds := memex.NewCommands()
	defer cmds.Close()

	if flag.NArg() < 1 {
		// No command provided
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if err := cmds.Edit(); err != nil {
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
		if len(args) != 1 {
			usage()
		}
		err = cmds.Init(args[0])

	case "connect":
		if len(args) != 1 {
			usage()
		}
		err = cmds.Connect(args[0])

	case "add":
		if len(args) != 1 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Add(args[0])

	case "delete":
		if len(args) != 1 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Delete(args[0])

	case "edit":
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Edit()

	case "link":
		if len(args) < 3 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		note := ""
		if len(args) > 3 {
			note = args[3]
		}
		err = cmds.Link(args[0], args[1], args[2], note)

	case "links":
		if len(args) != 1 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Links(args[0])

	case "status":
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Status()

	case "version":
		if err := cmds.AutoConnect(); err == nil {
			// If connected to a repo, show its version too
			err = cmds.ShowVersion()
		} else {
			// Just show memex version
			fmt.Println(memex.BuildInfo())
		}

	case "export":
		if len(args) != 1 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		err = cmds.Export(args[0])

	case "import":
		if len(args) < 1 {
			usage()
		}
		if err := cmds.AutoConnect(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Parse import flags
		importFlags.Parse(args[1:])
		opts := memex.ImportOptions{
			OnConflict: *onConflict,
			Merge:      *merge,
			Prefix:     *prefix,
		}

		err = cmds.Import(args[0], opts)

	case "module":
		err = cmds.Module(args...)

	default:
		// Check if it's a module command
		if err := cmds.AutoConnect(); err == nil {
			// Try to handle as a module command
			err = cmds.HandleModule(cmd, args...)
		} else {
			usage()
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
