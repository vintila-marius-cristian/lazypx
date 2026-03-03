package tui

// terminal.go — minimal VT100/VT220 terminal emulator for embedded SSH sessions.
// Handles cursor movement, SGR colors, erase, scroll regions, and scrollback.
// Designed to render inside a Lip Gloss pane with ANSI passthrough.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ── Cell types ────────────────────────────────────────────────────────────────

const attrDefault = -1 // sentinel: use terminal default color

// termAttr holds SGR visual attributes for a single cell.
type termAttr struct {
	fg, bg    int  // ANSI color index, or attrDefault
	bold      bool
	underline bool
	italic    bool
	reverse   bool
}

func defaultTermAttr() termAttr { return termAttr{fg: attrDefault, bg: attrDefault} }

// termCell is a single character cell in the terminal grid.
type termCell struct {
	ch   rune
	attr termAttr
}

// ── Parser states ─────────────────────────────────────────────────────────────

const (
	pStateNormal = iota
	pStateESC
	pStateCSI
	pStateOSC
	pStateCharset
)

// ── Terminal ──────────────────────────────────────────────────────────────────

// Terminal is a minimal VT100/VT220 terminal emulator.
// All public methods are NOT thread-safe; callers must serialise access.
type Terminal struct {
	cols, rows int
	grid       [][]termCell // [rows][cols]

	cx, cy           int // cursor position (0-based)
	savedCX, savedCY int // ESC 7/8 saved cursor

	curAttr termAttr

	// Scroll region (0-based row indices, inclusive)
	scrollTop, scrollBot int

	// Scrollback buffer: lines that have scrolled off the top.
	scrollback    [][]termCell
	maxScrollback int

	// viewOffset: 0 = show live screen; >0 = scrolled that many rows into history.
	viewOffset int

	// ANSI parser
	parseState int
	csiParams  string
	csiInter   string

	// Track alternate screen mode (simplified: just clear on switch)
	altScreen bool
}

// NewTerminal creates a Terminal with the given dimensions.
func NewTerminal(cols, rows int) *Terminal {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	t := &Terminal{
		cols:          cols,
		rows:          rows,
		curAttr:       defaultTermAttr(),
		maxScrollback: 2000,
	}
	t.scrollBot = rows - 1
	t.grid = t.makeGrid(cols, rows)
	return t
}

// ── Grid helpers ──────────────────────────────────────────────────────────────

func (t *Terminal) makeGrid(cols, rows int) [][]termCell {
	g := make([][]termCell, rows)
	for i := range g {
		g[i] = t.makeRow(cols)
	}
	return g
}

func (t *Terminal) makeRow(cols int) []termCell {
	row := make([]termCell, cols)
	for i := range row {
		row[i] = termCell{ch: ' ', attr: defaultTermAttr()}
	}
	return row
}

// ── Public API ────────────────────────────────────────────────────────────────

// Resize changes the terminal dimensions, preserving content where it fits.
func (t *Terminal) Resize(cols, rows int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cols == t.cols && rows == t.rows {
		return
	}
	newGrid := make([][]termCell, rows)
	for i := range newGrid {
		newGrid[i] = t.makeRow(cols)
		if i < len(t.grid) {
			n := cols
			if len(t.grid[i]) < n {
				n = len(t.grid[i])
			}
			copy(newGrid[i], t.grid[i][:n])
		}
	}
	t.grid = newGrid
	t.cols = cols
	t.rows = rows
	t.scrollTop = 0
	t.scrollBot = rows - 1
	if t.cy >= rows {
		t.cy = rows - 1
	}
	if t.cx >= cols {
		t.cx = cols - 1
	}
}

// Feed processes raw bytes from the PTY.
func (t *Terminal) Feed(data []byte) {
	i := 0
	for i < len(data) {
		b := data[i]
		// Handle UTF-8 multi-byte in normal state.
		if t.parseState == pStateNormal && b >= 0x80 {
			r, size := utf8.DecodeRune(data[i:])
			if r != utf8.RuneError || size != 1 {
				if r != utf8.RuneError {
					t.putChar(r)
				}
				i += size
				continue
			}
		}
		t.processByte(b)
		i++
	}
}

// ScrollViewUp scrolls the view into scrollback history by n rows.
func (t *Terminal) ScrollViewUp(n int) {
	t.viewOffset += n
	max := len(t.scrollback)
	if t.viewOffset > max {
		t.viewOffset = max
	}
}

// ScrollViewDown scrolls the view toward the live screen by n rows.
func (t *Terminal) ScrollViewDown(n int) {
	t.viewOffset -= n
	if t.viewOffset < 0 {
		t.viewOffset = 0
	}
}

// ScrollViewReset returns to the live view.
func (t *Terminal) ScrollViewReset() { t.viewOffset = 0 }

// IsScrolled reports whether the user has scrolled into history.
func (t *Terminal) IsScrolled() bool { return t.viewOffset > 0 }

// ── Feed internals ────────────────────────────────────────────────────────────

func (t *Terminal) processByte(b byte) {
	switch t.parseState {
	case pStateNormal:
		t.processNormal(b)
	case pStateESC:
		t.processESC(b)
	case pStateCSI:
		t.processCSI(b)
	case pStateOSC:
		// Consume OSC string until BEL (0x07) or ST (ESC \).
		if b == 0x07 || b == 0x9c {
			t.parseState = pStateNormal
		}
		// ESC inside OSC begins the string terminator ESC \; just reset.
		if b == 0x1b {
			t.parseState = pStateNormal
		}
	case pStateCharset:
		// Consume the charset designator byte and return to normal.
		t.parseState = pStateNormal
	}
}

func (t *Terminal) processNormal(b byte) {
	switch b {
	case 0x07: // BEL — ignore
	case 0x08: // BS
		if t.cx > 0 {
			t.cx--
		}
	case 0x09: // HT — advance to next tab stop
		next := (t.cx/8 + 1) * 8
		if next >= t.cols {
			next = t.cols - 1
		}
		t.cx = next
	case 0x0a, 0x0b, 0x0c: // LF / VT / FF
		t.newline()
	case 0x0d: // CR
		t.cx = 0
	case 0x0e, 0x0f: // SO / SI — charset shifts, ignore
	case 0x1b: // ESC
		t.parseState = pStateESC
		t.csiParams = ""
		t.csiInter = ""
	case 0x9b: // CSI (8-bit shortcut)
		t.parseState = pStateCSI
		t.csiParams = ""
		t.csiInter = ""
	case 0x9d: // OSC (8-bit shortcut)
		t.parseState = pStateOSC
	default:
		if b >= 0x20 {
			t.putChar(rune(b))
		}
	}
}

func (t *Terminal) putChar(ch rune) {
	if t.cy < 0 || t.cy >= t.rows {
		return
	}
	if t.cx >= t.cols {
		// Auto-wrap to next line.
		t.cx = 0
		t.newline()
	}
	if t.cx < t.cols && t.cy < t.rows {
		t.grid[t.cy][t.cx] = termCell{ch: ch, attr: t.curAttr}
		t.cx++
	}
}

func (t *Terminal) newline() {
	t.cy++
	if t.cy > t.scrollBot {
		t.scrollUp(1)
		t.cy = t.scrollBot
	}
}

func (t *Terminal) scrollUp(n int) {
	for i := 0; i < n; i++ {
		// Save departing top line to scrollback.
		saved := make([]termCell, t.cols)
		copy(saved, t.grid[t.scrollTop])
		t.scrollback = append(t.scrollback, saved)
		if len(t.scrollback) > t.maxScrollback {
			t.scrollback = t.scrollback[1:]
		}
		// Shift rows up within the scroll region.
		for row := t.scrollTop; row < t.scrollBot; row++ {
			copy(t.grid[row], t.grid[row+1])
		}
		// Clear bottom row.
		t.grid[t.scrollBot] = t.makeRow(t.cols)
	}
}

func (t *Terminal) scrollDown(n int) {
	for i := 0; i < n; i++ {
		for row := t.scrollBot; row > t.scrollTop; row-- {
			copy(t.grid[row], t.grid[row-1])
		}
		t.grid[t.scrollTop] = t.makeRow(t.cols)
	}
}

// ── ESC / CSI dispatch ────────────────────────────────────────────────────────

func (t *Terminal) processESC(b byte) {
	t.parseState = pStateNormal
	switch b {
	case '[':
		t.parseState = pStateCSI
		t.csiParams = ""
		t.csiInter = ""
	case ']':
		t.parseState = pStateOSC
	case '7': // DECSC — save cursor
		t.savedCX, t.savedCY = t.cx, t.cy
	case '8': // DECRC — restore cursor
		t.cx, t.cy = t.savedCX, t.savedCY
	case 'c': // RIS — reset
		t.curAttr = defaultTermAttr()
		t.cx, t.cy = 0, 0
		t.scrollTop, t.scrollBot = 0, t.rows-1
		t.grid = t.makeGrid(t.cols, t.rows)
	case 'D': // IND — index (newline without CR)
		t.newline()
	case 'M': // RI — reverse index
		if t.cy == t.scrollTop {
			t.scrollDown(1)
		} else if t.cy > 0 {
			t.cy--
		}
	case 'E': // NEL — next line
		t.cx = 0
		t.newline()
	case '(', ')', '*', '+': // charset designation
		t.parseState = pStateCharset
	case '=', '>': // application/normal keypad mode — ignore
	case 'P': // DCS — reuse OSC termination logic
		t.parseState = pStateOSC
	}
}

func (t *Terminal) processCSI(b byte) {
	if b >= 0x40 && b <= 0x7e {
		// Final byte — dispatch and reset.
		t.dispatchCSI(b)
		t.parseState = pStateNormal
		t.csiParams = ""
		t.csiInter = ""
	} else if b >= 0x20 && b <= 0x2f {
		t.csiInter += string(rune(b))
	} else {
		// Parameter bytes: digits, semicolons, ?, >, etc.
		t.csiParams += string(rune(b))
	}
}

func (t *Terminal) parseCSIParams() []int {
	raw := t.csiParams
	if len(raw) > 0 {
		first := raw[0]
		if first == '?' || first == '>' || first == '!' || first == '<' {
			raw = raw[1:]
		}
	}
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	out := make([]int, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			n, _ := strconv.Atoi(p)
			out[i] = n
		}
	}
	return out
}

func csiParam(params []int, i, def int) int {
	if i < len(params) && params[i] != 0 {
		return params[i]
	}
	return def
}

func (t *Terminal) dispatchCSI(final byte) {
	params := t.parseCSIParams()
	isPrivate := len(t.csiParams) > 0 &&
		(t.csiParams[0] == '?' || t.csiParams[0] == '>' || t.csiParams[0] == '!')

	switch final {
	case 'A': // CUU — cursor up
		n := csiParam(params, 0, 1)
		t.cy -= n
		if t.cy < t.scrollTop {
			t.cy = t.scrollTop
		}
	case 'B': // CUD — cursor down
		n := csiParam(params, 0, 1)
		t.cy += n
		if t.cy > t.scrollBot {
			t.cy = t.scrollBot
		}
	case 'C': // CUF — cursor right
		n := csiParam(params, 0, 1)
		t.cx += n
		if t.cx >= t.cols {
			t.cx = t.cols - 1
		}
	case 'D': // CUB — cursor left
		n := csiParam(params, 0, 1)
		t.cx -= n
		if t.cx < 0 {
			t.cx = 0
		}
	case 'E': // CNL — cursor next line
		n := csiParam(params, 0, 1)
		t.cx = 0
		t.cy += n
		if t.cy > t.scrollBot {
			t.cy = t.scrollBot
		}
	case 'F': // CPL — cursor preceding line
		n := csiParam(params, 0, 1)
		t.cx = 0
		t.cy -= n
		if t.cy < t.scrollTop {
			t.cy = t.scrollTop
		}
	case 'G': // CHA — cursor horizontal absolute
		col := csiParam(params, 0, 1) - 1
		if col < 0 {
			col = 0
		}
		if col >= t.cols {
			col = t.cols - 1
		}
		t.cx = col
	case 'H', 'f': // CUP / HVP — cursor position
		row := csiParam(params, 0, 1) - 1
		col := csiParam(params, 1, 1) - 1
		if row < 0 {
			row = 0
		}
		if col < 0 {
			col = 0
		}
		if row >= t.rows {
			row = t.rows - 1
		}
		if col >= t.cols {
			col = t.cols - 1
		}
		t.cy, t.cx = row, col
	case 'J': // ED — erase in display
		switch csiParam(params, 0, 0) {
		case 0:
			t.eraseFromCursor()
		case 1:
			t.eraseToCursor()
		case 2, 3:
			t.eraseAll()
		}
	case 'K': // EL — erase in line
		switch csiParam(params, 0, 0) {
		case 0:
			t.eraseLineRight()
		case 1:
			t.eraseLineLeft()
		case 2:
			t.eraseLine(t.cy)
		}
	case 'L': // IL — insert lines
		for i := 0; i < csiParam(params, 0, 1); i++ {
			t.scrollDown(1)
		}
	case 'M': // DL — delete lines
		t.scrollUp(csiParam(params, 0, 1))
	case 'P': // DCH — delete characters
		n := csiParam(params, 0, 1)
		if n > t.cols-t.cx {
			n = t.cols - t.cx
		}
		row := t.grid[t.cy]
		copy(row[t.cx:], row[t.cx+n:])
		for i := t.cols - n; i < t.cols; i++ {
			row[i] = termCell{ch: ' ', attr: defaultTermAttr()}
		}
	case 'S': // SU — scroll up
		t.scrollUp(csiParam(params, 0, 1))
	case 'T': // SD — scroll down
		t.scrollDown(csiParam(params, 0, 1))
	case 'X': // ECH — erase characters
		n := csiParam(params, 0, 1)
		for i := t.cx; i < t.cx+n && i < t.cols; i++ {
			t.grid[t.cy][i] = termCell{ch: ' ', attr: defaultTermAttr()}
		}
	case '@': // ICH — insert characters
		n := csiParam(params, 0, 1)
		if n > t.cols-t.cx {
			n = t.cols - t.cx
		}
		row := t.grid[t.cy]
		copy(row[t.cx+n:], row[t.cx:t.cols-n])
		for i := t.cx; i < t.cx+n; i++ {
			row[i] = termCell{ch: ' ', attr: defaultTermAttr()}
		}
	case 'd': // VPA — vertical position absolute
		row := csiParam(params, 0, 1) - 1
		if row < 0 {
			row = 0
		}
		if row >= t.rows {
			row = t.rows - 1
		}
		t.cy = row
	case 'h': // SM / DECSET
		if isPrivate {
			t.handleDECSET(params, true)
		}
	case 'l': // RM / DECRST
		if isPrivate {
			t.handleDECSET(params, false)
		}
	case 'm': // SGR
		t.processSGR(params)
	case 'n': // DSR — device status report, ignore
	case 'r': // DECSTBM — set scroll region
		top := csiParam(params, 0, 1) - 1
		bot := csiParam(params, 1, t.rows) - 1
		if top < 0 {
			top = 0
		}
		if bot >= t.rows {
			bot = t.rows - 1
		}
		if top < bot {
			t.scrollTop = top
			t.scrollBot = bot
		}
		t.cx, t.cy = 0, 0
	case 's': // SCP — save cursor
		t.savedCX, t.savedCY = t.cx, t.cy
	case 'u': // RCP — restore cursor
		t.cx, t.cy = t.savedCX, t.savedCY
	}
}

func (t *Terminal) handleDECSET(params []int, enable bool) {
	for _, p := range params {
		switch p {
		case 1049: // alternate screen buffer
			if enable {
				t.savedCX, t.savedCY = t.cx, t.cy
				t.altScreen = true
				t.eraseAll()
				t.cx, t.cy = 0, 0
			} else {
				t.altScreen = false
				t.cx, t.cy = t.savedCX, t.savedCY
				t.eraseAll()
			}
		case 1047: // use alternate screen
			if enable {
				t.eraseAll()
			}
		// 25 (cursor visibility), 1, 2004 (bracketed paste), etc. — ignore
		}
	}
}

// ── SGR ───────────────────────────────────────────────────────────────────────

func (t *Terminal) processSGR(params []int) {
	if len(params) == 0 {
		t.curAttr = defaultTermAttr()
		return
	}
	i := 0
	for i < len(params) {
		p := params[i]
		switch {
		case p == 0:
			t.curAttr = defaultTermAttr()
		case p == 1:
			t.curAttr.bold = true
		case p == 2:
			t.curAttr.bold = false // dim
		case p == 3:
			t.curAttr.italic = true
		case p == 4:
			t.curAttr.underline = true
		case p == 7:
			t.curAttr.reverse = true
		case p == 21, p == 22:
			t.curAttr.bold = false
		case p == 23:
			t.curAttr.italic = false
		case p == 24:
			t.curAttr.underline = false
		case p == 27:
			t.curAttr.reverse = false
		case p >= 30 && p <= 37:
			t.curAttr.fg = p - 30
		case p == 38:
			if i+1 < len(params) {
				switch params[i+1] {
				case 5:
					if i+2 < len(params) {
						t.curAttr.fg = params[i+2] + 256
						i += 2
					}
				case 2:
					i += 4 // skip R;G;B
				}
			}
		case p == 39:
			t.curAttr.fg = attrDefault
		case p >= 40 && p <= 47:
			t.curAttr.bg = p - 40
		case p == 48:
			if i+1 < len(params) {
				switch params[i+1] {
				case 5:
					if i+2 < len(params) {
						t.curAttr.bg = params[i+2] + 256
						i += 2
					}
				case 2:
					i += 4 // skip R;G;B
				}
			}
		case p == 49:
			t.curAttr.bg = attrDefault
		case p >= 90 && p <= 97: // bright foreground
			t.curAttr.fg = p - 90 + 8
		case p >= 100 && p <= 107: // bright background
			t.curAttr.bg = p - 100 + 8
		}
		i++
	}
}

// ── Erase helpers ─────────────────────────────────────────────────────────────

func (t *Terminal) eraseAll() {
	for i := range t.grid {
		t.grid[i] = t.makeRow(t.cols)
	}
}

func (t *Terminal) eraseLine(row int) {
	if row >= 0 && row < t.rows {
		t.grid[row] = t.makeRow(t.cols)
	}
}

func (t *Terminal) eraseLineRight() {
	if t.cy >= 0 && t.cy < t.rows {
		for col := t.cx; col < t.cols; col++ {
			t.grid[t.cy][col] = termCell{ch: ' ', attr: defaultTermAttr()}
		}
	}
}

func (t *Terminal) eraseLineLeft() {
	if t.cy >= 0 && t.cy < t.rows {
		for col := 0; col <= t.cx && col < t.cols; col++ {
			t.grid[t.cy][col] = termCell{ch: ' ', attr: defaultTermAttr()}
		}
	}
}

func (t *Terminal) eraseFromCursor() {
	t.eraseLineRight()
	for row := t.cy + 1; row < t.rows; row++ {
		t.grid[row] = t.makeRow(t.cols)
	}
}

func (t *Terminal) eraseToCursor() {
	t.eraseLineLeft()
	for row := 0; row < t.cy; row++ {
		t.grid[row] = t.makeRow(t.cols)
	}
}

// ── Render ────────────────────────────────────────────────────────────────────

// viewRows returns the rows to display, accounting for scroll offset.
// Returns t.grid when not scrolled, otherwise mixes scrollback + grid.
func (t *Terminal) viewRows() [][]termCell {
	if t.viewOffset == 0 {
		return t.grid
	}
	totalSB := len(t.scrollback)
	total := totalSB + t.rows
	start := total - t.rows - t.viewOffset

	result := make([][]termCell, t.rows)
	empty := t.makeRow(t.cols)
	for i := 0; i < t.rows; i++ {
		idx := start + i
		if idx < 0 || idx >= total {
			result[i] = empty
		} else if idx < totalSB {
			src := t.scrollback[idx]
			row := make([]termCell, t.cols)
			n := len(src)
			if n > t.cols {
				n = t.cols
			}
			copy(row, src[:n])
			result[i] = row
		} else {
			result[i] = t.grid[idx-totalSB]
		}
	}
	return result
}

// Render produces the terminal content as a string with embedded ANSI escape
// codes for colors and attributes. Rows are separated by '\n'.
// Lip Gloss preserves ANSI sequences in content, so colors render correctly.
func (t *Terminal) Render() string {
	var sb strings.Builder
	rows := t.viewRows()
	lastAttr := defaultTermAttr()
	attrActive := false

	for ri, row := range rows {
		for _, cell := range row {
			if cell.attr != lastAttr {
				if attrActive {
					sb.WriteString("\033[0m")
					attrActive = false
				}
				codes := buildSGRCodes(cell.attr)
				if codes != "" {
					sb.WriteString("\033[")
					sb.WriteString(codes)
					sb.WriteByte('m')
					attrActive = true
				}
				lastAttr = cell.attr
			}
			ch := cell.ch
			if ch == 0 {
				ch = ' '
			}
			sb.WriteRune(ch)
		}
		if attrActive {
			sb.WriteString("\033[0m")
			attrActive = false
			lastAttr = defaultTermAttr()
		}
		if ri < len(rows)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// buildSGRCodes produces the semicolon-separated SGR parameter string for an
// attribute. Returns "" for the default (no-op) attribute.
func buildSGRCodes(a termAttr) string {
	var parts []string
	if a.bold {
		parts = append(parts, "1")
	}
	if a.italic {
		parts = append(parts, "3")
	}
	if a.underline {
		parts = append(parts, "4")
	}
	if a.reverse {
		parts = append(parts, "7")
	}
	if a.fg != attrDefault {
		switch {
		case a.fg >= 0 && a.fg < 8:
			parts = append(parts, fmt.Sprintf("3%d", a.fg))
		case a.fg >= 8 && a.fg < 16:
			parts = append(parts, fmt.Sprintf("9%d", a.fg-8))
		case a.fg >= 256:
			parts = append(parts, fmt.Sprintf("38;5;%d", a.fg-256))
		}
	}
	if a.bg != attrDefault {
		switch {
		case a.bg >= 0 && a.bg < 8:
			parts = append(parts, fmt.Sprintf("4%d", a.bg))
		case a.bg >= 8 && a.bg < 16:
			parts = append(parts, fmt.Sprintf("10%d", a.bg-8))
		case a.bg >= 256:
			parts = append(parts, fmt.Sprintf("48;5;%d", a.bg-256))
		}
	}
	return strings.Join(parts, ";")
}
