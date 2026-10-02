package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestHeadlessInspectionNeverLaunches(t *testing.T) {
	var output, errOutput bytes.Buffer
	code := run(context.Background(), []string{"start", "--path", "../../fixtures/next-basic", "--json"}, strings.NewReader(""), &output, &errOutput, false)
	if code != 1 {
		t.Fatalf("broken fixture should return 1, got %d: %s", code, errOutput.String())
	}
	var report map[string]any
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("not JSON: %v: %s", err, output.String())
	}
	if strings.Contains(output.String(), "APPROVED LAUNCH") {
		t.Fatal("headless inspection launched")
	}
}

func TestInvalidCLIInputs(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"doctor", "--yes"}, {"start", "--json", "--yes"}, {"doctor", "extra"}} {
		var output, errOutput bytes.Buffer
		if code := run(context.Background(), args, strings.NewReader(""), &output, &errOutput, false); code != 2 {
			t.Fatalf("args %v returned %d", args, code)
		}
	}
}

func TestVersionDoesNotInspectRepository(t *testing.T) {
	var output, errOutput bytes.Buffer
	code := run(context.Background(), []string{"--version", "--path", "/nonexistent-primer-path"}, strings.NewReader(""), &output, &errOutput, false)
	if code != 0 || output.String() != "primer "+version+"\n" || errOutput.Len() != 0 {
		t.Fatalf("version failed: %d %q %q", code, output.String(), errOutput.String())
	}
}

func TestJSONRedactionPreservesStructure(t *testing.T) {
	var output bytes.Buffer
	secret := "quoted\"\ncredential"
	err := writeJSON(&output, map[string]any{"command": "echo " + secret, "count": 123, "secret": secret}, func(text string) string { return strings.ReplaceAll(text, secret, "[redacted]") })
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["count"] != float64(123) || result["secret"] != "[redacted]" || result["command"] != "echo [redacted]" {
		t.Fatalf("JSON redaction: %s", output.String())
	}
}

func TestCommandsAndRunReview(t *testing.T) {
	var output, errs bytes.Buffer
	if code := run(context.Background(), []string{"commands", "--path", "../../fixtures/go-service", "--json"}, strings.NewReader(""), &output, &errs, false); code != 0 {
		t.Fatalf("%d %s", code, errs.String())
	}
	if !strings.Contains(output.String(), "go test") {
		t.Fatal(output.String())
	}
	output.Reset()
	errs.Reset()
	if code := run(context.Background(), []string{"run", "not-declared", "--path", "../../fixtures/go-service"}, strings.NewReader(""), &output, &errs, false); code != 2 {
		t.Fatalf("unknown command accepted: %d", code)
	}
	output.Reset()
	errs.Reset()
	run(context.Background(), []string{"run", "test", "--path", "../../fixtures/go-service"}, strings.NewReader(""), &output, &errs, false)
	if strings.Contains(output.String(), "APPROVED LAUNCH") {
		t.Fatal("unapproved execution")
	}
}
