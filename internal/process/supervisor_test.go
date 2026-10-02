//go:build darwin || linux

package process

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCaptureFinalOutputAndRedact(t *testing.T) {
	s := New([]string{"secret-token"})
	defer s.Stop()
	if err := s.Start(context.Background(), t.TempDir(), []string{"/bin/sh", "-c", "printf 'secret-\\033[31mtoken\\033[0m\\n'; printf 'last error\\n' >&2; exit 7"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return s.Snapshot().ExitCode == 7 })
	snapshot := s.Snapshot()
	if snapshot.State != Failed || len(snapshot.Logs) != 2 {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	for _, entry := range snapshot.Logs {
		if strings.Contains(entry.Text, "secret-token") {
			t.Fatal("secret leaked")
		}
	}
	if snapshot.Logs[0].Sequence == snapshot.Logs[1].Sequence {
		t.Fatal("log sequence must be unique")
	}
}

func TestStopReapsProcessTree(t *testing.T) {
	s := New(nil)
	defer s.Stop()
	script := `sleep 60 & child=$!; trap 'kill "$child" 2>/dev/null; wait "$child"; exit 0' TERM; echo child:$child; wait "$child"`
	if err := s.Start(context.Background(), t.TempDir(), []string{"/bin/sh", "-c", script}); err != nil {
		t.Fatal(err)
	}
	var child int
	waitFor(t, func() bool {
		for _, entry := range s.Snapshot().Logs {
			if strings.HasPrefix(entry.Text, "child:") {
				child, _ = strconv.Atoi(strings.TrimPrefix(entry.Text, "child:"))
				return child > 0
			}
		}
		return false
	})
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return syscall.Kill(child, 0) == syscall.ESRCH })
	if s.Snapshot().State != Stopped {
		t.Fatal("stop did not transition state")
	}
}

func TestCancellationEscalatesAndRestartWorks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := New(nil)
	defer s.Stop()
	if err := s.Start(ctx, t.TempDir(), []string{"/bin/sh", "-c", `trap '' TERM; echo ready; while :; do :; done`}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(s.Snapshot().Logs) > 0 })
	pid := s.Snapshot().PID
	cancel()
	waitFor(t, func() bool { return s.Snapshot().State == Stopped })
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("process %d remains: %v", pid, err)
	}
	if err := s.Start(context.Background(), t.TempDir(), []string{"/bin/sh", "-c", "echo restarted"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return s.Snapshot().State == Exited })
	if s.Snapshot().Restarts != 1 {
		t.Fatal("restart count")
	}
}

func TestLogBufferBounded(t *testing.T) {
	s := New(nil)
	for i := 0; i < logLimit+7; i++ {
		s.appendLog("stdout", fmt.Sprint(i))
	}
	snapshot := s.Snapshot()
	if len(snapshot.Logs) != logLimit || snapshot.Logs[0].Text != "7" {
		t.Fatal("unbounded or incorrect log history")
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for process state")
}
