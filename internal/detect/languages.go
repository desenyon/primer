package detect

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/desenyon/primer/internal/project"
	"github.com/pelletier/go-toml/v2"
)

func detectPython(p *project.Project) error {
	p.Runtime = project.Runtime{Name: "Python", Confidence: 1, Evidence: []project.Evidence{{File: "requirements.txt", Detail: "Python dependencies"}}}
	p.Manager = project.Manager{Name: "pip", Confidence: .5, Evidence: []project.Evidence{{File: "requirements.txt", Detail: "No uv or Poetry declaration; pip fallback"}}}
	var dependencyEvidence []project.Evidence
	var dependencies []string
	var scripts map[string]string
	if exists(p.Root, "pyproject.toml") {
		data, err := ReadFile(p.Root, "pyproject.toml")
		if err != nil {
			return err
		}
		var config struct {
			Project struct {
				Name           string
				RequiresPython string `toml:"requires-python"`
				Dependencies   []string
				Scripts        map[string]string
			}
			Tool struct {
				Poetry struct {
					Name         string
					Dependencies map[string]any
					Scripts      map[string]string
				}
				Uv map[string]any `toml:"uv"`
			}
		}
		if err = toml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("read pyproject.toml: %w", err)
		}
		if config.Project.Name != "" {
			p.Name = config.Project.Name
		} else if config.Tool.Poetry.Name != "" {
			p.Name = config.Tool.Poetry.Name
		}
		p.Runtime.Required = config.Project.RequiresPython
		p.Runtime.Evidence = []project.Evidence{{File: "pyproject.toml", Detail: "Python project declaration"}}
		p.Manager.Evidence = []project.Evidence{{File: "pyproject.toml", Detail: "No uv or Poetry declaration; pip fallback"}}
		dependencyEvidence = append(dependencyEvidence, project.Evidence{File: "pyproject.toml", Detail: "Declared Python dependencies"})
		dependencies = config.Project.Dependencies
		scripts = config.Project.Scripts
		if len(config.Tool.Poetry.Dependencies) > 0 {
			p.Manager.Name = "poetry"
			p.Manager.Confidence = 1
			p.Manager.Evidence = []project.Evidence{{File: "pyproject.toml", Detail: "tool.poetry declaration"}}
			for name := range config.Tool.Poetry.Dependencies {
				dependencies = append(dependencies, name)
			}
			if v, ok := config.Tool.Poetry.Dependencies["python"].(string); ok && p.Runtime.Required == "" {
				p.Runtime.Required = v
			}
			if scripts == nil {
				scripts = config.Tool.Poetry.Scripts
			}
		}
		if config.Tool.Uv != nil {
			p.Manager = project.Manager{Name: "uv", Confidence: 1, Evidence: []project.Evidence{{File: "pyproject.toml", Detail: "tool.uv declaration"}}}
		}
	}
	if exists(p.Root, "uv.lock") {
		p.Manager.Name = "uv"
		p.Manager.Confidence = 1
		p.Manager.Evidence = []project.Evidence{{File: "uv.lock", Detail: "uv lockfile"}}
	}
	if exists(p.Root, "poetry.lock") && p.Manager.Name == "uv" {
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "ambiguous-manager", Summary: "Python lockfiles conflict", Blocking: true, Repair: "Keep the intended uv or Poetry lockfile."})
	}
	if exists(p.Root, "poetry.lock") && p.Manager.Name != "uv" {
		p.Manager = project.Manager{Name: "poetry", Confidence: 1, Evidence: []project.Evidence{{File: "poetry.lock", Detail: "Poetry lockfile"}}}
	}
	if exists(p.Root, "requirements.txt") {
		data, err := ReadFile(p.Root, "requirements.txt")
		if err != nil {
			return err
		}
		dependencies = append(dependencies, strings.Split(string(data), "\n")...)
		dependencyEvidence = append(dependencyEvidence, project.Evidence{File: "requirements.txt", Detail: "Python dependency requirements"})
	}
	if exists(p.Root, ".python-version") {
		data, err := ReadFile(p.Root, ".python-version")
		if err != nil {
			return err
		}
		p.Runtime.Pins = []string{strings.TrimSpace(string(data))}
		p.Runtime.Evidence = append(p.Runtime.Evidence, project.Evidence{File: ".python-version", Detail: "Python version pin"})
	}
	for _, dep := range dependencies {
		lower := strings.ToLower(dep)
		for _, f := range []struct{ key, name string }{{"fastapi", "FastAPI"}, {"django", "Django"}, {"flask", "Flask"}} {
			if regexp.MustCompile(`^` + f.key + `(?:\[|[<>=!~; \t]|$)`).MatchString(lower) {
				p.Framework = f.name
				p.FrameworkConfidence = .9
				p.FrameworkEvidence = dependencyEvidence
			}
		}
	}
	prefix := []string{"python3"}
	if p.Manager.Name == "uv" {
		prefix = []string{"uv", "run", "--no-sync", "--no-python-downloads", "python"}
	} else if p.Manager.Name == "poetry" {
		prefix = []string{"poetry", "run", "python"}
	} else if exists(p.Root, ".venv/bin/python") {
		prefix = []string{".venv/bin/python"}
	}
	for name, entry := range scripts {
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]*$`).MatchString(name) {
			continue
		}
		var args []string
		if p.Manager.Name == "uv" {
			args = []string{"uv", "run", "--no-sync", "--no-python-downloads", name}
		} else if p.Manager.Name == "poetry" {
			args = []string{"poetry", "run", name}
		} else {
			args = []string{".venv/bin/" + name}
		}
		p.Commands = append(p.Commands, command(name, entry, "pyproject.toml", args...))
	}
	// Framework entrypoints are inferred only from a small set of conventional files.
	_, declaredDev := p.DevCommand()
	if !declaredDev && p.Framework == "Django" && exists(p.Root, "manage.py") {
		c := command("dev", "manage.py runserver", "manage.py", append(prefix, "manage.py", "runserver", "127.0.0.1:8000")...)
		c.Confidence = .9
		c.Evidence[0].Detail = "Conventional Django manage.py; inferred launch"
		p.Commands = append(p.Commands, c)
		p.PortEvidence = c.Evidence
		p.Port = 8000
		p.PortConfidence = .8
	}
	if !declaredDev && (p.Framework == "FastAPI" || p.Framework == "Flask") {
		for _, file := range []string{"main.py", "app.py", "app/main.py"} {
			if !exists(p.Root, file) {
				continue
			}
			data, err := ReadFile(p.Root, file)
			if err != nil {
				return err
			}
			if !strings.Contains(string(data), "app = "+p.Framework+"(") && !strings.Contains(string(data), "app="+p.Framework+"(") {
				continue
			}
			module := strings.ReplaceAll(strings.TrimSuffix(file, ".py"), "/", ".")
			args := append(append([]string{}, prefix...), "-m", "uvicorn", module+":app", "--host", "127.0.0.1", "--port", "8000", "--reload")
			p.Port = 8000
			if p.Framework == "Flask" {
				args = append(append([]string{}, prefix...), "-m", "flask", "--app", module+":app", "run", "--host", "127.0.0.1", "--port", "5000")
				p.Port = 5000
			}
			c := command("dev", strings.Join(args, " "), file, args...)
			c.Confidence = .8
			c.Evidence[0].Detail = "Conventional app assignment; inferred framework entrypoint"
			p.Commands = append(p.Commands, c)
			p.PortConfidence = .8
			p.PortEvidence = c.Evidence
			break
		}
	}
	if _, ok := p.DevCommand(); !ok && exists(p.Root, "main.py") {
		c := command("dev", "python main.py", "main.py", append(prefix, "main.py")...)
		c.Confidence = .6
		c.Evidence[0].Detail = "Conventional main.py script; inferred launch"
		p.Commands = append(p.Commands, c)
	}
	return nil
}
func detectGo(p *project.Project) error {
	data, err := ReadFile(p.Root, "go.mod")
	if err != nil {
		return err
	}
	p.Runtime = project.Runtime{Name: "Go", Confidence: 1, Evidence: []project.Evidence{{File: "go.mod", Detail: "Go module"}}}
	p.Manager = project.Manager{Name: "go", Confidence: 1, Evidence: p.Runtime.Evidence}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			p.Runtime.Required = ">=" + fields[1]
		}
	}
	p.Commands = append(p.Commands, command("test", "go test ./...", "go.mod", "go", "test", "./..."), command("build", "go build ./...", "go.mod", "go", "build", "./..."))
	for _, entry := range []string{"main.go", "cmd/server/main.go"} {
		if exists(p.Root, entry) {
			data, err := ReadFile(p.Root, entry)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "package main") {
				dir := "."
				if entry != "main.go" {
					dir = "./cmd/server"
				}
				c := command("dev", "go run "+dir, entry, "go", "run", dir)
				c.Confidence = .9
				c.Evidence[0].Detail = "Conventional Go main package; inferred launch"
				p.Commands = append(p.Commands, c)
				break
			}
		}
	}
	return nil
}
func detectRust(p *project.Project) error {
	data, err := ReadFile(p.Root, "Cargo.toml")
	if err != nil {
		return err
	}
	var config struct{ Package struct{ Name string } }
	if err = toml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("read Cargo.toml: %w", err)
	}
	if config.Package.Name != "" {
		p.Name = config.Package.Name
	}
	p.Runtime = project.Runtime{Name: "Rust", Confidence: 1, Evidence: []project.Evidence{{File: "Cargo.toml", Detail: "Cargo package"}}}
	p.Manager = project.Manager{Name: "cargo", Confidence: 1, Evidence: p.Runtime.Evidence}
	p.Commands = append(p.Commands, command("test", "cargo test", "Cargo.toml", "cargo", "test"), command("build", "cargo build", "Cargo.toml", "cargo", "build"))
	if exists(p.Root, "src/main.rs") {
		c := command("dev", "cargo run", "src/main.rs", "cargo", "run")
		c.Confidence = .9
		c.Evidence[0].Detail = "Conventional Rust main; inferred launch"
		p.Commands = append(p.Commands, c)
	}
	return nil
}
