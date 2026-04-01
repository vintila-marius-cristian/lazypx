package tui

import (
	"strings"
	"testing"
	"time"

	"lazypx/api"
	"lazypx/sessions"
	"lazypx/state"
)

// prevent unused import errors
var _ = time.Now
var _ = api.Task{}
var _ = sessions.New
var _ = state.New

// ── GaugeBar / repeatRune ──────────────────────────────────────────────────

func TestGaugeBar_50Percent(t *testing.T) {
	bar := GaugeBar(10, 0.5)
	if bar == "" {
		t.Fatal("GaugeBar returned empty")
	}
}

func TestGaugeBar_ZeroPercent(t *testing.T) {
	bar := GaugeBar(10, 0)
	if bar == "" {
		t.Fatal("GaugeBar(0%) returned empty")
	}
}

func TestGaugeBar_100Percent(t *testing.T) {
	bar := GaugeBar(10, 1.0)
	if bar == "" {
		t.Fatal("GaugeBar(100%) returned empty")
	}
}

func TestGaugeBar_Negative(t *testing.T) {
	bar := GaugeBar(10, -0.5)
	if bar == "" {
		t.Fatal("GaugeBar(negative) returned empty")
	}
}

func TestGaugeBar_Over100(t *testing.T) {
	bar := GaugeBar(10, 1.5)
	if bar == "" {
		t.Fatal("GaugeBar(>100%) returned empty")
	}
}

func TestRepeatRune_Zero(t *testing.T) {
	if repeatRune('x', 0) != "" {
		t.Error("repeatRune(x, 0) should be empty")
	}
}

func TestRepeatRune_Negative(t *testing.T) {
	if repeatRune('x', -1) != "" {
		t.Error("repeatRune(x, -1) should be empty")
	}
}

func TestRepeatRune_5(t *testing.T) {
	if repeatRune('a', 5) != "aaaaa" {
		t.Error("repeatRune(a, 5) should be aaaaa")
	}
}

// ── Utility functions ──────────────────────────────────────────────────────

func TestSafeDiv_Extra(t *testing.T) {
	if safeDiv(10, 0) != 0 {
		t.Error("safeDiv(10, 0) should be 0")
	}
	if safeDiv(10, 2) != 5.0 {
		t.Error("safeDiv(10, 2) should be 5.0")
	}
}

func TestFormatBytes_Extra(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{-1, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, c := range cases {
		got := formatBytes(c.in)
		if got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatUptime_Extra(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0h 0m"},
		{60, "0h 1m"},
		{3600, "1h 0m"},
		{90061, "1d 1h 1m"},
	}
	for _, c := range cases {
		got := formatUptime(c.in)
		if got != c.want {
			t.Errorf("formatUptime(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatAge_Extra(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{2 * time.Hour, "2h"},
	}
	for _, c := range cases {
		got := formatAge(c.d)
		if got != c.want {
			t.Errorf("formatAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestFormatElapsed_Extra(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m"},
		{2*time.Hour + 30*time.Minute, "2h30m"},
	}
	for _, c := range cases {
		got := formatElapsed(c.d)
		if got != c.want {
			t.Errorf("formatElapsed(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTruncate_Extra(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncate("hello world", 5) != "hell…" {
		t.Errorf("truncate(\"hello world\", 5) = %q", truncate("hello world", 5))
	}
	if truncate("hello", 0) != "" {
		t.Error("truncate to 0 should be empty")
	}
}

// ── Terminal escape sequences ──────────────────────────────────────────────

func TestTerminal_Feed_CursorHome(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[Hxyz"))
	rendered := term.Render()
	if !strings.Contains(rendered, "xyz") {
		t.Errorf("Render should contain 'xyz' after cursor home, got %q", rendered)
	}
}

func TestTerminal_Feed_EraseScreen(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[2J"))
}

func TestTerminal_Feed_EraseLine(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("hello\x1b[2Kworld"))
}

func TestTerminal_Feed_EraseLineRight(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("hello\x1b[K"))
}

func TestTerminal_Feed_EraseLineLeft(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("hello\x1b[1K"))
}

func TestTerminal_Feed_ColorRed(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[31mred\x1b[0m"))
	rendered := term.Render()
	if !strings.Contains(rendered, "red") {
		t.Errorf("Render should contain 'red', got %q", rendered)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Error("Render should contain ANSI escape for color")
	}
}

func TestTerminal_Feed_CursorMovement(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[2;3H@"))
	rendered := term.Render()
	if !strings.Contains(rendered, "@") {
		t.Errorf("Render should contain '@', got %q", rendered)
	}
}

func TestTerminal_Feed_CursorUp(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("ab\ncd\x1b[2A@"))
}

func TestTerminal_Feed_CursorDown(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x1b[3B@"))
}

func TestTerminal_Feed_CursorRight(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x1b[5C@"))
}

func TestTerminal_Feed_CursorLeft(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abcdef\x1b[3D@"))
}

func TestTerminal_Feed_CR(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abcdef\rXY"))
}

func TestTerminal_Feed_Backspace(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("ab\x08c"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ac") {
		t.Errorf("Backspace should overwrite, got %q", rendered)
	}
}

func TestTerminal_Feed_Tab(t *testing.T) {
	term := NewTerminal(40, 5)
	term.Feed([]byte("a\tb"))
	rendered := term.Render()
	if !strings.Contains(rendered, "a") || !strings.Contains(rendered, "b") {
		t.Errorf("Tab should work, got %q", rendered)
	}
}

func TestTerminal_Feed_SaveRestoreCursor(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b7\x1b[Hxxx\x1b8d"))
	rendered := term.Render()
	if !strings.Contains(rendered, "d") {
		t.Errorf("Restore cursor should work, got %q", rendered)
	}
}

func TestTerminal_Feed_Reset(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[31mred\x1b[0m"))
	term.Feed([]byte("\x1bc"))
	rendered := term.Render()
	if strings.Contains(rendered, "red") {
		t.Error("After reset, content should be cleared")
	}
}

func TestTerminal_Feed_ScrollRegion(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[2;4r"))
	term.Feed([]byte("line\nline\nline\nline\nline"))
}

func TestTerminal_Feed_ScrollUp(t *testing.T) {
	term := NewTerminal(20, 5)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\n"))
	}
	term.Feed([]byte("\x1b[S"))
}

func TestTerminal_Feed_ScrollDown(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("hello\n"))
	term.Feed([]byte("\x1b[T"))
}

func TestTerminal_Feed_DeleteChar(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abcdef\x1b[3D\x1b[2P"))
}

func TestTerminal_Feed_DeleteLine(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("line1\nline2\nline3\x1b[1H\x1b[1M"))
}

func TestTerminal_Feed_InsertChar(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[1H\x1b[2@"))
}

func TestTerminal_Feed_VPA(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[3dX"))
}

func TestTerminal_Feed_CHA(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[5GX"))
}

func TestTerminal_Feed_CNLCPL(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[2EX"))
	term.Feed([]byte("def\x1b[1FY"))
}

func TestTerminal_Feed_ECH(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abcdef\x1b[2D\x1b[3X"))
}

func TestTerminal_Feed_EraseFromCursor(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\ndef\nghi\x1b[1;1H\x1b[0J"))
}

func TestTerminal_Feed_EraseToCursor(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\ndef\nghi\x1b[3;3H\x1b[1J"))
}

func TestTerminal_Feed_EraseAll(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[2J"))
}

func TestTerminal_SGR_AllAttributes(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[1;3;4;7;31;42mX\x1b[0m"))
	rendered := term.Render()
	if !strings.Contains(rendered, "X") {
		t.Errorf("SGR should render, got %q", rendered)
	}
}

func TestTerminal_SGR_BrightColors(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[90;100mX\x1b[0m"))
}

func TestTerminal_SGR_256Color(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[38;5;196mX\x1b[0m"))
	term.Feed([]byte("\x1b[48;5;21mY\x1b[0m"))
}

func TestTerminal_SGR_ResetBold(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[1mbold\x1b[22mnormal"))
}

func TestTerminal_SGR_ResetItalic(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[3mitalic\x1b[23mnormal"))
}

func TestTerminal_SGR_ResetUnderline(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[4munder\x1b[24mnormal"))
}

func TestTerminal_SGR_ResetReverse(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[7mrev\x1b[27mnormal"))
}

func TestTerminal_SGR_DefaultFg(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[31mred\x1b[39mdefault"))
}

func TestTerminal_SGR_DefaultBg(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[41mred\x1b[49mdefault"))
}

func TestTerminal_SGR_Empty(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b[m"))
}

func TestTerminal_ScrollViewUp_Extra(t *testing.T) {
	term := NewTerminal(20, 3)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\n"))
	}
	term.ScrollViewUp(5)
	if !term.IsScrolled() {
		t.Error("Should be scrolled")
	}
}

func TestTerminal_ScrollViewDown_Extra(t *testing.T) {
	term := NewTerminal(20, 3)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\n"))
	}
	term.ScrollViewUp(5)
	term.ScrollViewDown(3)
}

func TestTerminal_ScrollViewUp_Clamp(t *testing.T) {
	term := NewTerminal(20, 3)
	term.ScrollViewUp(100)
}

func TestTerminal_Render_Scrolled(t *testing.T) {
	term := NewTerminal(20, 3)
	for i := 0; i < 10; i++ {
		term.Feed([]byte("line\n"))
	}
	term.ScrollViewUp(2)
	rendered := term.Render()
	if rendered == "" {
		t.Error("Scrolled render should not be empty")
	}
}

func TestTerminal_Feed_BEL(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x07b"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ab") {
		t.Errorf("BEL should be ignored, got %q", rendered)
	}
}

func TestTerminal_Feed_SO_SI(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x0e\x0fb"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ab") {
		t.Errorf("SO/SI should be ignored, got %q", rendered)
	}
}

func TestTerminal_Feed_OSC(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x1b]0;title\x07b"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ab") {
		t.Errorf("OSC should be consumed, got %q", rendered)
	}
}

func TestTerminal_Feed_Charset(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x1b(Bb"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ab") {
		t.Errorf("Charset designation should be consumed, got %q", rendered)
	}
}

func TestTerminal_Feed_ReverseIndex(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1bM"))
}

func TestTerminal_Feed_NEL(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1bEdef"))
}

func TestTerminal_Feed_IND(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1bDdef"))
}

func TestTerminal_Feed_ApplicationKeypad(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("\x1b=\x1b>"))
}

func TestTerminal_Feed_DCS(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x1bPstuff\x07b"))
	rendered := term.Render()
	if !strings.Contains(rendered, "ab") {
		t.Errorf("DCS should be consumed, got %q", rendered)
	}
}

func TestTerminal_Feed_CSI_8bit(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x9b@b"))
}

func TestTerminal_Feed_VT(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x0bb"))
}

func TestTerminal_Feed_FF(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("a\x0cb"))
}

func TestTerminal_Feed_DECSET_AltScreen(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[?1049h"))
	term.Feed([]byte("xyz\x1b[?1049l"))
}

func TestTerminal_Feed_DECSET_1047(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[?1047h"))
}

func TestTerminal_Feed_SaveRestoreCursor_CSI(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("abc\x1b[s\x1b[Hxxx\x1b[u"))
}

func TestTerminal_Feed_WrapLine(t *testing.T) {
	term := NewTerminal(5, 3)
	term.Feed([]byte("abcdefghijk"))
	rendered := term.Render()
	if !strings.Contains(rendered, "abcde") {
		t.Errorf("Wrap should work, got %q", rendered)
	}
}

func TestTerminal_Render_WithAttrs(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[1;31mHello\x1b[0m"))
	rendered := term.Render()
	if !strings.Contains(rendered, "Hello") {
		t.Errorf("Colored text should render, got %q", rendered)
	}
}

func TestTerminal_Render_BrightFG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[91mX\x1b[0m"))
}

func TestTerminal_Render_BrightBG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[101mX\x1b[0m"))
}

func TestTerminal_Render_256FG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[38;5;123mX\x1b[0m"))
}

func TestTerminal_Render_256BG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[48;5;123mX\x1b[0m"))
}

func TestTerminal_Render_RGB_FG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[38;2;255;0;0mX\x1b[0m"))
}

func TestTerminal_Render_RGB_BG(t *testing.T) {
	term := NewTerminal(10, 3)
	term.Feed([]byte("\x1b[48;2;0;255;0mX\x1b[0m"))
}

func TestTerminal_Feed_AutoWrap(t *testing.T) {
	term := NewTerminal(5, 3)
	term.Feed([]byte("123456789"))
}

func TestTerminal_CursorBounds(t *testing.T) {
	term := NewTerminal(10, 5)
	term.Feed([]byte("\x1b[100;100H@"))
}

func TestTerminal_Feed_UTF8(t *testing.T) {
	term := NewTerminal(20, 5)
	term.Feed([]byte("héllo wörld"))
	rendered := term.Render()
	if !strings.Contains(rendered, "héllo") {
		t.Errorf("UTF-8 should work, got %q", rendered)
	}
}

// ── TasksModel ─────────────────────────────────────────────────────────────

func TestTasksModel_Sync_Extra(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	st2 := state.New("test", false)
	m.Sync(st2)
}

func TestTasksModel_View_WithTasks(t *testing.T) {
	st := state.New("test", false)
	st.LocalEvents = []state.LocalEvent{{Label: "test", Level: "info", At: time.Now()}}
	st.ActiveTasks = []state.ActiveTask{{Label: "backup", Done: true, Success: true}}
	st.Snapshot.Tasks = []api.Task{{Type: "vzdump", Status: "stopped", Node: "node1", User: "root@pam"}}
	m := NewTasksModel(st)
	v := m.View(60, 15, false)
	if !strings.Contains(v, "backup") {
		t.Errorf("View should contain task, got %q", v)
	}
}

func TestTasksModel_View_WithFailedTask(t *testing.T) {
	st := state.New("test", false)
	st.ActiveTasks = []state.ActiveTask{{Label: "restore", Done: true, Success: false}}
	m := NewTasksModel(st)
	v := m.View(60, 10, true)
	if !strings.Contains(v, "restore") {
		t.Errorf("View should contain failed task, got %q", v)
	}
}

func TestTasksModel_View_RunningTask(t *testing.T) {
	st := state.New("test", false)
	st.ActiveTasks = []state.ActiveTask{{Label: "migrate", Done: false}}
	m := NewTasksModel(st)
	v := m.View(60, 10, true)
	if !strings.Contains(v, "migrate") {
		t.Errorf("View should contain running task, got %q", v)
	}
}

func TestTasksModel_View_WarnEvent(t *testing.T) {
	st := state.New("test", false)
	st.LocalEvents = []state.LocalEvent{{Label: "warn", Level: "warn", At: time.Now()}}
	m := NewTasksModel(st)
	v := m.View(60, 10, true)
	if !strings.Contains(v, "warn") {
		t.Errorf("View should contain warn event, got %q", v)
	}
}

func TestTasksModel_View_ErrorEvent(t *testing.T) {
	st := state.New("test", false)
	st.LocalEvents = []state.LocalEvent{{Label: "err", Level: "error", At: time.Now()}}
	m := NewTasksModel(st)
	v := m.View(60, 10, true)
	if !strings.Contains(v, "err") {
		t.Errorf("View should contain error event, got %q", v)
	}
}

func TestTasksModel_View_WithLogLines(t *testing.T) {
	st := state.New("test", false)
	st.ActiveTasks = []state.ActiveTask{{Label: "backup", Done: false, Logs: []string{"starting...", "50% done"}}}
	m := NewTasksModel(st)
	v := m.View(60, 10, true)
	if !strings.Contains(v, "50% done") {
		t.Errorf("View should show last log line, got %q", v)
	}
}

func TestTasksModel_View_TooSmall_Extra(t *testing.T) {
	st := state.New("test", false)
	m := NewTasksModel(st)
	if m.View(1, 1, true) != "" {
		t.Error("View with too small size should return empty")
	}
}

func TestRenderClusterTaskRow_Extra(t *testing.T) {
	task := api.Task{Type: "vzdump", Status: "stopped", Node: "node1", User: "root@pam", ID: "100"}
	row := renderClusterTaskRow(task, 80)
	if !strings.Contains(row, "vzdump") {
		t.Errorf("Cluster task row should contain type, got %q", row)
	}
}

func TestRenderClusterTaskRow_Running_Extra(t *testing.T) {
	task := api.Task{Type: "qmstart", Status: "running", Node: "node1", User: "root@pam"}
	row := renderClusterTaskRow(task, 80)
	if !strings.Contains(row, "qmstart") {
		t.Errorf("Running task row should contain type, got %q", row)
	}
}

func TestRenderActiveTaskRow_Extra(t *testing.T) {
	task := state.ActiveTask{Label: "backup", Done: true, Success: true}
	row := renderActiveTaskRow(task, 80)
	if !strings.Contains(row, "backup") {
		t.Errorf("Active task row should contain label, got %q", row)
	}
}

func TestRenderActiveTaskRow_Failed_Extra(t *testing.T) {
	task := state.ActiveTask{Label: "restore", Done: true, Success: false}
	row := renderActiveTaskRow(task, 80)
	if !strings.Contains(row, "restore") {
		t.Errorf("Failed task row should contain label, got %q", row)
	}
}

func TestRenderLocalEventRow_Extra(t *testing.T) {
	ev := state.LocalEvent{Label: "shell opened", Level: "info", At: time.Now()}
	row := renderLocalEventRow(ev, 80)
	if !strings.Contains(row, "shell opened") {
		t.Errorf("Event row should contain label, got %q", row)
	}
}

func TestRenderLocalEventRow_Warn_Extra(t *testing.T) {
	ev := state.LocalEvent{Label: "warning", Level: "warn", At: time.Now()}
	row := renderLocalEventRow(ev, 80)
	if !strings.Contains(row, "warning") {
		t.Errorf("Warn event row should contain label, got %q", row)
	}
}

func TestRenderLocalEventRow_Error_Extra(t *testing.T) {
	ev := state.LocalEvent{Label: "failed", Level: "error", At: time.Now()}
	row := renderLocalEventRow(ev, 80)
	if !strings.Contains(row, "failed") {
		t.Errorf("Error event row should contain label, got %q", row)
	}
}

// ── SessionsModel ──────────────────────────────────────────────────────────

func TestSessionsModel_New_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	if m.cursor != 0 {
		t.Error("Initial cursor should be 0")
	}
}

func TestSessionsModel_Refresh_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.Refresh()
	if len(m.sessions) != 0 {
		t.Error("No sessions expected")
	}
}

func TestSessionsModel_MoveUp_Empty_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.MoveUp()
	if m.cursor != 0 {
		t.Error("Cursor should stay at 0 for empty list")
	}
}

func TestSessionsModel_MoveDown_Empty_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.MoveDown()
	if m.cursor != 0 {
		t.Error("Cursor should stay at 0 for empty list")
	}
}

func TestSessionsModel_Selected_Nil_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.Refresh()
	if m.Selected() != nil {
		t.Error("Selected should be nil for empty sessions")
	}
}

func TestSessionsModel_View_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	m := NewSessionsModel(st, mgr)
	m.Refresh()
	v := m.View(80, 20)
	if !strings.Contains(v, "Shell Sessions") {
		t.Errorf("View should contain title, got %q", v)
	}
	if !strings.Contains(v, "No active sessions") {
		t.Errorf("View should contain empty message, got %q", v)
	}
}

func TestSessionsModel_View_WithSessions_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	key := mgr.SessionKey(100)
	mgr.OpenSession(key, "sleep", []string{"60"})
	defer mgr.CloseAll()

	m := NewSessionsModel(st, mgr)
	m.Refresh()
	v := m.View(80, 20)
	if !strings.Contains(v, "100") {
		t.Errorf("View should contain VMID, got %q", v)
	}
}

func TestSessionsModel_MoveDown_WithSessions_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	key1 := mgr.SessionKey(100)
	key2 := mgr.SessionKey(200)
	mgr.OpenSession(key1, "sleep", []string{"60"})
	mgr.OpenSession(key2, "sleep", []string{"60"})
	defer mgr.CloseAll()

	m := NewSessionsModel(st, mgr)
	m.Refresh()
	m.MoveDown()
	if m.cursor != 1 {
		t.Errorf("Cursor should be 1, got %d", m.cursor)
	}
	m.MoveDown()
	if m.cursor != 0 {
		t.Errorf("Cursor should wrap to 0, got %d", m.cursor)
	}
}

func TestSessionsModel_MoveUp_WithSessions_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	key1 := mgr.SessionKey(100)
	key2 := mgr.SessionKey(200)
	mgr.OpenSession(key1, "sleep", []string{"60"})
	mgr.OpenSession(key2, "sleep", []string{"60"})
	defer mgr.CloseAll()

	m := NewSessionsModel(st, mgr)
	m.Refresh()
	m.MoveDown()
	m.MoveUp()
	if m.cursor != 0 {
		t.Errorf("Cursor should be 0, got %d", m.cursor)
	}
	m.MoveUp()
	if m.cursor != 1 {
		t.Errorf("Cursor should wrap to 1, got %d", m.cursor)
	}
}

func TestSessionsModel_Selected_WithSessions_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	key := mgr.SessionKey(100)
	mgr.OpenSession(key, "sleep", []string{"60"})
	defer mgr.CloseAll()

	m := NewSessionsModel(st, mgr)
	m.Refresh()
	s := m.Selected()
	if s == nil {
		t.Fatal("Selected should not be nil")
	}
	if s.VMID != 100 {
		t.Errorf("Selected VMID should be 100, got %d", s.VMID)
	}
}

func TestSessionsModel_Refresh_ClampCursor_Extra(t *testing.T) {
	st := state.New("test", false)
	mgr := sessions.New("test")
	key1 := mgr.SessionKey(100)
	key2 := mgr.SessionKey(200)
	mgr.OpenSession(key1, "sleep", []string{"60"})
	mgr.OpenSession(key2, "sleep", []string{"60"})
	defer mgr.CloseAll()

	m := NewSessionsModel(st, mgr)
	m.Refresh()
	m.cursor = 5
	m.Refresh()
	if m.cursor != 1 {
		t.Errorf("Cursor should be clamped to 1, got %d", m.cursor)
	}
}

// ── Detail helpers ─────────────────────────────────────────────────────────

func TestExtractDisks_Extra(t *testing.T) {
	cfg := &api.VMConfig{
		Scsi0:   "SlowSmsgSSD:vm-221-disk-0,replicate=0,size=384G",
		Scsi1:   "local-lvm:vm-100-disk-1,size=32G",
		Virtio0: "",
	}
	disks := extractDisks(cfg)
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if !strings.Contains(disks[0].Val, "384G") {
		t.Errorf("disk 0 should contain size, got %q", disks[0].Val)
	}
}

func TestExtractDisks_Cdrom_Extra(t *testing.T) {
	cfg := &api.VMConfig{Scsi0: "local:iso/debian.iso,media=cdrom"}
	disks := extractDisks(cfg)
	if len(disks) != 0 {
		t.Errorf("cdrom should be excluded, got %d disks", len(disks))
	}
}

func TestExtractDisks_Empty_Extra(t *testing.T) {
	cfg := &api.VMConfig{}
	disks := extractDisks(cfg)
	if len(disks) != 0 {
		t.Errorf("empty config should have 0 disks, got %d", len(disks))
	}
}

func TestExtractNets_Extra(t *testing.T) {
	cfg := &api.VMConfig{
		Net0: "virtio=BC:24:11:3B:92:A1,bridge=vmbr0,tag=4",
		Net1: "",
	}
	nets := extractNets(cfg)
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	if !strings.Contains(nets[0].Val, "vmbr0") {
		t.Errorf("net should contain bridge, got %q", nets[0].Val)
	}
}

func TestExtractNets_NoTag_Extra(t *testing.T) {
	cfg := &api.VMConfig{Net0: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"}
	nets := extractNets(cfg)
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
	if strings.Contains(nets[0].Val, "vlan") {
		t.Error("net without tag should not contain vlan")
	}
}

func TestExtractNets_MacOnly_Extra(t *testing.T) {
	cfg := &api.VMConfig{Net0: "virtio=AA:BB:CC:DD:EE:FF"}
	nets := extractNets(cfg)
	if len(nets) != 1 {
		t.Fatalf("expected 1 net, got %d", len(nets))
	}
}

func TestExtractNets_Empty_Extra(t *testing.T) {
	cfg := &api.VMConfig{}
	nets := extractNets(cfg)
	if len(nets) != 0 {
		t.Errorf("empty config should have 0 nets, got %d", len(nets))
	}
}

// ── Layout ─────────────────────────────────────────────────────────────────

func TestGaugeWidth_Extra(t *testing.T) {
	for _, in := range []int{10, 80, 200} {
		got := GaugeWidth(in)
		if got < 14 || got > 32 {
			t.Errorf("GaugeWidth(%d) = %d, out of bounds [14,32]", in, got)
		}
	}
}

func TestClamp_Extra(t *testing.T) {
	if clamp(5, 0, 10) != 5 {
		t.Error("clamp(5,0,10) should be 5")
	}
	if clamp(-1, 0, 10) != 0 {
		t.Error("clamp(-1,0,10) should be 0")
	}
	if clamp(15, 0, 10) != 10 {
		t.Error("clamp(15,0,10) should be 10")
	}
}

func TestClipToHeight_Extra(t *testing.T) {
	if clipToHeight("a\nb\nc", 2) != "a\nb" {
		t.Errorf("clipToHeight should trim, got %q", clipToHeight("a\nb\nc", 2))
	}
	if clipToHeight("a", 5) != "a" {
		t.Error("clipToHeight should not add lines")
	}
}

// ── Compile check ──────────────────────────────────────────────────────────
// Ensure unused imports are used
var _ = time.Now
var _ = api.Task{}
var _ = sessions.New
var _ = state.New
