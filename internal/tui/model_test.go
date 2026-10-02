package tui

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/desenyon/primer/internal/app"
	"github.com/desenyon/primer/internal/doctor"
	"github.com/desenyon/primer/internal/process"
	"github.com/desenyon/primer/internal/project"
)

func demoModel() Model {
	m := New(context.Background(), app.New("."), io.Discard, true)
	m.report = doctor.Report{Project: project.Project{Name: "roots", Framework: "Next.js", Runtime: project.Runtime{Name: "Node", Required: ">=22"}, Manager: project.Manager{Name: "pnpm"}, Commands: []project.Command{{Name: "dev", Script: "next dev", Args: []string{"pnpm", "run", "dev"}}}, Port: 3000}, Tools: []doctor.Tool{{Name: "node", Version: "22.4.1", Ready: true}, {Name: "pnpm", Version: "10.1.0", Ready: true}}}
	m.snapshot = process.Snapshot{State: process.Running, PID: 81042, Started: time.Now().Add(-time.Minute), ExitCode: -1, Logs: []process.Log{{Time: time.Now(), Stream: "stdout", Text: "ready on localhost:3000"}}}
	return m
}

func TestScreenDimensionsAndNoColor(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 35}, {180, 45}, {40, 16}} {
		for _, screen := range []screen{scan, plan, dashboard, diagnostics, logs, commands, environment} {
			m := demoModel()
			m.width = size.w
			m.height = size.h
			m.screen = screen
			m.report.Diagnostics = []project.Diagnostic{{Summary: strings.Repeat("long diagnostic ", 20), Expected: strings.Repeat("expected ", 20), Repair: strings.Repeat("Review the configuration. ", 20)}}
			view := m.View()
			if strings.Contains(view, "\x1b") {
				t.Fatal("no-color screen contains terminal escapes")
			}
			lines := strings.Split(view, "\n")
			if len(lines) > size.h {
				t.Fatalf("screen %d: %d rows at %dx%d", screen, len(lines), size.w, size.h)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size.w {
					t.Fatalf("screen %d overflows: %d columns at %dx%d", screen, ansi.StringWidth(line), size.w, size.h)
				}
			}
		}
	}
}

func TestPlanAndKeyboardSafety(t *testing.T) {
	m := demoModel()
	m.screen = plan
	m.report.Diagnostics = []project.Diagnostic{{Blocking: true, Summary: "missing runtime"}}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil {
		t.Fatal("blocked plan started a command")
	}
	if !strings.Contains(m.View(), "Repository scripts execute") {
		t.Fatal("missing command risk preview")
	}
	m.screen = dashboard
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	if !updated.(Model).palette {
		t.Fatal("palette did not open")
	}
}

func TestSafeTextStripsTerminalInstructions(t *testing.T) {
	text := "\x1b[31merror\x1b[0m\x1b]52;c;Y2xpcGJvYXJk\x07\rnext"
	if got := SafeText(text); got != "errornext" {
		t.Fatalf("unsafe text: %q", got)
	}
	if got := PreviewText("next dev\nsecond command"); got != "next dev\\nsecond command" {
		t.Fatalf("script syntax hidden: %q", got)
	}
}

func TestLongCommandMustBeReviewedBeforeApproval(t *testing.T) {
	m := demoModel()
	m.screen = plan
	m.report.Project.Commands[0].Script = strings.Repeat("echo inspect-this; ", 100) + "echo final-command"
	if m.reviewComplete() {
		t.Fatal("long command incorrectly treated as visible")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("approved unseen command")
	}
	for i := 0; i < 200 && !m.reviewComplete(); i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(Model)
	}
	if !m.reviewComplete() || !strings.Contains(m.View(), "final-command") {
		t.Fatal("cannot inspect end of command")
	}
	if !strings.Contains(m.View(), "q quit") {
		t.Fatal("review footer is hidden")
	}
}

func TestCommandBrowserRequiresSeparateReview(t *testing.T) {
	m := demoModel()
	m.screen = commands
	m.snapshot.State = process.Stopped
	m.report.Project.Commands = append(m.report.Project.Commands, project.Command{Name: "test", Script: "go test ./...", Args: []string{"go", "test", "./..."}})
	m.selection = 1
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.screen != plan || m.selectedCommand() != "test" || !strings.Contains(m.View(), "go test") {
		t.Fatal("command bypassed review")
	}
}

func TestPaletteDiscoversAndReviewsProjectCommands(t *testing.T) {
	m := demoModel()
	m.snapshot.State = process.Stopped
	m.screen = plan
	m.palette = true
	m.query = "run dev"
	if len(m.paletteCommands()) != 1 {
		t.Fatalf("palette: %v", m.paletteCommands())
	}
	updated, cmd := m.updatePalette("enter")
	m = updated.(Model)
	if cmd != nil || m.palette || m.screen != plan {
		t.Fatal("palette skipped review")
	}
}

func TestProcfileArgumentPreviewRedactsSecrets(t *testing.T) {
	m := demoModel()
	m.screen = plan
	m.report.Project.Secrets = []string{"private-token"}
	m.report.Project.Commands[0].Args = []string{"sh", "-c", "echo private-token"}
	if strings.Contains(m.View(), "private-token") {
		t.Fatal("secret leaked from argv")
	}
}
