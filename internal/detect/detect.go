// Package detect derives a project model from bounded, read-only file inspection.
// It never runs repository code, package scripts, or tools.
package detect

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/desenyon/primer/internal/project"
)

const maxFileSize = 1 << 20

// Locate chooses the nearest project marker before a Git boundary. This also
// works for a package inside a monorepo and avoids scanning parent directories.
func Locate(start string) (string, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", root)
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		for _, name := range []string{"package.json", "go.mod", "pyproject.toml", "Cargo.toml", "requirements.txt", "Procfile", "Makefile", ".git"} {
			if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
				return dir, nil
			}
		}
		if filepath.Dir(dir) == dir {
			return root, nil
		}
	}
}

// ReadFile rejects symlinks: cloned repositories cannot redirect inspection to
// secrets outside the repository or special devices.
func ReadFile(root, name string) ([]byte, error) {
	for dir := filepath.Dir(name); dir != "."; dir = filepath.Dir(dir) {
		info, err := os.Lstat(filepath.Join(root, dir))
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%s is not a repository directory", dir)
		}
	}
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("%s exceeds the 1 MiB inspection limit", name)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("%s exceeds the inspection limit", name)
	}
	return data, err
}

type packageJSON struct {
	Name            string            `json:"name"`
	PackageManager  string            `json:"packageManager"`
	Engines         map[string]string `json:"engines"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func scanNode(ctx context.Context, start string) (project.Project, error) {
	root, err := Locate(start)
	if err != nil {
		return project.Project{}, err
	}
	p := project.Project{Root: root, Name: filepath.Base(root), Commands: []project.Command{}, Environment: []project.EnvironmentVariable{}, Diagnostics: []project.Diagnostic{}}
	if err := ctx.Err(); err != nil {
		return p, err
	}
	data, err := ReadFile(root, "package.json")
	if errors.Is(err, os.ErrNotExist) {
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "unsupported-project", Summary: "No Node package found", Expected: "package.json", Found: root, Repair: "Run Primer from a Node package directory. Other runtimes are not supported in this milestone.", Blocking: true})
		return p, nil
	}
	if err != nil {
		return p, err
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return p, fmt.Errorf("read package.json: %w", err)
	}
	if pkg.Name != "" {
		p.Name = pkg.Name
	}
	p.DependenciesDeclared = len(pkg.Dependencies) > 0 || len(pkg.DevDependencies) > 0
	p.Runtime = project.Runtime{Confidence: 1, Name: "Node", Required: pkg.Engines["node"], Evidence: []project.Evidence{{File: "package.json", Detail: "Node package manifest"}}}
	if pkg.Engines["node"] != "" {
		p.Runtime.Evidence = append(p.Runtime.Evidence, project.Evidence{File: "package.json", Detail: "engines.node"})
	}
	for _, name := range []string{".nvmrc", ".node-version"} {
		data, err := ReadFile(root, name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return p, err
		}
		version := strings.TrimSpace(string(data))
		if version == "" {
			continue
		}
		if p.Runtime.Required == "" {
			p.Runtime.Required = version
		}
		p.Runtime.Pins = append(p.Runtime.Pins, version)
		p.Runtime.Evidence = append(p.Runtime.Evidence, project.Evidence{File: name, Line: 1, Detail: "Node version pin: " + version})
		// Separate pins are checked individually by doctor.
	}
	p.Manager, p.Diagnostics = detectManager(root, pkg.PackageManager)
	if pkg.Dependencies["next"] != "" || pkg.DevDependencies["next"] != "" {
		p.Framework = "Next.js"
		p.FrameworkConfidence = 1
		p.FrameworkEvidence = []project.Evidence{{File: "package.json", Detail: "next dependency"}}
	}
	if p.Framework == "" {
		for _, framework := range []struct{ dependency, name string }{{"vite", "Vite"}, {"astro", "Astro"}, {"nuxt", "Nuxt"}, {"typescript", "TypeScript"}} {
			if pkg.Dependencies[framework.dependency] != "" || pkg.DevDependencies[framework.dependency] != "" {
				p.Framework, p.FrameworkConfidence = framework.name, 1
				p.FrameworkEvidence = []project.Evidence{{File: "package.json", Detail: framework.dependency + " dependency"}}
				break
			}
		}
	}

	names := make([]string, 0, len(pkg.Scripts))
	for name := range pkg.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if name == "" || strings.HasPrefix(name, "-") {
			continue
		}
		p.Commands = append(p.Commands, project.Command{Confidence: 1, Name: name, Script: pkg.Scripts[name], Args: []string{p.Manager.Name, "run", name}, Evidence: []project.Evidence{{File: "package.json", Detail: "scripts." + name}}})
	}
	if _, ok := p.DevCommand(); !ok && pkg.Scripts["start"] != "" {
		for _, c := range p.Commands {
			if c.Name == "start" {
				c.Name = "dev"
				p.Commands = append(p.Commands, c)
				break
			}
		}
	}
	if dev, ok := p.DevCommand(); ok {
		if p.Framework == "Vite" && strings.HasPrefix(dev.Script, "vite") {
			p.Port, p.PortConfidence, p.PortEvidence = 5173, .8, dev.Evidence
			if match := explicitPort.FindStringSubmatch(dev.Script); len(match) > 1 {
				p.Port, _ = strconv.Atoi(match[1])
				p.PortConfidence = 1
			}
		}
		if p.Framework == "Next.js" && nextDev.MatchString(dev.Script) {
			p.Port = 3000
			p.PortConfidence = 0.8
			p.PortEvidence = dev.Evidence
			if port := os.Getenv("PORT"); port != "" {
				p.PortConfidence = 1
				p.Port, _ = strconv.Atoi(port)
				p.PortEvidence = append(p.PortEvidence, project.Evidence{File: "process environment", Detail: "PORT override"})
			}
			if match := nextDev.FindStringSubmatch(dev.Script); len(match) > 1 && match[1] != "" {
				p.PortConfidence = 1
				p.Port, _ = strconv.Atoi(match[1])
			}
			if match := explicitPort.FindStringSubmatch(dev.Script); len(match) > 1 {
				p.PortConfidence = 1
				p.Port, _ = strconv.Atoi(match[1])
			}
			if p.Port < 1 || p.Port > 65535 {
				p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "invalid-port", Summary: "Next.js port is invalid", Blocking: true, Evidence: p.PortEvidence})
				p.Port = 0
				p.PortConfidence = 0
			}
		}
	} else {
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "missing-dev-command", Summary: "No dev command found", Expected: "scripts.dev in package.json", Repair: "Define the project's dev script before launching.", Blocking: true, Evidence: []project.Evidence{{File: "package.json", Detail: "scripts"}}})
	}
	if p.Port < 0 || p.Port > 65535 {
		p.Port = 0
		p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "invalid-port", Summary: "Dev script declares an invalid port", Blocking: true, Evidence: p.PortEvidence})
	}
	if err := detectEnvironment(&p); err != nil {
		return p, err
	}
	return p, ctx.Err()
}

var nextDev = regexp.MustCompile(`^(?:cross-env[ \t]+)?(?:PORT=([0-9]+)[ \t]+)?next[ \t]+dev(?:[ \t][^;&|\r\n]*)?$`)
var explicitPort = regexp.MustCompile(`(?:--port(?:=|\s+)|-p\s+)([0-9]+)`)

func detectManager(root, declared string) (project.Manager, []project.Diagnostic) {
	manager := project.Manager{Confidence: 1}
	var diagnostics []project.Diagnostic
	if declared != "" {
		parts := strings.SplitN(declared, "@", 2)
		manager.Name = parts[0]
		if len(parts) == 2 {
			manager.Version = strings.SplitN(parts[1], "+", 2)[0]
		}
		manager.Evidence = []project.Evidence{{File: "package.json", Detail: "packageManager: " + declared}}
		if !supportedManager(manager.Name) {
			diagnostics = append(diagnostics, project.Diagnostic{ID: "unsupported-manager", Summary: "Unsupported package manager", Found: manager.Name, Blocking: true, Evidence: manager.Evidence})
			manager.Name = ""
		}
	}
	locks := []struct{ file, manager string }{{"pnpm-lock.yaml", "pnpm"}, {"package-lock.json", "npm"}, {"yarn.lock", "yarn"}, {"bun.lock", "bun"}, {"bun.lockb", "bun"}}
	found := map[string]bool{}
	for _, lock := range locks {
		if _, err := os.Lstat(filepath.Join(root, lock.file)); err == nil {
			found[lock.manager] = true
			manager.Evidence = append(manager.Evidence, project.Evidence{File: lock.file, Detail: lock.manager + " lockfile"})
		}
	}
	if len(found) > 1 || (manager.Name != "" && len(found) == 1 && !found[manager.Name]) {
		manager.Confidence = 0
		diagnostics = append(diagnostics, project.Diagnostic{ID: "ambiguous-manager", Summary: "Package manager evidence conflicts", Repair: "Keep the intended lockfile and align packageManager before launching.", Blocking: true, Evidence: manager.Evidence})
	}
	if declared == "" {
		if len(found) == 1 {
			for name := range found {
				manager.Name = name
			}
		} else if len(found) == 0 {
			manager.Name = "npm"
			manager.Confidence = 0.5
			manager.Evidence = []project.Evidence{{File: "package.json", Detail: "No manager or lockfile declared; npm default"}}
		}
	}
	return manager, diagnostics
}

func supportedManager(name string) bool {
	return name == "npm" || name == "pnpm" || name == "yarn" || name == "bun"
}
