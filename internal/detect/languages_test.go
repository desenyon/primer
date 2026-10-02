package detect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLanguageFixtures(t *testing.T) {
	for _, tc := range []struct{ name, runtime, framework, launch string }{{"python-uv", "Python", "FastAPI", "uvicorn"}, {"python-requirements", "Python", "", "main.py"}, {"go-service", "Go", "", "go run"}, {"rust-service", "Rust", "", "cargo run"}, {"typescript-vite", "Node", "Vite", "vite"}, {"procfile-service", "", "", "make"}} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Scan(context.Background(), "../../fixtures/"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			dev, ok := p.DevCommand()
			if !ok || p.Runtime.Name != tc.runtime || p.Framework != tc.framework || !strings.Contains(dev.Script, tc.launch) {
				t.Fatalf("derived %+v", p)
			}
			if len(dev.Evidence) == 0 || dev.Confidence <= 0 || p.Fingerprint == "" {
				t.Fatal("missing evidence")
			}
		})
	}
}
func TestPythonDeclarationsAndManager(t *testing.T) {
	root := t.TempDir()
	for name, data := range map[string]string{"pyproject.toml": "[project]\nname='declared'\nrequires-python='>=3.11,<4'\n[project.scripts]\ndev='pkg:main'\ntest='pkg:test'\nmy-cli='pkg:main'\n", "uv.lock": "version = 1\n", ".python-version": "3.12"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Command("my-cli"); !ok {
		t.Fatal("hyphenated project script not discovered")
	}
	c, _ := p.DevCommand()
	if p.Manager.Name != "uv" || strings.Join(c.Args, " ") != "uv run --no-sync --no-python-downloads dev" || p.Runtime.Pins[0] != "3.12" {
		t.Fatalf("derived %+v", p)
	}
}
func TestInspectionRejectsSymlinkedEntrypointDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	os.WriteFile(filepath.Join(root, "pyproject.toml"), []byte("[project]\nname='test'\n"), 0600)
	if err := os.Symlink(outside, filepath.Join(root, "app")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(outside, "main.py"), []byte("private"), 0600)
	if _, err := Scan(context.Background(), root); err == nil {
		t.Fatal("followed symlinked directory")
	}
}
func TestGeneralCommandDiscoveryNeverExecutes(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "executed")
	os.WriteFile(filepath.Join(root, "Procfile"), []byte("web: touch "+marker+"\n"), 0600)
	p, err := Scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := p.DevCommand()
	if c.Args[0] != "sh" || c.Evidence[0].Line != 1 {
		t.Fatalf("derived %+v", p)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inspection executed code")
	}
}
