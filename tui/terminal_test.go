package tui

import (
	"strings"
	"testing"
)

func TestTerminal_BasicWrite(t *testing.T) {
	term := NewTerminal(80, 24)
	term.Feed([]byte("hello"))
	rendered := term.Render()
	lines := strings.Split(rendered, "\n")
	if !strings.HasPrefix(lines[0], "hello") {
		t.Errorf("Expected line 0 to start with 'hello', got: %q", lines[0][:min8(len(lines[0]), 20)])
	}
}

func TestTerminal_CursorPositionAndOverwrite(t *testing.T) {
	term := NewTerminal(80, 24)
	term.Feed([]byte("AB"))
	term.Feed([]byte("\033[1;1H")) // cursor to row 1, col 1 (1-based)
	term.Feed([]byte("X"))
	lines := strings.Split(term.Render(), "\n")
	if !strings.HasPrefix(lines[0], "XB") {
		t.Errorf("Expected line 0 to start with 'XB', got: %q", lines[0][:min8(len(lines[0]), 20)])
	}
}

func TestTerminal_ClearLine(t *testing.T) {
	term := NewTerminal(80, 24)
	term.Feed([]byte("hello\033[1;1H\033[2K")) // write, home, erase line
	lines := strings.Split(term.Render(), "\n")
	// Erase-in-line 2 clears the entire line; it should be all spaces.
	if strings.TrimSpace(stripANSI(lines[0])) != "" {
		t.Errorf("Expected cleared line to be blank, got: %q", lines[0][:min8(len(lines[0]), 20)])
	}
}

func TestTerminal_Resize(t *testing.T) {
	term := NewTerminal(80, 24)
	term.Feed([]byte("hello"))
	term.Resize(40, 10)
	if term.cols != 40 || term.rows != 10 {
		t.Errorf("Expected 40×10 after resize, got %d×%d", term.cols, term.rows)
	}
	rendered := term.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 rendered rows, got %d", len(lines))
	}
}

func TestTerminal_ResizeSmallerBoundsCheck(t *testing.T) {
	term := NewTerminal(80, 24)
	term.cx = 79
	term.cy = 23
	term.Resize(40, 10)
	if term.cx >= term.cols {
		t.Errorf("cursor cx %d out of bounds after resize (cols=%d)", term.cx, term.cols)
	}
	if term.cy >= term.rows {
		t.Errorf("cursor cy %d out of bounds after resize (rows=%d)", term.cy, term.rows)
	}
}

func TestTerminal_ScrollbackBuffer(t *testing.T) {
	term := NewTerminal(80, 5)
	// Writing 10 lines into a 5-row terminal forces 5 lines into scrollback.
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\r\n"))
	}
	if len(term.scrollback) < 5 {
		t.Errorf("Expected ≥5 scrollback lines, got %d", len(term.scrollback))
	}
}

func TestTerminal_ScrollViewUpDown(t *testing.T) {
	term := NewTerminal(80, 5)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\r\n"))
	}
	term.ScrollViewUp(3)
	if term.viewOffset != 3 {
		t.Errorf("Expected viewOffset=3, got %d", term.viewOffset)
	}
	if !term.IsScrolled() {
		t.Error("Expected IsScrolled() == true after ScrollViewUp")
	}
	term.ScrollViewDown(3)
	if term.viewOffset != 0 {
		t.Errorf("Expected viewOffset=0 after ScrollViewDown, got %d", term.viewOffset)
	}
	if term.IsScrolled() {
		t.Error("Expected IsScrolled() == false after full scroll down")
	}
}

func TestTerminal_ScrollViewReset(t *testing.T) {
	term := NewTerminal(80, 5)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\r\n"))
	}
	term.ScrollViewUp(100) // clamp to actual scrollback size
	term.ScrollViewReset()
	if term.viewOffset != 0 {
		t.Errorf("Expected viewOffset=0 after Reset, got %d", term.viewOffset)
	}
}

func TestTerminal_SGRColors(t *testing.T) {
	term := NewTerminal(80, 24)
	// ESC[32m sets foreground green (color index 2)
	term.Feed([]byte("\033[32mhello\033[0m"))
	if term.grid[0][0].attr.fg != 2 {
		t.Errorf("Expected fg=2 (green), got fg=%d", term.grid[0][0].attr.fg)
	}
	if term.grid[0][5].attr.fg != attrDefault {
		t.Errorf("Expected reset fg after ESC[0m, got fg=%d", term.grid[0][5].attr.fg)
	}
}

func TestTerminal_RenderRowCount(t *testing.T) {
	term := NewTerminal(40, 10)
	rendered := term.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) != 10 {
		t.Errorf("Expected exactly 10 rows in Render(), got %d", len(lines))
	}
}

func TestTerminal_ScrollUp_ClearsBottom(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("A\r\nB\r\nC\r\nD")) // D should be on row 2 after scroll
	lines := strings.Split(term.Render(), "\n")
	row0 := strings.TrimSpace(stripANSI(lines[0]))
	if !strings.HasPrefix(row0, "B") {
		t.Errorf("Expected 'B' on row 0 after scroll, got: %q", row0)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func min8(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stripANSI removes ANSI escape sequences from s for plain-text comparison.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7e) {
				i++
			}
			i++ // skip final byte
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
