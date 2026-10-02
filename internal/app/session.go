// Package app coordinates inspection, review and runtime. The terminal consumes
// this layer and contains no repository detection or process-management policy.
package app

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/desenyon/primer/internal/detect"
	"github.com/desenyon/primer/internal/doctor"
	"github.com/desenyon/primer/internal/process"
)

type Session struct {
	root       string
	mu         sync.Mutex
	report     doctor.Report
	supervisor *process.Supervisor
}

func New(root string) *Session { return &Session{root: root} }

func Inspect(ctx context.Context, root string) (doctor.Report, error) {
	p, err := detect.Scan(ctx, root)
	if err != nil {
		return doctor.Report{}, err
	}
	r := doctor.Check(ctx, p)
	return r, ctx.Err()
}

func (s *Session) Inspect(ctx context.Context) (doctor.Report, error) {
	r, err := Inspect(ctx, s.root)
	if err != nil {
		return r, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.report = r
	if s.supervisor == nil {
		s.supervisor = process.New(r.Project.Secrets)
	}
	return r, nil
}

// Start is called only after the user approves the preview. Re-inspection
// rejects changes to package scripts after review rather than running new code.
func (s *Session) Start(ctx context.Context) error { return s.Run(ctx, "dev") }

func (s *Session) Run(ctx context.Context, name string) error {
	s.mu.Lock()
	reviewed := s.report
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor == nil {
		return errors.New("inspect the repository before launching")
	}
	fresh, err := Inspect(ctx, s.root)
	if err != nil {
		return err
	}
	if fresh.Project.Fingerprint != reviewed.Project.Fingerprint {
		return errors.New("Repository launch evidence changed after review; quit and inspect again")
	}
	fresh = fresh.ForCommand(name)
	if fresh.Blocked() {
		return errors.New("launch blocked by local checks; resolve diagnostics and inspect again")
	}
	dev, ok := reviewed.Project.Command(name)
	if !ok {
		return errors.New("unknown project command: " + name)
	}
	freshDev, ok := fresh.Project.Command(name)
	if !ok || !slices.Equal(freshDev.Args, dev.Args) || fresh.Project.Root != reviewed.Project.Root {
		return errors.New("launch command changed after review; quit and inspect again")
	}
	supervisor.AddSecrets(fresh.Project.Secrets)
	return supervisor.Start(ctx, reviewed.Project.Root, dev.Args)
}

func (s *Session) Restart(ctx context.Context) error { return s.RestartCommand(ctx, "dev") }

func (s *Session) RestartCommand(ctx context.Context, name string) error {
	if err := s.Stop(); err != nil {
		return err
	}
	return s.Run(ctx, name)
}

func (s *Session) Stop() error {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor != nil {
		return supervisor.Stop()
	}
	return nil
}

func (s *Session) Snapshot() process.Snapshot {
	s.mu.Lock()
	supervisor := s.supervisor
	s.mu.Unlock()
	if supervisor == nil {
		return process.Snapshot{State: process.Stopped, ExitCode: -1, Logs: []process.Log{}}
	}
	return supervisor.Snapshot()
}
