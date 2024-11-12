package memex

import (
	"fmt"
	"os"

	"memex/internal/memex/mx"

	"golang.org/x/term"
)

// Editor represents a simple terminal-based text editor
type Editor struct {
	content    [][]rune
	cursorX    int
	cursorY    int
	screenRows int
	screenCols int
}

// NewEditor creates a new editor instance
func NewEditor() *Editor {
	rows, cols, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		rows = 24
		cols = 80
	}
	return &Editor{
		content:    [][]rune{{}}, // Start with one empty line
		screenRows: rows - 2,     // Leave room for status line
		screenCols: cols,
	}
}

// Run starts the editor and returns the edited content
func (e *Editor) Run() (string, error) {
	// Switch to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

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

func (e *Editor) refreshScreen() {
	// Clear screen
	fmt.Print("\x1b[2J")
	fmt.Print("\x1b[H")

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
	fmt.Print(status)
	fmt.Print("\x1b[m") // Reset colors

	// Position cursor
	fmt.Printf("\x1b[%d;%dH", e.cursorY+1, e.cursorX+1)
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
	// Get raw content
	var content string
	for i, line := range e.content {
		if i > 0 {
			content += "\n"
		}
		content += string(line)
	}

	// Create MX document
	doc := mx.New(content)

	// Try to use first non-empty line as title
	for _, line := range e.content {
		if len(line) > 0 {
			doc.SetTitle(string(line))
			break
		}
	}

	return doc.String()
}

// Ctrl converts a character to its control sequence value
func Ctrl(b byte) byte {
	return b & 0x1f
}

// IsCntrl checks if a byte is a control character
func IsCntrl(b byte) bool {
	return b < 32 || b == 127
}
