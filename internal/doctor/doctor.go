// Package doctor checks the local system without running repository commands.
package doctor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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
	if p.Runtime.Name == "" {
		return r
	}
	for _, name := range []string{"node", p.Manager.Name} {
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
			if name == "node" {
				requirements = append([]string{p.Runtime.Required}, p.Runtime.Pins...)
				evidence = p.Runtime.Evidence
			}
			for _, requirement := range requirements {
				if requirement == "" {
					continue
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
	entries, err := os.ReadDir(filepath.Join(p.Root, "node_modules"))
	if err != nil || len(entries) == 0 {
		r.Diagnostics = append(r.Diagnostics, project.Diagnostic{ID: "dependencies", Summary: "Node dependencies are missing", Expected: "A populated node_modules directory", Repair: "Review and run the package install command. Install scripts execute repository and dependency code.", Blocking: true, Evidence: []project.Evidence{{File: "package.json", Detail: "Node package dependencies"}}})
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
	cmd := exec.CommandContext(checkCtx, path, "--version")
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
	version = strings.TrimPrefix(version, "v")
	if _, err := semver.NewVersion(version); err != nil {
		return "", fmt.Errorf("%s returned an unrecognized version", name)
	}
	return version, nil
}
