package sessions

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// SessionInfo describes a known lazypx PTY session.
type SessionInfo struct {
	Key       string
	VMID      int
	Kind      string
	Status    string
	StartedAt time.Time
	Attached  bool
}

// session holds the active background process state.
type session struct {
	key       string
	vmid      int
	cmd       *exec.Cmd
	ptm       *os.File // pty master
	startedAt time.Time
	attached  bool
	exited    atomic.Bool // set by background goroutine after Wait()
	mu        sync.Mutex
}

// Manager controls purely Go-native PTY-backed SSH sessions.
type Manager struct {
	mu       sync.Mutex
	prefix   string
	sessions map[string]*session
}

// New creates a new Manager with the given profile-based prefix.
func New(profileName string) *Manager {
	return &Manager{
		prefix:   fmt.Sprintf("lazypx-%s", sanitize(profileName)),
		sessions: make(map[string]*session),
	}
}

// HasTmux is kept for compatibility but native PTY means we always have persistence!
func HasTmux() bool {
	return true
}

// SessionKey returns the session name for a given VMID.
func (m *Manager) SessionKey(vmid int) string {
	return fmt.Sprintf("%s-%d", m.prefix, vmid)
}

// OpenSession ensures a PTY session exists for the given key.
func (m *Manager) OpenSession(key string, cmdName string, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up if process died
	if s, exists := m.sessions[key]; exists {
		if !s.exited.Load() {
			// Process is still running -> return nil
			return nil
		}
		delete(m.sessions, key)
	}

	c := exec.Command(cmdName, args...)
	ptm, err := pty.Start(c)
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	var vmid int
	fmt.Sscanf(key, m.prefix+"-%d", &vmid)

	sess := &session{
		key:       key,
		vmid:      vmid,
		cmd:       c,
		ptm:       ptm,
		startedAt: time.Now(),
	}

	m.sessions[key] = sess

	// Monitor for exit in the background.
	// We do NOT hold sess.mu during Wait() to avoid lock ordering issues.
	// The atomic flag is set after Wait() returns so ListSessions/IsAlive
	// see a consistent "exited" state without touching cmd.ProcessState.
	go func(s *session) {
		s.cmd.Wait()
		s.exited.Store(true)
	}(sess)

	return nil
}

// CloseSession kills a session.
func (m *Manager) CloseSession(key string) error {
	m.mu.Lock()
	s, exists := m.sessions[key]
	if exists {
		delete(m.sessions, key)
	}
	m.mu.Unlock()

	if !exists {
		return nil
	}

	if s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}
	s.ptm.Close()
	return nil
}

// CloseAll kills all sessions. Called on app quit.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	sessions := m.sessions
	m.sessions = make(map[string]*session)
	m.mu.Unlock()

	for _, s := range sessions {
		if s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
		s.ptm.Close()
	}
}

// ListSessions returns all active sessions.
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.Lock()
	sessions := make([]*session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	var out []SessionInfo
	for _, s := range sessions {
		status := "running"
		if s.exited.Load() {
			status = "exited"
		}
		s.mu.Lock()
		attached := s.attached
		s.mu.Unlock()
		out = append(out, SessionInfo{
			Key:       s.key,
			VMID:      s.vmid,
			Status:    status,
			StartedAt: s.startedAt,
			Attached:  attached,
		})
	}
	return out
}

// PTYAttacher encapsulates a session attach operation for Bubbletea.
type PTYAttacher struct {
	mgr *Manager
	key string
}

func (p PTYAttacher) SetStdin(io.Reader)  {}
func (p PTYAttacher) SetStdout(io.Writer) {}
func (p PTYAttacher) SetStderr(io.Writer) {}

func (p PTYAttacher) Run() error {
	p.mgr.mu.Lock()
	s, exists := p.mgr.sessions[p.key]
	p.mgr.mu.Unlock()

	if !exists {
		return fmt.Errorf("session not found")
	}

	s.mu.Lock()
	if s.attached {
		s.mu.Unlock()
		return fmt.Errorf("session already attached")
	}
	s.attached = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.attached = false
		s.mu.Unlock()
	}()

	// Check if already dead
	if s.exited.Load() {
		return fmt.Errorf("session process has exited")
	}

	// 1. Put standard input into raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// 2. Handle window resize (SIGWINCH)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go func() {
		for range sigChan {
			pty.InheritSize(os.Stdin, s.ptm)
		}
	}()
	sigChan <- syscall.SIGWINCH // trigger initial resize
	defer signal.Stop(sigChan)

	// Print a small helper header
	fmt.Print("\r\n\033[1;36m=== Attached to " + p.key + " | Press Ctrl+q to detach ===\033[0m\r\n")

	// 3. Copy PTY -> Stdout in background
	go func() {
		io.Copy(os.Stdout, s.ptm)
	}()

	// 4. Copy Stdin -> PTY, intercepting Ctrl+Q (0x11) for detach
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			// Scan for Ctrl+q
			for i := 0; i < n; i++ {
				if buf[i] == 17 { // 17 is Ctrl+Q
					// Detach safely
					fmt.Print("\r\n\033[1;36m=== Detached ===\033[0m\r\n")
					return nil
				}
			}
			s.ptm.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return nil
}

// AttachCmd returns an object that bubbletea can execute via tea.Exec.
func (m *Manager) AttachCmd(key string) PTYAttacher {
	return PTYAttacher{mgr: m, key: key}
}

// GetPTY returns the PTY master file for the session, or nil if not found.
// The returned file must not be closed by the caller.
func (m *Manager) GetPTY(key string) *os.File {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[key]; ok {
		return s.ptm
	}
	return nil
}

// ResizePTY resizes the PTY window for the given session.
func (m *Manager) ResizePTY(key string, cols, rows int) error {
	m.mu.Lock()
	s, ok := m.sessions[key]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", key)
	}
	return pty.Setsize(s.ptm, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// IsAlive reports whether the session process is still running.
func (m *Manager) IsAlive(key string) bool {
	m.mu.Lock()
	s, ok := m.sessions[key]
	m.mu.Unlock()
	if !ok {
		return false
	}
	return !s.exited.Load()
}

// sanitize replaces non-alphanumeric characters in profile names.
func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}
