package detect

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/desenyon/primer/internal/project"
)

// Scan dispatches bounded detectors and combines explicitly declared commands.
// No repository executable is invoked during detection.
func Scan(ctx context.Context, start string) (project.Project, error) {
	root, err := Locate(start)
	if err != nil {
		return project.Project{}, err
	}
	p := project.Project{Root: root, Name: filepath.Base(root), Commands: []project.Command{}, Environment: []project.EnvironmentVariable{}, Diagnostics: []project.Diagnostic{}}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	switch {
	case exists(root, "package.json"):
		p, err = scanNode(ctx, root)
	case exists(root, "pyproject.toml") || exists(root, "requirements.txt"):
		err = detectPython(&p)
	case exists(root, "go.mod"):
		err = detectGo(&p)
	case exists(root, "Cargo.toml"):
		err = detectRust(&p)
	}
	if err != nil {
		return p, err
	}
	if err = detectCommands(&p); err != nil {
		return p, err
	}
	if _, ok := p.DevCommand(); ok {
		diagnostics := []project.Diagnostic{}
		for _, d := range p.Diagnostics {
			if d.ID != "missing-dev-command" {
				diagnostics = append(diagnostics, d)
			}
		}
		p.Diagnostics = diagnostics
	} else if !hasDiagnostic(p, "missing-dev-command") {
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "missing-dev-command", Summary: "No development launch found", Repair: "Declare a dev or web process in Procfile, or a dev target in Makefile. Use primer commands to inspect other available commands.", Blocking: true})
	}
	if p.Runtime.Name != "Node" {
		if err = detectEnvironment(&p); err != nil {
			return p, err
		}
	}
	hash := sha256.New()
	// Include every bounded source used for inference and command review.
	for _, name := range []string{"package.json", "pyproject.toml", "requirements.txt", "uv.lock", "poetry.lock", "go.mod", "Cargo.toml", "Procfile", "Makefile", ".python-version", ".nvmrc", ".node-version", "manage.py", "app.py", "main.py", "app/main.py", "main.go", "cmd/server/main.go", "src/main.rs"} {
		data, e := ReadFile(root, name)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return p, e
		}
		fmt.Fprintf(hash, "%s\x00", name)
		hash.Write(data)
	}
	if p.Port < 0 || p.Port > 65535 {
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "invalid-port", Summary: "Declared port is invalid", Blocking: true, Evidence: p.PortEvidence})
		p.Port = 0
	}
	p.Fingerprint = fmt.Sprintf("%x", hash.Sum(nil))
	return p, ctx.Err()
}
func exists(root, name string) bool { _, err := os.Lstat(filepath.Join(root, name)); return err == nil }
func hasDiagnostic(p project.Project, id string) bool {
	for _, d := range p.Diagnostics {
		if d.ID == id {
			return true
		}
	}
	return false
}
func command(name, script, file string, args ...string) project.Command {
	return project.Command{Name: name, Script: script, Args: args, Confidence: 1, Evidence: []project.Evidence{{File: file, Detail: "Declared command " + name}}}
}

var targetPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*):(?:[^=]|$)`)

func detectCommands(p *project.Project) error {
	for _, file := range []string{"Procfile", "Makefile"} {
		data, err := ReadFile(p.Root, file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			var c project.Command
			if file == "Procfile" {
				name, script, ok := strings.Cut(line, ":")
				name = strings.TrimSpace(name)
				script = strings.TrimSpace(script)
				if !ok || !envKey.MatchString(name) || script == "" {
					continue
				}
				c = command(name, script, file, "sh", "-c", script)
			} else {
				match := targetPattern.FindStringSubmatch(line)
				if len(match) < 2 {
					continue
				}
				name := match[1]
				c = command(name, "make "+name+" (recipes, includes and shell expansions execute)", file, "make", name)
			}
			c.Evidence[0].Line = i + 1
			// Existing language commands win; explicit dev/web declarations override an inferred launch.
			duplicate := false
			for _, old := range p.Commands {
				if old.Name == c.Name {
					duplicate = true
				}
			}
			if !duplicate {
				p.Commands = append(p.Commands, c)
			}
			if (c.Name == "web" || c.Name == "dev") && p.Runtime.Name != "Node" {
				c.Name = "dev"
				p.Port, p.PortConfidence, p.PortEvidence = 0, 0, nil
				if match := explicitPort.FindStringSubmatch(c.Script); len(match) > 1 {
					p.Port, _ = strconv.Atoi(match[1])
					p.PortConfidence = 1
					p.PortEvidence = c.Evidence
				}
				var commands []project.Command
				for _, old := range p.Commands {
					if old.Name != "dev" {
						commands = append(commands, old)
					}
				}
				p.Commands = append(commands, c)
			}
		}
	}
	sort.SliceStable(p.Commands, func(i, j int) bool { return p.Commands[i].Name < p.Commands[j].Name })
	return nil
}
