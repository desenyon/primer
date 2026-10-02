//go:build darwin || linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchRejectsChangedScriptAfterReview(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	for name, version := range map[string]string{"node": "v22.4.1", "npm": "10.1.0"} {
		if err := os.WriteFile(filepath.Join(tools, name), []byte("#!/bin/sh\necho "+version+"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools)
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"scripts":{"dev":"echo reviewed"}}`)
	if err := os.Mkdir(filepath.Join(root, "node_modules"), 0700); err != nil {
		t.Fatal(err)
	}
	write("node_modules/marker", "fixture")
	session := New(root)
	defer session.Stop()
	r, err := session.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.Blocked() {
		t.Fatalf("unexpected checks: %+v", r.Diagnostics)
	}
	write("package.json", `{"scripts":{"dev":"echo changed"}}`)
	if err := session.Start(context.Background()); err == nil || !strings.Contains(err.Error(), "changed after review") {
		t.Fatalf("got %v", err)
	}
	if session.Snapshot().PID != 0 {
		t.Fatal("changed command started")
	}
}

func TestCancelledLaunchCannotStart(t *testing.T) {
	session := New("../../fixtures/next-basic")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Inspect(ctx); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
	if err := session.Start(ctx); err == nil {
		t.Fatal("cancelled launch accepted")
	}
	if session.Snapshot().PID != 0 {
		t.Fatal("cancelled launch started")
	}
}
