// Package process owns process lifecycle and a bounded log timeline. The TUI
// observes snapshots; it never starts or signals an OS process directly.
package process

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
)

const logLimit = 2000

type State string

const (
	Stopped State = "stopped"
	Running State = "running"
	Exited  State = "exited"
	Failed  State = "failed"
)

type Log struct {
	Sequence uint64    `json:"sequence"`
	Time     time.Time `json:"time"`
	Source   string    `json:"source"`
	Stream   string    `json:"stream"`
	Text     string    `json:"text"`
}

type Snapshot struct {
	State    State     `json:"state"`
	PID      int       `json:"pid,omitempty"`
	Started  time.Time `json:"started,omitempty"`
	ExitCode int       `json:"exit_code"`
	Restarts int       `json:"restarts"`
	Logs     []Log     `json:"logs"`
}

type Supervisor struct {
	op       sync.Mutex
	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	state    Snapshot
	secrets  []string
	logCount uint64
}

func New(secrets []string) *Supervisor {
	copySecrets := append([]string{}, secrets...)
	// Replace longer values first so overlapping credentials cannot leave tails.
	sort.Slice(copySecrets, func(i, j int) bool { return len(copySecrets[i]) > len(copySecrets[j]) })
	return &Supervisor{state: Snapshot{State: Stopped, ExitCode: -1}, secrets: copySecrets}
}

func (s *Supervisor) AddSecrets(secrets []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets = append(s.secrets, secrets...)
	sort.Slice(s.secrets, func(i, j int) bool { return len(s.secrets[i]) > len(s.secrets[j]) })
	s.secrets = slices.Compact(s.secrets)
}

func (s *Supervisor) Start(ctx context.Context, root string, args []string) error {
	s.op.Lock()
	defer s.op.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "" {
		return errors.New("no launch command")
	}
	s.mu.Lock()
	if s.state.State == Running {
		s.mu.Unlock()
		return errors.New("service is already running")
	}
	s.mu.Unlock()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	cmd.Env = os.Environ()
	if err := configureGroup(cmd); err != nil {
		return err
	}
	// Own the pipes so Wait cannot close them before their final bytes are read.
	outR, outW, err := os.Pipe()
	if err != nil {
		return err
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		outR.Close()
		outW.Close()
		return err
	}
	cmd.Stdout = outW
	cmd.Stderr = errW
	if err = cmd.Start(); err != nil {
		outR.Close()
		outW.Close()
		errR.Close()
		errW.Close()
		return fmt.Errorf("start %s: %w", args[0], err)
	}
	outW.Close()
	errW.Close()
	s.mu.Lock()
	if s.cmd != nil {
		s.state.Restarts++
	}
	s.cmd = cmd
	s.done = make(chan struct{})
	s.state.State = Running
	s.state.PID = cmd.Process.Pid
	s.state.Started = time.Now()
	s.state.ExitCode = -1
	done := s.done
	s.mu.Unlock()
	var readers sync.WaitGroup
	readers.Add(2)
	go func() { defer readers.Done(); s.read(outR, "stdout") }()
	go func() { defer readers.Done(); s.read(errR, "stderr") }()
	go func() {
		err := cmd.Wait()
		// The launcher can exit while descendants retain the pipes. Kill the
		// remaining group before joining readers, including on a failed launch.
		_ = killGroup(cmd)
		readers.Wait()
		s.mu.Lock()
		s.state.State = Exited
		if err != nil {
			s.state.State = Failed
		}
		s.state.ExitCode = cmd.ProcessState.ExitCode()
		s.mu.Unlock()
		close(done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = s.Stop()
		case <-done:
		}
	}()
	return nil
}

func (s *Supervisor) Stop() error {
	s.op.Lock()
	defer s.op.Unlock()
	s.mu.Lock()
	cmd, done := s.cmd, s.done
	s.mu.Unlock()
	if cmd == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	default:
	}
	err := terminateGroup(cmd)
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		_ = killGroup(cmd)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = killGroup(cmd)
		<-done
	}
	s.mu.Lock()
	s.state.State = Stopped
	s.mu.Unlock()
	return nil
}

func (s *Supervisor) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyState := s.state
	copyState.Logs = append([]Log{}, s.state.Logs...)
	for i := range copyState.Logs {
		text := ansi.Strip(copyState.Logs[i].Text)
		for _, secret := range s.secrets {
			if secret != "" {
				text = strings.ReplaceAll(text, secret, "[redacted]")
			}
		}
		copyState.Logs[i].Text = text
	}
	return copyState
}

func (s *Supervisor) read(pipe io.ReadCloser, stream string) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		s.appendLog(stream, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		s.appendLog("primer", "Log stream ended: line exceeds limit or pipe failed")
	}
}

func (s *Supervisor) appendLog(stream, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logCount++
	s.state.Logs = append(s.state.Logs, Log{Sequence: s.logCount, Time: time.Now(), Source: "dev", Stream: stream, Text: text})
	if len(s.state.Logs) > logLimit {
		s.state.Logs = append([]Log{}, s.state.Logs[len(s.state.Logs)-logLimit:]...)
	}
}
