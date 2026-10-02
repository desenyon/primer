package detect

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextDetection(t *testing.T) {
	p, err := Scan(context.Background(), "../../fixtures/next-basic")
	if err != nil {
		t.Fatal(err)
	}
	if p.Framework != "Next.js" || p.Runtime.Required != ">=22" || p.Manager.Name != "pnpm" || p.Manager.Version != "11.4.0" || p.Port != 3000 {
		t.Fatalf("unexpected model: %+v", p)
	}
	command, ok := p.DevCommand()
	if !ok || strings.Join(command.Args, " ") != "pnpm run dev" || len(command.Evidence) == 0 {
		t.Fatalf("unexpected dev command: %+v", command)
	}
}

func TestConflictingManagerBlocksLaunch(t *testing.T) {
	p, err := Scan(context.Background(), "../../fixtures/ambiguous-manager")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Blocked() {
		t.Fatal("conflicting manager must block launch")
	}
	if len(p.Diagnostics) != 1 || p.Diagnostics[0].ID != "ambiguous-manager" {
		t.Fatalf("unexpected diagnostics: %+v", p.Diagnostics)
	}
}

func TestInspectionNeverRunsScripts(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"dev":"touch executed","predev":"touch executed"}}`)
	if _, err := Scan(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
		t.Fatal("inspection ran a repository script")
	}
}

func TestEnvironmentDoesNotSerializeSecrets(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{"scripts":{"dev":"node server.js"}}`)
	write(t, root, ".env.example", "PRIMER_TEST_TOKEN=\nPRIMER_TEST_EMPTY=\nPRIMER_TEST_ABSENT=\n")
	write(t, root, ".env", "PRIMER_TEST_TOKEN=older-credential\nPRIMER_TEST_EMPTY=\n")
	write(t, root, ".env.local", "PRIMER_TEST_TOKEN=local-credential\n")
	t.Setenv("PRIMER_TEST_TOKEN", "inherited-credential")
	p, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, variable := range p.Environment {
		states[variable.Name] = variable.State
	}
	if states["PRIMER_TEST_TOKEN"] != "configured" || states["PRIMER_TEST_EMPTY"] != "empty" || states["PRIMER_TEST_ABSENT"] != "missing" {
		t.Fatalf("states: %v", states)
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"older-credential", "local-credential", "inherited-credential"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("JSON leaked %s", secret)
		}
	}
	for _, secret := range []string{"older-credential", "local-credential", "inherited-credential"} {
		found := false
		for _, value := range p.Secrets {
			if value == secret {
				found = true
			}
		}
		if !found {
			t.Fatal("missing shadowed secret for redaction")
		}
	}
}

func TestBoundedInspectionAndSymlinkRejection(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, outside, "private", "credential")
	if err := os.Symlink(filepath.Join(outside, "private"), filepath.Join(root, ".env")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, ".env"); err == nil {
		t.Fatal("read a symlink")
	}
	write(t, root, "large", strings.Repeat("x", maxFileSize+1))
	if _, err := ReadFile(root, "large"); err == nil {
		t.Fatal("read an oversized file")
	}
}

func TestLocateNearestPackage(t *testing.T) {
	root := t.TempDir()
	write(t, root, "package.json", `{}`)
	nested := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(filepath.Join(nested, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	write(t, nested, "package.json", `{}`)
	got, err := Locate(filepath.Join(nested, "src"))
	if err != nil {
		t.Fatal(err)
	}
	if got != nested {
		t.Fatalf("got %s; want %s", got, nested)
	}
}

func TestCancelledScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, "../../fixtures/next-basic"); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}

func TestNextPortEvidencePrecedence(t *testing.T) {
	for _, test := range []struct {
		script, env string
		port        int
	}{{"next dev", "", 3000}, {"next dev", "4001", 4001}, {"PORT=4002 next dev", "4001", 4002}, {"next dev --port 4003", "4001", 4003}, {"cross-env PORT=4002 next dev -p 4003", "4001", 4003}, {"echo hello; next dev", "", 0}} {
		root := t.TempDir()
		data, _ := json.Marshal(map[string]any{"scripts": map[string]string{"dev": test.script}, "dependencies": map[string]string{"next": "16"}})
		write(t, root, "package.json", string(data))
		t.Setenv("PORT", test.env)
		p, err := Scan(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		if p.Port != test.port {
			t.Fatalf("%q PORT=%q: %d, want %d", test.script, test.env, p.Port, test.port)
		}
	}
}

func write(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
