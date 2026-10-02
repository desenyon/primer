package scripts

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstaller(t *testing.T) {
	for _, test := range []struct {
		name, os, arch, checksum, binaryVersion string
		success                                 bool
	}{
		{"darwin-arm64", "Darwin", "arm64", "valid", "v0.1.0-alpha.1", true},
		{"linux-amd64", "Linux", "x86_64", "valid", "v0.1.0-alpha.1", true},
		{"corrupt", "Darwin", "arm64", "corrupt", "v0.1.0-alpha.1", false},
		{"duplicate", "Darwin", "arm64", "duplicate", "v0.1.0-alpha.1", false},
		{"wrong-version", "Darwin", "arm64", "valid", "v0.0.0", false},
		{"unsupported-os", "Windows", "arm64", "valid", "v0.1.0-alpha.1", false},
		{"unsupported-arch", "Linux", "riscv64", "valid", "v0.1.0-alpha.1", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			tools := filepath.Join(root, "tools")
			assets := filepath.Join(root, "assets")
			target := filepath.Join(root, "installed bin")
			for _, dir := range []string{tools, assets, target} {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			write(t, filepath.Join(target, "primer"), "existing binary", 0700)
			platform := "darwin"
			if test.os == "Linux" {
				platform = "linux"
			}
			arch := "arm64"
			if test.arch == "x86_64" {
				arch = "amd64"
			}
			asset := "primer_v0.1.0-alpha.1_" + platform + "_" + arch + ".tar.gz"
			archive := archiveBinary(t, "#!/bin/sh\nprintf 'primer "+test.binaryVersion+"\\n'\n")
			if err := os.WriteFile(filepath.Join(assets, asset), archive, 0600); err != nil {
				t.Fatal(err)
			}
			hash := fmt.Sprintf("%x", sha256.Sum256(archive))
			if test.checksum == "corrupt" {
				hash = strings.Repeat("0", 64)
			}
			checksum := hash + "  " + asset + "\n"
			if test.checksum == "duplicate" {
				checksum += checksum
			}
			write(t, filepath.Join(assets, "checksums.txt"), checksum, 0600)
			write(t, filepath.Join(tools, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo "+test.os+";; -m) echo "+test.arch+";; esac\n", 0700)
			write(t, filepath.Join(tools, "curl"), `#!/bin/sh
set -eu
source=
destination=
while [ "$#" -gt 0 ]; do
  case "$1" in
    https://*) source=${1##*/} ;;
    --output) shift; destination=$1 ;;
  esac
  shift
done
cp "$PRIMER_TEST_ASSETS/$source" "$destination"
`, 0700)
			command := exec.Command("/bin/sh", "../install.sh")
			command.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "PRIMER_INSTALL_DIR="+target, "PRIMER_TEST_ASSETS="+assets, "PRIMER_VERSION=v0.1.0-alpha.1")
			output, err := command.CombinedOutput()
			if test.success && err != nil {
				t.Fatalf("installer: %v\n%s", err, output)
			}
			if !test.success && err == nil {
				t.Fatalf("unsafe install succeeded: %s", output)
			}
			installed, err := os.ReadFile(filepath.Join(target, "primer"))
			if err != nil {
				t.Fatal(err)
			}
			if test.success {
				if !strings.Contains(string(installed), test.binaryVersion) {
					t.Fatal("binary was not installed")
				}
			} else if string(installed) != "existing binary" {
				t.Fatal("failed download replaced the existing binary")
			}
			entries, err := os.ReadDir(target)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatal("installer left staging files")
			}
		})
	}
}

func TestInstallerDefaultVersionMatchesRelease(t *testing.T) {
	version, err := os.ReadFile("../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	installer, err := os.ReadFile("../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installer), "${PRIMER_VERSION:-v"+strings.TrimSpace(string(version))+"}") {
		t.Fatal("installer default does not match VERSION")
	}
}

func archiveBinary(t *testing.T, text string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	zip := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(zip)
	if err := archive.WriteHeader(&tar.Header{Name: "primer", Mode: 0755, Size: int64(len(text))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func write(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}
