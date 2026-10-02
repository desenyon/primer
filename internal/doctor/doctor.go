// Package doctor checks the local system without running repository commands.
package doctor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/desenyon/primer/internal/project"
)

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Ready   bool   `json:"ready"`
}

type Report struct {
	Project     project.Project      `json:"project"`
	Tools       []Tool               `json:"tools"`
	Diagnostics []project.Diagnostic `json:"diagnostics"`
}

func Check(ctx context.Context, p project.Project) Report {
	r := Report{Project: p, Tools: []Tool{}, Diagnostics: append([]project.Diagnostic{}, p.Diagnostics...)}
	runtimeTool := map[string]string{"Node": "node", "Python": "python3", "Go": "go", "Rust": "rustc"}[p.Runtime.Name]
	tools := []string{runtimeTool}
	if p.Manager.Name != "pip" && p.Manager.Name != runtimeTool {
		tools = append(tools, p.Manager.Name)
	}
	for _, name := range tools {
		if name == "" {
			continue
		}
		version, err := toolVersion(ctx, p.Root, name)
		t := Tool{Name: name, Version: version, Ready: err == nil}
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "tool-" + name, Summary: name + " is unavailable", Found: err.Error(), Repair: "Install or activate the required tool using your existing runtime manager.", Blocking: true})
		} else {
			requirements := []string{p.Manager.Version}
			evidence := p.Manager.Evidence
			if name == runtimeTool {
				requirements = append([]string{p.Runtime.Required}, p.Runtime.Pins...)
				evidence = p.Runtime.Evidence
			}
			for _, requirement := range requirements {
				if requirement == "" {
					continue
				}
				if p.Runtime.Name == "Python" {
					requirement = strings.ReplaceAll(requirement, "==", "=")
				}
				if regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`).MatchString(requirement) {
					requirement += ".x"
				}
				constraint, parseErr := semver.NewConstraint(requirement)
				actual, versionErr := semver.NewVersion(version)
				if parseErr != nil || versionErr != nil {
					t.Ready = false
					r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "version-" + name, Summary: name + " version requirement cannot be verified", Expected: requirement, Found: version, Repair: "Resolve the version alias or use a numeric version requirement.", Blocking: true, Evidence: evidence})
					break
				}
				if !constraint.Check(actual) {
					t.Ready = false
					r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "version-" + name, Summary: name + " version does not match", Expected: requirement, Found: version, Repair: "Activate the required version before launching.", Blocking: true, Evidence: evidence})
					break
				}
			}
		}
		r.Tools = append(r.Tools, t)
	}
	if p.Runtime.Name == "Node" && p.DependenciesDeclared {
		entries, err := os.ReadDir(filepath.Join(p.Root, "node_modules"))
		if err != nil || len(entries) == 0 {
			r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "dependencies", Summary: "Node dependencies are missing", Expected: "A populated node_modules directory", Repair: "Review and run the package install command. Install scripts execute repository and dependency code.", Blocking: true, Evidence: p.Runtime.Evidence})
		}
	}
	if p.Runtime.Name == "Python" && (p.Manager.Name == "uv" || p.Manager.Name == "poetry" || fileExists(p.Root, "requirements.txt") || fileExists(p.Root, "pyproject.toml")) {
		if !fileExists(p.Root, ".venv/bin/python") && p.Manager.Name != "poetry" {
			r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "dependencies", Summary: "Python environment is missing", Expected: "A project-local .venv", Repair: "Review uv sync, or python3 -m venv .venv followed by .venv/bin/python -m pip install -r requirements.txt (or -e . for pyproject projects). Installation executes dependency code.", Blocking: true, Evidence: p.Runtime.Evidence})
		}
	}
	if p.Port > 0 {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p.Port))
		if err != nil {
			r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "port", Summary: fmt.Sprintf(":%d is unavailable", p.Port), Expected: "An unused local port", Found: "Local bind failed; another process or system policy prevents use", Repair: "Stop the owning process yourself or change the project's dev port. Primer will not kill unrelated processes.", Blocking: true, Evidence: p.PortEvidence})
		} else {
			listener.Close()
		}
	}
	return r
}

func (r Report) Blocked() bool {
	for _, d := range r.Diagnostics {
		if d.Blocking {
			return true
		}
	}
	return false
}

func toolVersion(ctx context.Context, root, name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH", name)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		return "", fmt.Errorf("refusing repository-local %s during inspection", name)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	args := []string{"--version"}
	if name == "go" {
		args = []string{"version"}
	}
	cmd := exec.CommandContext(checkCtx, path, args...)
	cmd.WaitDelay = 250 * time.Millisecond
	// Package manager configuration in the repository is untrusted.
	cmd.Dir = os.TempDir()
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s version check failed", name)
	}
	version := strings.TrimSpace(string(output))
	if len(version) > 128 || strings.ContainsAny(version, "\n\r\x1b") {
		return "", fmt.Errorf("%s returned an invalid version", name)
	}
	switch name {
	case "python3":
		version = strings.TrimPrefix(version, "Python ")
	case "go":
		fields := strings.Fields(version)
		if len(fields) >= 3 {
			version = strings.TrimPrefix(fields[2], "go")
		}
	case "rustc", "cargo", "uv", "poetry":
		fields := strings.Fields(version)
		if len(fields) >= 2 {
			version = fields[1]
		}
		if name == "poetry" && len(fields) >= 3 {
			version = strings.TrimSuffix(fields[2], ")")
		}
	}
	version = strings.TrimPrefix(version, "v")
	if _, err := semver.NewVersion(version); err != nil {
		return "", fmt.Errorf("%s returned an unrecognized version", name)
	}
	return version, nil
}

func fileExists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

// A library need not declare a dev process to run its declared test/build commands.
func (r Report) ForCommand(name string) Report {
	if name == "dev" {
		return r
	}
	filtered := make([]project.Diagnostic, 0, len(r.Diagnostics))
	for _, d := range r.Diagnostics {
		if d.ID != "missing-dev-command" && d.ID != "port" {
			filtered = append(filtered, d)
		}
	}
	r.Diagnostics = filtered
	return r
}
