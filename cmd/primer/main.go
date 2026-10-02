package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/desenyon/primer/internal/app"
	"github.com/desenyon/primer/internal/detect"
	"github.com/desenyon/primer/internal/doctor"
	"github.com/desenyon/primer/internal/process"
	"github.com/desenyon/primer/internal/tui"
	"github.com/mattn/go-isatty"
)

// Release builds set version through the linker; source builds remain explicit.
var version = "dev"

const help = `PRIMER

Understand a repository. Review its commands. Bring it up.

Usage
  primer                       scan, review and enter dashboard
  primer start                 scan, review and launch
  primer commands              list discovered project commands
  primer run NAME              review and run a discovered command
  primer why                   explain current diagnostics and evidence
  primer doctor                check without launching
  primer env                   inspect environment template names and states

Options
  --path DIR                   inspect this directory (default .)
  --json                       machine-readable inspection
  --no-color                   plain presentation
  --no-interactive             useful plain output; no implicit execution
  --yes                        explicitly approve the reviewed command
  --version                    show version
  --help                       show help

Non-interactive launch
  primer start --yes --no-interactive

Quit stops the entire dev process group. Launch requires macOS or Linux.
`

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	interactive := isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) && os.Getenv("TERM") != "dumb" && os.Getenv("CI") == ""
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, interactive))
}

func run(ctx context.Context, args []string, input io.Reader, output, errOutput io.Writer, interactive bool) int {
	command := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command != "" && command != "start" && command != "doctor" && command != "env" && command != "commands" && command != "run" && command != "why" {
		fmt.Fprintf(errOutput, "Unknown command: %s\nUse primer --help.\n", tui.SafeText(command))
		return 2
	}
	selected := "dev"
	if command == "run" {
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			fmt.Fprintln(errOutput, "Use primer run NAME [options].")
			return 2
		}
		selected = args[0]
		args = args[1:]
	}
	flags := flag.NewFlagSet("primer", flag.ContinueOnError)
	flags.SetOutput(errOutput)
	path := flags.String("path", ".", "repository directory")
	jsonOutput := flags.Bool("json", false, "JSON inspection")
	noColor := flags.Bool("no-color", false, "disable color")
	noInteractive := flags.Bool("no-interactive", false, "plain output")
	yes := flags.Bool("yes", false, "approve repository dev script")
	showHelp := flags.Bool("help", false, "help")
	showVersion := flags.Bool("version", false, "version")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(output, help)
			return 0
		}
		return 2
	}
	if *showHelp {
		fmt.Fprint(output, help)
		return 0
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(errOutput, "Unexpected argument. Use primer --help.")
		return 2
	}
	if *showVersion {
		fmt.Fprintf(output, "primer %s\n", version)
		return 0
	}
	if *yes && command != "start" && command != "run" {
		fmt.Fprintln(errOutput, "--yes is only valid with primer start or primer run.")
		return 2
	}
	if *yes && *jsonOutput {
		fmt.Fprintln(errOutput, "--json is inspection only; remove --yes.")
		return 2
	}
	interactive = interactive && !*noInteractive && !*jsonOutput
	if command == "env" {
		p, err := detect.Scan(ctx, *path)
		if err != nil {
			fmt.Fprintln(errOutput, tui.SafeText(err.Error()))
			return 1
		}
		if *jsonOutput {
			if err := json.NewEncoder(output).Encode(p.Environment); err != nil {
				return 1
			}
		} else {
			fmt.Fprint(output, "ENVIRONMENT\n\n")
			for _, variable := range p.Environment {
				fmt.Fprintf(output, "%-28s %s\n", variable.Name, variable.State)
			}
			if len(p.Environment) == 0 {
				fmt.Fprintln(output, "No environment template entries found.")
			}
		}
		for _, variable := range p.Environment {
			if variable.State != "configured" {
				return 1
			}
		}
		return 0
	}
	if interactive && command != "doctor" && command != "commands" && command != "why" && command != "run" && !*yes {
		sessionCtx, cancel := context.WithCancel(ctx)
		session := app.New(*path)
		defer func() { cancel(); _ = session.Stop() }()
		_, err := tea.NewProgram(tui.New(sessionCtx, session, output, *noColor), tea.WithInput(input), tea.WithOutput(output), tea.WithAltScreen(), tea.WithContext(sessionCtx)).Run()
		if err != nil && ctx.Err() == nil {
			fmt.Fprintln(errOutput, tui.SafeText(err.Error()))
			return 1
		}
		return 0
	}
	session := app.New(*path)
	defer session.Stop()
	r, err := session.Inspect(ctx)
	if err != nil {
		fmt.Fprintln(errOutput, tui.SafeText(err.Error()))
		return 1
	}
	if command == "commands" {
		if *jsonOutput {
			if err := writeJSON(output, r.Project.Commands, r.Project.Redact); err != nil {
				return 1
			}
		} else {
			fmt.Fprint(output, "COMMANDS\n\n")
			for _, c := range r.Project.Commands {
				fmt.Fprintf(output, "%-18s %s\n", tui.SafeText(c.Name), tui.PreviewText(r.Project.Redact(c.Script)))
			}
			fmt.Fprintln(output, "\nRun with primer run NAME. Each command is reviewed before execution.")
		}
		return 0
	}
	if command == "run" {
		if _, ok := r.Project.Command(selected); !ok {
			fmt.Fprintln(errOutput, "Unknown project command:", tui.SafeText(selected))
			return 2
		}
		r = r.ForCommand(selected)
		if !*jsonOutput && !*yes {
			printNamedCommand(output, r, selected)
		}
	}
	if *jsonOutput {
		if err := writeJSON(output, r, r.Project.Redact); err != nil {
			return 1
		}
	} else {
		printReport(output, r)
	}
	approved := *yes
	if command == "run" && !approved && interactive {
		fmt.Fprint(output, "\nRun this command? [y/N] ")
		var answer string
		fmt.Fscanln(input, &answer)
		approved = answer == "y" || answer == "Y"
	}
	if (command == "start" || command == "run") && approved {
		if r.Blocked() {
			fmt.Fprintln(errOutput, "Launch blocked. Resolve the diagnostics and run again.")
			return 1
		}
		if _, ok := r.Project.Command(selected); ok {
			fmt.Fprintln(output, "\nAPPROVED LAUNCH")
			printNamedCommand(output, r, selected)
		}
		if err := session.Run(ctx, selected); err != nil {
			fmt.Fprintln(errOutput, tui.SafeText(err.Error()))
			return 1
		}
		return follow(ctx, session, output)
	}
	if command == "run" && !*jsonOutput {
		fmt.Fprintf(output, "\nApprove explicitly with primer run %s --yes --no-interactive\n", tui.SafeText(selected))
	} else if command != "doctor" && command != "why" && !*jsonOutput {
		fmt.Fprintln(output, "\nLaunch interactively with primer, or approve explicitly:\n  primer start --yes --no-interactive")
	}
	if len(r.Diagnostics) > 0 {
		return 1
	}
	return 0
}

// Redact string values structurally so quoted/multiline credentials cannot
// corrupt JSON, and numeric values and object keys keep their original meaning.
func writeJSON(output io.Writer, value any, redact func(string) string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var public any
	if err := decoder.Decode(&public); err != nil {
		return err
	}
	var scrub func(any) any
	scrub = func(value any) any {
		switch value := value.(type) {
		case string:
			return redact(value)
		case []any:
			for i, entry := range value {
				value[i] = scrub(entry)
			}
			return value
		case map[string]any:
			for key, entry := range value {
				value[key] = scrub(entry)
			}
			return value
		default:
			return value
		}
	}
	return json.NewEncoder(output).Encode(scrub(public))
}

func printReport(output io.Writer, r doctor.Report) {
	p := r.Project
	fmt.Fprintf(output, "PRIMER / %s\n\nPROJECT\n%s\n", tui.SafeText(p.Name), tui.SafeText(p.Root))
	if p.Framework != "" {
		fmt.Fprintln(output, p.Framework)
	}
	fmt.Fprintln(output, "\nRUNTIME")
	for _, tool := range r.Tools {
		state := "ready"
		if !tool.Ready {
			state = "unavailable"
		}
		fmt.Fprintf(output, "%-8s %-16s %s\n", tool.Name, tui.SafeText(tool.Version), state)
	}
	fmt.Fprintf(output, "\n%d environment variables · %d diagnostics\n", len(p.Environment), len(r.Diagnostics))
	for _, d := range r.Diagnostics {
		fmt.Fprintf(output, "\n%s\n", tui.SafeText(p.Redact(d.Summary)))
		if d.Expected != "" {
			fmt.Fprintln(output, "Expected:", tui.SafeText(d.Expected))
		}
		if d.Found != "" {
			fmt.Fprintln(output, "Found:   ", tui.SafeText(d.Found))
		}
		for _, e := range d.Evidence {
			location := e.File
			if e.Line > 0 {
				location += fmt.Sprintf(":%d", e.Line)
			}
			fmt.Fprintln(output, "Evidence:", tui.SafeText(location+"  "+e.Detail))
		}
		if d.Repair != "" {
			fmt.Fprintln(output, tui.SafeText(d.Repair))
		}
	}
}

func printCommand(output io.Writer, r doctor.Report) { printNamedCommand(output, r, "dev") }

func printNamedCommand(output io.Writer, r doctor.Report, name string) {
	dev, _ := r.Project.Command(name)
	fmt.Fprintln(output, tui.PreviewText(r.Project.Redact(strings.Join(dev.Args, " "))))
	for _, name := range []string{"pre" + name, name, "post" + name} {
		for _, c := range r.Project.Commands {
			if c.Name == name {
				fmt.Fprintf(output, "  %s: %s\n", name, tui.PreviewText(r.Project.Redact(c.Script)))
			}
		}
	}
	fmt.Fprintln(output, "Repository scripts execute with your local permissions.")
}

func follow(ctx context.Context, session *app.Session, output io.Writer) int {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var sequence uint64
	for {
		snapshot := session.Snapshot()
		for _, log := range snapshot.Logs {
			if log.Sequence > sequence {
				fmt.Fprintf(output, "%s  %-6s  %s\n", log.Time.Format("15:04:05"), log.Stream, tui.SafeText(log.Text))
				sequence = log.Sequence
			}
		}
		if snapshot.State == process.Failed || snapshot.State == process.Exited {
			if snapshot.ExitCode < 0 {
				return 1
			}
			return max(0, snapshot.ExitCode)
		}
		select {
		case <-ctx.Done():
			return 0
		case <-ticker.C:
		}
	}
}
