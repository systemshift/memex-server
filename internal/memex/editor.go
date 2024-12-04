package memex

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

var (
	ErrEditorClosed = errors.New("editor is closed")
)

// Editor represents a simple terminal-based text editor
type Editor struct {
	tempFile   string   // Path to temporary file
	cursorX    int      // Cursor X position
	cursorY    int      // Cursor Y position
	screenRows int      // Terminal height
	screenCols int      // Terminal width
	repoName   string   // Repository name
	content    [][]rune // Current screen content (for display only)
	closed     bool     // Whether the editor is closed
}

// NewEditor creates a new editor instance
func NewEditor(repoPath string) *Editor {
	rows, cols, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		rows = 24
		cols = 80
	}

	// Create temporary file without extension
	tmpFile, err := os.CreateTemp("", "memex-*")
	if err != nil {
		return nil
	}
	tmpFile.Close()

	return &Editor{
		tempFile:   tmpFile.Name(),
		content:    [][]rune{{}}, // Start with one empty line
		screenRows: rows - 3,     // Leave room for title and status line
		screenCols: cols,
		repoName:   filepath.Base(repoPath),
		closed:     false,
	}
}

// GetTempFile returns the path to the temporary file
func (e *Editor) GetTempFile() string {
	return e.tempFile
}

// Close cleans up resources
func (e *Editor) Close() {
	if !e.closed && e.tempFile != "" {
		os.Remove(e.tempFile)
		e.closed = true
		e.tempFile = ""
	}
}

// WriteContent writes content to the temporary file
func (e *Editor) WriteContent(content []byte) error {
	if e.closed {
		return ErrEditorClosed
	}
	return os.WriteFile(e.tempFile, content, 0600) // More secure permissions for temp files
}

// ReadContent reads content from the temporary file
func (e *Editor) ReadContent() ([]byte, error) {
	if e.closed {
		return nil, ErrEditorClosed
	}
	return os.ReadFile(e.tempFile)
}

// Run starts the editor and returns the edited content
func (e *Editor) Run() (string, error) {
	if e.closed {
		return "", ErrEditorClosed
	}

	// Switch to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Load initial content
	content, err := e.ReadContent()
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	e.loadContent(content)

	for {
		e.refreshScreen()

		// Read a key
		buf := make([]byte, 1)
		os.Stdin.Read(buf)

		switch buf[0] {
		case Ctrl('q'): // Ctrl-Q to quit
			fmt.Print("\x1b[2J") // Clear screen
			fmt.Print("\x1b[H")  // Move cursor to top
			fmt.Print("\n")      // Add newline for clean exit
			return "", nil
		case Ctrl('s'): // Ctrl-S to save
			content := e.getContent()
			if err := e.WriteContent([]byte(content)); err != nil {
				return "", err
			}
			fmt.Print("\x1b[2J") // Clear screen
			fmt.Print("\x1b[H")  // Move cursor to top
			fmt.Print("\n")      // Add newline for clean exit
			return content, nil
		case 13: // Enter
			e.insertNewline()
		case 127: // Backspace
			e.handleBackspace()
		default:
			if !IsCntrl(buf[0]) {
				e.insertChar(rune(buf[0]))
			}
		}
	}
}

// loadContent loads content into the editor buffer
func (e *Editor) loadContent(content []byte) {
	if len(content) == 0 {
		e.content = [][]rune{{}}
		return
	}

	lines := strings.Split(string(content), "\n")
	e.content = make([][]rune, len(lines))
	for i, line := range lines {
		e.content[i] = []rune(line)
	}
}

func (e *Editor) refreshScreen() {
	// Clear screen
	fmt.Print("\x1b[2J")
	fmt.Print("\x1b[H")

	// Draw title
	fmt.Print("\x1b[7m") // Invert colors
	title := fmt.Sprintf(" %s ", e.repoName)
	padding := e.screenCols - len(title)
	if padding > 0 {
		title += strings.Repeat(" ", padding)
	}
	fmt.Print(title)
	fmt.Print("\x1b[m") // Reset colors
	fmt.Print("\r\n")

	// Draw content
	for i, line := range e.content {
		if i >= e.screenRows {
			break
		}
		fmt.Print(string(line))
		fmt.Print("\r\n")
	}

	// Draw status line
	fmt.Print("\x1b[7m") // Invert colors
	status := fmt.Sprintf("Ctrl-S = Save | Ctrl-Q = Quit | Lines: %d", len(e.content))
	if len(status) > e.screenCols {
		status = status[:e.screenCols]
	}
	padding = e.screenCols - len(status)
	if padding > 0 {
		status += strings.Repeat(" ", padding)
	}
	fmt.Print(status)
	fmt.Print("\x1b[m") // Reset colors

	// Position cursor
	fmt.Printf("\x1b[%d;%dH", e.cursorY+2, e.cursorX+1) // +2 to account for title line
}

func (e *Editor) insertChar(ch rune) {
	if e.cursorY >= len(e.content) {
		e.content = append(e.content, []rune{})
	}
	line := e.content[e.cursorY]
	if e.cursorX >= len(line) {
		line = append(line, ch)
	} else {
		line = append(line[:e.cursorX+1], line[e.cursorX:]...)
		line[e.cursorX] = ch
	}
	e.content[e.cursorY] = line
	e.cursorX++
}

func (e *Editor) insertNewline() {
	if e.cursorY >= len(e.content) {
		e.content = append(e.content, []rune{})
	}
	line := e.content[e.cursorY]
	newLine := make([]rune, len(line[e.cursorX:]))
	copy(newLine, line[e.cursorX:])
	e.content[e.cursorY] = line[:e.cursorX]
	e.content = append(e.content[:e.cursorY+1], append([][]rune{newLine}, e.content[e.cursorY+1:]...)...)
	e.cursorY++
	e.cursorX = 0
}

func (e *Editor) handleBackspace() {
	if e.cursorX > 0 {
		line := e.content[e.cursorY]
		e.content[e.cursorY] = append(line[:e.cursorX-1], line[e.cursorX:]...)
		e.cursorX--
	} else if e.cursorY > 0 {
		// Join with previous line
		prevLine := e.content[e.cursorY-1]
		e.cursorX = len(prevLine)
		e.content[e.cursorY-1] = append(prevLine, e.content[e.cursorY]...)
		e.content = append(e.content[:e.cursorY], e.content[e.cursorY+1:]...)
		e.cursorY--
	}
}

func (e *Editor) getContent() string {
	var content string
	for i, line := range e.content {
		if i > 0 {
			content += "\n"
		}
		content += string(line)
	}
	return content
}

// Ctrl converts a character to its control sequence value
func Ctrl(b byte) byte {
	return b & 0x1f
}

// IsCntrl checks if a byte is a control character
func IsCntrl(b byte) bool {
	return b < 32 || b == 127
}
