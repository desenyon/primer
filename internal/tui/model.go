package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/desenyon/primer/internal/app"
	"github.com/desenyon/primer/internal/doctor"
	"github.com/desenyon/primer/internal/process"
	"github.com/desenyon/primer/internal/tui/theme"
)

type screen int

const (
	scan screen = iota
	plan
	dashboard
	diagnostics
	logs
	commands
	environment
)

type scanned struct {
	report doctor.Report
	err    error
}
type operation struct {
	err  error
	stop bool
}
type tick time.Time

type Model struct {
	ctx                              context.Context
	session                          *app.Session
	report                           doctor.Report
	snapshot                         process.Snapshot
	theme                            theme.Theme
	screen                           screen
	previous                         screen
	width, height                    int
	busy                             bool
	err                              string
	help, palette, filtering, paused bool
	query                            string
	selection                        int
	logOffset                        int
	reviewOffset                     int
	commandName                      string
}

func New(ctx context.Context, session *app.Session, output io.Writer, noColor bool) Model {
	return Model{ctx: ctx, session: session, theme: theme.New(output, noColor), width: 80, height: 24, screen: scan, commandName: "dev"}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { report, err := m.session.Inspect(m.ctx); return scanned{report, err} }
}
func refresh() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return tick(t) })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case scanned:
		if msg.err != nil {
			m.err = msg.err.Error()
		}
		m.report = msg.report
		m.screen = plan
	case operation:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err = ""
		m.screen = dashboard
		m.snapshot = m.session.Snapshot()
		return m, refresh()
	case tick:
		if !m.paused || m.screen != logs {
			m.snapshot = m.session.Snapshot()
		}
		return m, refresh()
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.help {
			if key == "esc" || key == "?" || key == "q" {
				m.help = false
			}
			return m, nil
		}
		if m.palette {
			return m.updatePalette(key)
		}
		if m.filtering {
			switch key {
			case "esc", "enter":
				m.filtering = false
			case "backspace":
				m.query = removeLast(m.query)
			default:
				if len(msg.Runes) > 0 {
					m.query += string(msg.Runes)
				}
			}
			m.logOffset = 0
			return m, nil
		}
		switch key {
		case "q":
			return m, tea.Quit
		case "?":
			m.help = true
		case ":", "ctrl+k":
			m.palette = true
			m.query = ""
			m.selection = 0
		case "esc":
			if m.screen == logs || m.screen == diagnostics || m.screen == commands || m.screen == environment {
				m.screen = m.previous
				m.query = ""
			}
		case "c":
			m.previous = m.screen
			m.screen = commands
			m.selection = 0
		case "e":
			m.previous = m.screen
			m.screen = environment
			m.selection = 0
		case "d":
			if m.screen == diagnostics {
				return m, nil
			}
			m.previous = m.screen
			m.screen = diagnostics
			m.selection = 0
		case "l":
			if m.screen == logs {
				return m, nil
			}
			m.previous = m.screen
			m.screen = logs
			m.query = ""
			m.logOffset = 0
		case "/":
			if m.screen == logs {
				m.filtering = true
				m.query = ""
			}
		case " ":
			if m.screen == logs {
				m.paused = !m.paused
			}
		case "up", "k":
			if m.screen == plan {
				m.reviewOffset = max(0, m.reviewOffset-1)
			} else if m.screen == diagnostics {
				m.selection = max(0, m.selection-1)
			} else if m.screen == commands || m.screen == environment {
				m.selection = max(0, m.selection-1)
			} else if m.screen == logs {
				m.paused = true
				m.logOffset++
			}
		case "down", "j":
			if m.screen == plan {
				m.reviewOffset++
			} else if m.screen == diagnostics {
				m.selection = min(max(0, len(m.report.Diagnostics)-1), m.selection+1)
			} else if m.screen == commands {
				m.selection = min(max(0, len(m.report.Project.Commands)-1), m.selection+1)
			} else if m.screen == environment {
				m.selection = min(max(0, len(m.report.Project.Environment)-1), m.selection+1)
			} else if m.screen == logs {
				m.logOffset = max(0, m.logOffset-1)
			}
		case "enter":
			if m.screen == commands && len(m.report.Project.Commands) > 0 && !m.busy && m.snapshot.State != process.Running {
				m.commandName = m.report.Project.Commands[m.selection].Name
				m.screen = plan
				m.reviewOffset = 0
				m.err = ""
				return m, nil
			}
			if m.screen == plan && !m.busy && !m.commandReport().Blocked() && m.err == "" && m.reviewComplete() {
				m.busy = true
				return m, func() tea.Msg { return operation{err: m.session.Run(m.ctx, m.selectedCommand())} }
			}
		case "r":
			if m.screen == dashboard && !m.busy {
				m.busy = true
				return m, func() tea.Msg { return operation{err: m.session.RestartCommand(m.ctx, m.selectedCommand())} }
			}
		case "s":
			if m.screen == dashboard && !m.busy {
				m.busy = true
				return m, func() tea.Msg { return operation{err: m.session.Stop(), stop: true} }
			}
		}
	}
	return m, nil
}

func removeLast(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		return string(r[:len(r)-1])
	}
	return s
}

func (m Model) paletteCommands() []string {
	commands := []string{"diagnostics", "logs", "overview", "project commands", "environment"}
	if m.screen == dashboard || m.snapshot.Started != (time.Time{}) {
		commands = append(commands, "restart command", "stop command")
	}
	if m.snapshot.State != process.Running && !m.busy {
		for _, c := range m.report.Project.Commands {
			commands = append(commands, "run "+c.Name)
		}
	}
	var found []string
	for _, command := range commands {
		if strings.Contains(command, strings.ToLower(m.query)) {
			found = append(found, command)
		}
	}
	return found
}

func (m Model) updatePalette(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.palette = false
		m.query = ""
	case "up":
		m.selection = max(0, m.selection-1)
	case "down":
		m.selection = min(max(0, len(m.paletteCommands())-1), m.selection+1)
	case "backspace":
		m.query = removeLast(m.query)
		m.selection = 0
	case "enter":
		choices := m.paletteCommands()
		if len(choices) == 0 {
			return m, nil
		}
		command := choices[min(m.selection, len(choices)-1)]
		m.palette = false
		m.query = ""
		switch command {
		case "project commands":
			m.previous = m.screen
			m.screen = commands
		case "environment":
			m.previous = m.screen
			m.screen = environment
		case "diagnostics":
			m.previous = m.screen
			m.screen = diagnostics
		case "logs":
			m.previous = m.screen
			m.screen = logs
		case "overview":
			if m.snapshot.Started.IsZero() {
				m.screen = plan
			} else {
				m.screen = dashboard
			}
		case "restart command":
			if !m.busy {
				m.busy = true
				return m, func() tea.Msg { return operation{err: m.session.RestartCommand(m.ctx, m.selectedCommand())} }
			}
		case "stop command":
			if !m.busy {
				m.busy = true
				return m, func() tea.Msg { return operation{err: m.session.Stop(), stop: true} }
			}
		}
		if strings.HasPrefix(command, "run ") && m.snapshot.State != process.Running && !m.busy {
			m.commandName = strings.TrimPrefix(command, "run ")
			m.screen = plan
			m.reviewOffset = 0
			m.err = ""
		}
		m.selection = 0
	default:
		if len([]rune(key)) == 1 {
			m.query += key
			m.selection = 0
		}
	}
	return m, nil
}

// SafeText removes terminal controls, including OSC hyperlinks and clipboard
// commands. Raw process escape sequences must not become UI instructions.
func SafeText(text string) string {
	text = ansi.Strip(text)
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			if r == '\t' {
				return ' '
			}
			return -1
		}
		return r
	}, text)
}

// PreviewText keeps multiline script syntax visible without emitting controls.
func PreviewText(text string) string {
	return SafeText(strings.ReplaceAll(strings.ReplaceAll(text, "\r", "\\r"), "\n", "\\n"))
}

func (m Model) View() string {
	w := max(12, min(108, m.width-4))
	h := max(1, m.height-4)
	var body string
	if m.help {
		body = m.helpView(w)
	} else if m.palette {
		body = m.paletteView(w)
	} else {
		switch m.screen {
		case scan:
			body = m.scanView(w)
		case plan:
			body = m.planView(w)
		case dashboard:
			body = m.dashboardView(w, h)
		case diagnostics:
			body = m.diagnosticView(w, h)
		case logs:
			body = m.logsView(w, h)
		case commands:
			body = m.commandsView(w, h)
		case environment:
			body = m.environmentView(w, h)
		}
	}
	// Width and height are both bounded, including hostile long repository names.
	lines := strings.Split(body, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, w, "")
	}
	return lipgloss.NewStyle().Margin(1, 2).Render(strings.Join(lines, "\n"))
}

func (m Model) header(w int) string {
	name := SafeText(m.report.Project.Name)
	if name == "" {
		name = "UNDERSTANDING REPOSITORY"
	}
	return m.theme.Title.Render("PRIMER") + m.theme.Muted.Render(" / "+name) + "\n" + m.theme.Divider(w)
}

func (m Model) scanView(w int) string {
	return m.theme.Hero.Width(w).Padding(1, 2).Render("PRIMER\n\n· · · · · ·  ░▒▓\n\nunderstanding repository") + "\n\nReading manifests and environment templates."
}

func (m Model) planContent(w int) []string {
	p := m.report.Project
	title := "REPOSITORY UNDERSTOOD"
	if m.commandReport().Blocked() || m.err != "" {
		title = "ATTENTION REQUIRED"
	}
	hero := m.theme.Hero.Width(w).Padding(1, 2).Render("PRIMER / " + SafeText(p.Name) + "\n" + title)
	details := []string{}
	if p.Framework != "" {
		details = append(details, p.Framework)
	}
	if p.Runtime.Name != "" {
		details = append(details, strings.TrimSpace(p.Runtime.Name+" "+p.Runtime.Required))
	}
	if p.Manager.Name != "" && !strings.EqualFold(p.Manager.Name, p.Runtime.Name) {
		details = append(details, p.Manager.Name)
	}
	if len(details) == 0 {
		details = append(details, "Declared project commands")
	}
	lines := []string{hero, "", SafeText(strings.Join(details, " · ")), ""}
	for _, t := range m.commandReport().Tools {
		state := "ready"
		if !t.Ready {
			state = "attention"
		}
		lines = append(lines, fmt.Sprintf("%s %-8s %-16s %s", m.theme.Status(state), SafeText(t.Name), SafeText(t.Version), state))
	}
	lines = append(lines, "", m.theme.Heading.Render("LAUNCH / REVIEW"))
	if dev, ok := p.Command(m.selectedCommand()); ok {
		lines = append(lines, "→ "+PreviewText(p.Redact(strings.Join(dev.Args, " "))))
		for _, name := range []string{"pre" + m.selectedCommand(), m.selectedCommand(), "post" + m.selectedCommand()} {
			for _, command := range p.Commands {
				if command.Name == name {
					lines = append(lines, "  "+name+": "+PreviewText(p.Redact(command.Script)))
				}
			}
		}
		lines = append(lines, m.theme.Muted.Render("Repository scripts execute with your local permissions."))
	}
	if len(m.commandReport().Diagnostics) > 0 {
		lines = append(lines, "", fmt.Sprintf("%s %d diagnostics · d to inspect", m.theme.Status("attention"), len(m.commandReport().Diagnostics)))
	}
	if m.err != "" {
		lines = append(lines, "", SafeText(m.err))
	}
	var wrapped []string
	for _, line := range lines {
		wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, w, true), "\n")...)
	}
	return wrapped
}

func (m Model) reviewComplete() bool {
	w := max(12, min(108, m.width-4))
	available := max(1, m.height-6)
	return m.reviewOffset >= max(0, len(m.planContent(w))-available)
}

func (m Model) planView(w int) string {
	lines := m.planContent(w)
	available := max(1, m.height-6)
	start := min(m.reviewOffset, max(0, len(lines)-available))
	end := min(len(lines), start+available)
	foot := "enter approve and launch    c commands    d issues    q quit"
	if !m.reviewComplete() {
		foot = "↓ review remaining lines    d diagnostics    q quit"
	}
	if m.commandReport().Blocked() || m.err != "" {
		foot = "↑↓ scroll    d diagnostics    q quit"
	}
	if m.busy {
		foot = "starting dev process    q cancel"
	}
	return strings.Join(append(lines[start:end], "", m.theme.Muted.Render(foot)), "\n")
}

func (m Model) dashboardView(w, h int) string {
	s := m.snapshot
	state := string(s.State)
	if s.State == process.Exited && s.ExitCode == 0 {
		state = "completed"
	}
	if state == "" {
		state = "stopped"
	}
	uptime := "--:--"
	if !s.Started.IsZero() && s.State == process.Running {
		uptime = time.Since(s.Started).Truncate(time.Second).String()
	}
	lines := []string{m.header(w), "", m.theme.Heading.Render(fmt.Sprintf("%-21s %-12s %-12s %s", "PROCESS", "STATE", "PID", "UPTIME")), fmt.Sprintf("%s %-19s %-12s %-12d %s", m.theme.Status(state), SafeText(m.selectedCommand()), state, s.PID, uptime)}
	if m.selectedCommand() == "dev" && m.report.Project.Port > 0 {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("  expected URL  http://localhost:%d", m.report.Project.Port)))
	}
	if s.ExitCode >= 0 {
		lines = append(lines, fmt.Sprintf("  exit %d · restarts %d", s.ExitCode, s.Restarts))
	}
	if m.err != "" {
		lines = append(lines, SafeText(m.err))
	}
	lines = append(lines, "", m.theme.Muted.Render(fmt.Sprintf("%d commands   %d environment entries   %d diagnostics", len(m.report.Project.Commands), len(m.report.Project.Environment), len(m.report.Diagnostics))), "", m.theme.Heading.Render("ACTIVITY"))
	count := max(0, h-len(lines)-4)
	if len(s.Logs) == 0 {
		lines = append(lines, m.theme.Muted.Render("No process output yet."))
	} else {
		for _, log := range tail(s.Logs, count) {
			lines = append(lines, renderLog(log, w))
		}
	}
	lines = append(lines, "", m.theme.Divider(w), m.theme.Muted.Render("l logs   c commands   e env   r restart   s stop   : palette   q quit"))
	return strings.Join(lines, "\n")
}

func renderLog(log process.Log, w int) string {
	return ansi.Truncate(fmt.Sprintf("%s  %-6s  %s", log.Time.Format("15:04:05"), strings.ToUpper(log.Stream), SafeText(log.Text)), w, "")
}
func tail(logs []process.Log, count int) []process.Log {
	if count == 0 {
		return nil
	}
	return logs[max(0, len(logs)-count):]
}

func (m Model) diagnosticView(w, h int) string {
	lines := []string{m.header(w), "", m.theme.Heading.Render("DIAGNOSTICS"), ""}
	if len(m.report.Diagnostics) == 0 {
		return strings.Join(append(lines, "● No issues found by the available checks.", "", "esc back    q quit"), "\n")
	}
	selected := min(m.selection, len(m.report.Diagnostics)-1)
	d := m.report.Diagnostics[selected]
	lines = append(lines, fmt.Sprintf("%d / %d", selected+1, len(m.report.Diagnostics)), m.theme.Focus.Render(" "+SafeText(d.Summary)+" "), "")
	if d.Expected != "" {
		lines = append(lines, "Expected  "+SafeText(d.Expected))
	}
	if d.Found != "" {
		lines = append(lines, "Found     "+SafeText(d.Found))
	}
	lines = append(lines, "", m.theme.Heading.Render("EVIDENCE"))
	for _, e := range d.Evidence {
		location := e.File
		if e.Line > 0 {
			location += fmt.Sprintf(":%d", e.Line)
		}
		lines = append(lines, SafeText(location+"  "+e.Detail))
	}
	if len(d.Evidence) == 0 {
		lines = append(lines, "Local system check.")
	}
	if d.Repair != "" {
		lines = append(lines, "", m.theme.Heading.Render("NEXT ACTION"))
		lines = append(lines, strings.Split(lipgloss.NewStyle().Width(w).Render(SafeText(d.Repair)), "\n")...)
	}
	footer := "↑↓ select    esc back    q quit"
	if len(lines) > h-2 {
		lines = lines[:max(0, h-2)]
	}
	return strings.Join(append(lines, "", m.theme.Muted.Render(footer)), "\n")
}

func (m Model) logsView(w, h int) string {
	state := "FOLLOW"
	if m.paused {
		state = "PAUSED"
	}
	lines := []string{m.header(w), "", m.theme.Heading.Render("LOGS / " + state)}
	if m.query != "" || m.filtering {
		lines = append(lines, "/ "+SafeText(m.query))
	}
	lines = append(lines, "")
	var entries []process.Log
	for _, entry := range m.snapshot.Logs {
		if strings.Contains(strings.ToLower(SafeText(entry.Text)), strings.ToLower(m.query)) {
			entries = append(entries, entry)
		}
	}
	count := max(0, h-len(lines)-3)
	offset := min(m.logOffset, max(0, len(entries)-count))
	entries = entries[:len(entries)-offset]
	if len(entries) == 0 {
		lines = append(lines, "No matching output.")
	} else {
		for _, entry := range tail(entries, count) {
			lines = append(lines, renderLog(entry, w))
		}
	}
	lines = append(lines, "", m.theme.Muted.Render("/ filter    space pause/follow    ↑↓ scroll    esc back    q quit"))
	return strings.Join(lines, "\n")
}

func (m Model) helpView(w int) string {
	return m.header(w) + "\n\nKEYBOARD\n\nenter     approve reviewed launch\nl         logs\nd         diagnostics and evidence\nc         discovered project commands\ne         environment names and states\nr         restart dev\ns         stop dev\n/         filter logs\nspace     pause / follow logs\n:         command palette\n↑↓ / j k  navigate\nesc       back\nq         quit and stop process group\n\n? / esc   close help"
}

func (m Model) paletteView(w int) string {
	lines := []string{m.header(w), "", m.theme.Heading.Render("COMMAND"), "", "> " + SafeText(m.query), ""}
	choices := m.paletteCommands()
	count := max(1, m.height-13)
	start := max(0, m.selection-count+1)
	for i := start; i < min(len(choices), start+count); i++ {
		command := choices[i]
		if i == m.selection {
			lines = append(lines, m.theme.Focus.Render(" → "+SafeText(command)+" "))
		} else {
			lines = append(lines, "   "+SafeText(command))
		}
	}
	if len(choices) == 0 {
		lines = append(lines, "No matching command.")
	}
	return strings.Join(append(lines, "", "↑↓ select    enter run    esc cancel"), "\n")
}

func (m Model) selectedCommand() string {
	if m.commandName == "" {
		return "dev"
	}
	return m.commandName
}
func (m Model) commandReport() doctor.Report { return m.report.ForCommand(m.selectedCommand()) }
func (m Model) commandsView(w, h int) string {
	lines := []string{m.header(w), "", m.theme.Heading.Render("PROJECT COMMANDS"), ""}
	entries := m.report.Project.Commands
	count := max(1, h-9)
	start := max(0, m.selection-count+1)
	for i := start; i < min(len(entries), start+count); i++ {
		c := entries[i]
		row := fmt.Sprintf("  %-18s %s", SafeText(c.Name), PreviewText(m.report.Project.Redact(c.Script)))
		row = ansi.Truncate(row, w, "")
		if i == m.selection {
			row = m.theme.Focus.Width(w).Render("→" + strings.TrimPrefix(row, " "))
		}
		lines = append(lines, row)
	}
	if len(entries) == 0 {
		lines = append(lines, "No declared commands found.")
	}
	if m.snapshot.State == process.Running {
		lines = append(lines, "", "Stop the active process before choosing another command.")
	} else {
		lines = append(lines, "", "Commands are reviewed before execution.")
	}
	return strings.Join(append(lines, "", m.theme.Muted.Render("↑↓ select   enter review   esc back   q quit")), "\n")
}
func (m Model) environmentView(w, h int) string {
	lines := []string{m.header(w), "", m.theme.Heading.Render("ENVIRONMENT / VALUES HIDDEN"), ""}
	entries := m.report.Project.Environment
	count := max(1, h-9)
	start := max(0, m.selection-count+1)
	for i := start; i < min(len(entries), start+count); i++ {
		e := entries[i]
		row := fmt.Sprintf("%s %-32s %s", m.theme.Status(e.State), SafeText(e.Name), e.State)
		lines = append(lines, row)
	}
	if len(entries) == 0 {
		lines = append(lines, "No environment template entries found.")
	}
	return strings.Join(append(lines, "", "Templates describe names; optionality is not inferred.", "", m.theme.Muted.Render("↑↓ scroll   esc back   q quit")), "\n")
}
