//go:build darwin || linux

package doctor

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/desenyon/primer/internal/detect"
)

func TestDoctorChecksRuntimeDependenciesAndPort(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	write(t, root, "package.json", `{"name":"checks","engines":{"node":">=22"},"scripts":{"dev":"next dev"},"dependencies":{"next":"16"}}`)
	write(t, root, ".nvmrc", "24.1.0\n")
	writeExecutable(t, tools, "node", "#!/bin/sh\necho v22.4.1\n")
	writeExecutable(t, tools, "npm", "#!/bin/sh\necho 10.9.0\n")
	t.Setenv("PATH", tools)
	p, err := detect.Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	p.Port = listener.Addr().(*net.TCPAddr).Port
	report := Check(context.Background(), p)
	ids := map[string]bool{}
	for _, d := range report.Diagnostics {
		ids[d.ID] = true
	}
	for _, id := range []string{"version-node", "dependencies", "port"} {
		if !ids[id] {
			t.Fatalf("missing %s: %+v", id, report)
		}
	}
}

func TestDoctorRefusesRepositoryLocalTool(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"dev":"echo unsafe"}}`)
	writeExecutable(t, root, "node", fmt.Sprintf("#!/bin/sh\ntouch '%s'\necho v22.0.0\n", filepath.Join(root, "executed")))
	t.Setenv("PATH", root)
	if _, err := toolVersion(context.Background(), root, "node"); err == nil {
		t.Fatal("accepted repository tool")
	}
	if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
		t.Fatal("ran repository code during doctor")
	}
}

func write(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func writeExecutable(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0700); err != nil {
		t.Fatal(err)
	}
}
