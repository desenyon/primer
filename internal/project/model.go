package project

import (
	"sort"
	"strings"
)

// Evidence points to the repository fact supporting a conclusion. It never holds
// environment values or source contents that may contain credentials.
type Evidence struct {
	File   string `json:"file"`
	Line   int    `json:"line,omitempty"`
	Detail string `json:"detail"`
}

type Runtime struct {
	Confidence float64    `json:"confidence"`
	Name       string     `json:"name"`
	Required   string     `json:"required,omitempty"`
	Pins       []string   `json:"pins,omitempty"`
	Evidence   []Evidence `json:"evidence"`
}

type Manager struct {
	Confidence float64    `json:"confidence"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	Evidence   []Evidence `json:"evidence"`
}

type Command struct {
	Confidence float64    `json:"confidence"`
	Name       string     `json:"name"`
	Script     string     `json:"script"`
	Args       []string   `json:"args"`
	Evidence   []Evidence `json:"evidence"`
}

type EnvironmentVariable struct {
	Confidence float64    `json:"confidence"`
	Name       string     `json:"name"`
	State      string     `json:"state"`
	Evidence   []Evidence `json:"evidence"`
}

type Diagnostic struct {
	ID       string     `json:"id"`
	Summary  string     `json:"summary"`
	Expected string     `json:"expected,omitempty"`
	Found    string     `json:"found,omitempty"`
	Repair   string     `json:"repair,omitempty"`
	Blocking bool       `json:"blocking"`
	Evidence []Evidence `json:"evidence"`
}

type Project struct {
	DependenciesDeclared bool                  `json:"dependencies_declared"`
	Root                 string                `json:"root"`
	Name                 string                `json:"name"`
	Framework            string                `json:"framework,omitempty"`
	FrameworkEvidence    []Evidence            `json:"framework_evidence,omitempty"`
	FrameworkConfidence  float64               `json:"framework_confidence,omitempty"`
	Runtime              Runtime               `json:"runtime"`
	Manager              Manager               `json:"package_manager"`
	Commands             []Command             `json:"commands"`
	Environment          []EnvironmentVariable `json:"environment"`
	Port                 int                   `json:"port,omitempty"`
	PortConfidence       float64               `json:"port_confidence,omitempty"`
	PortEvidence         []Evidence            `json:"port_evidence,omitempty"`
	Diagnostics          []Diagnostic          `json:"diagnostics"`
	Fingerprint          string                `json:"fingerprint"`
	// Secrets only live in memory and never enter machine-readable output.
	Secrets []string `json:"-"`
}

func (p Project) DevCommand() (Command, bool) {
	for _, c := range p.Commands {
		if c.Name == "dev" {
			return c, true
		}
	}
	return Command{}, false
}

func (p Project) Blocked() bool {
	for _, d := range p.Diagnostics {
		if d.Blocking {
			return true
		}
	}
	return false
}

// Redact conceals known private environment values in previews and reports.
func (p Project) Redact(text string) string {
	secrets := append([]string{}, p.Secrets...)
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, value := range secrets {
		if value != "" {
			text = strings.ReplaceAll(text, value, "[redacted]")
		}
	}
	return text
}

func (p Project) Command(name string) (Command, bool) {
	for _, c := range p.Commands {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}
