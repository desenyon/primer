package theme

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const (
	Blue  = "#2D24E8"
	Paper = "#F4F2EC"
	Muted = "#93918B"
)

type Theme struct {
	Title   lipgloss.Style
	Heading lipgloss.Style
	Muted   lipgloss.Style
	Focus   lipgloss.Style
	Hero    lipgloss.Style
	Rule    lipgloss.Style
	Plain   bool
	ASCII   bool
}

func New(output io.Writer, noColor bool) Theme {
	r := lipgloss.NewRenderer(output)
	term := os.Getenv("TERM")
	_, hasNoColor := os.LookupEnv("NO_COLOR")
	plain := noColor || hasNoColor || term == "dumb"
	if plain {
		r.SetColorProfile(termenv.Ascii)
	} else if strings.Contains(os.Getenv("COLORTERM"), "truecolor") || strings.Contains(os.Getenv("COLORTERM"), "24bit") {
		r.SetColorProfile(termenv.TrueColor)
	}
	return Theme{
		Title:   r.NewStyle().Bold(true).Foreground(lipgloss.Color(Paper)),
		Heading: r.NewStyle().Bold(true),
		Muted:   r.NewStyle().Foreground(lipgloss.Color(Muted)),
		Focus:   r.NewStyle().Foreground(lipgloss.Color(Paper)).Background(lipgloss.Color(Blue)),
		Hero:    r.NewStyle().Foreground(lipgloss.Color(Paper)).Background(lipgloss.Color(Blue)),
		Rule:    r.NewStyle().Foreground(lipgloss.Color(Blue)),
		Plain:   plain, ASCII: term == "dumb",
	}
}

func (t Theme) Status(state string) string {
	symbol := "○"
	switch state {
	case "ready", "running", "configured":
		symbol = "●"
	case "attention", "empty", "missing", "checking":
		symbol = "◐"
	case "failed", "exited":
		symbol = "×"
	case "completed":
		symbol = "✓"
	}
	if t.ASCII {
		switch symbol {
		case "●":
			symbol = "+"
		case "◐":
			symbol = "!"
		case "×":
			symbol = "x"
		case "✓":
			symbol = "+"
		default:
			symbol = "-"
		}
	}
	return symbol
}

func (t Theme) Divider(width int) string {
	character := "─"
	if t.ASCII {
		character = "-"
	}
	return t.Rule.Render(strings.Repeat(character, max(0, width)))
}
