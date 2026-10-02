package detect

import (
	"bufio"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/desenyon/primer/internal/project"
)

var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type envEntry struct {
	value string
	line  int
}

func parseEnv(data []byte) map[string]envEntry {
	entries := map[string]envEntry{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), maxFileSize)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(scanner.Text()), "export "))
		key, value, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKey.MatchString(key) {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		entries[key] = envEntry{value: value, line: line}
	}
	return entries
}

func detectEnvironment(p *project.Project) error {
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if value != "" && (strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.HasSuffix(upper, "_KEY") || strings.HasSuffix(upper, "DATABASE_URL")) {
			p.Secrets = append(p.Secrets, value)
		}
	}
	template := map[string][]project.Evidence{}
	for _, name := range []string{".env.example", ".env.sample", ".env.template"} {
		data, err := ReadFile(p.Root, name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for key, entry := range parseEnv(data) {
			template[key] = append(template[key], project.Evidence{File: name, Line: entry.line, Detail: "Environment template entry"})
		}
	}
	values := map[string]envEntry{}
	sources := map[string]string{}
	for _, name := range []string{".env", ".env.local"} {
		data, err := ReadFile(p.Root, name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for key, entry := range parseEnv(data) {
			values[key] = entry
			sources[key] = name
			if entry.value != "" && !strings.HasPrefix(key, "NEXT_PUBLIC_") && !strings.HasPrefix(key, "VITE_") {
				p.Secrets = append(p.Secrets, entry.value)
			}
		}
	}
	keys := make([]string, 0, len(template))
	for key := range template {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		state := "missing"
		evidence := template[key]
		entry, ok := values[key]
		if inherited, exists := os.LookupEnv(key); exists {
			entry.value = inherited
			ok = true
			if inherited != "" && !strings.HasPrefix(key, "NEXT_PUBLIC_") && !strings.HasPrefix(key, "VITE_") {
				p.Secrets = append(p.Secrets, inherited)
			}
			evidence = append(evidence, project.Evidence{File: "process environment", Detail: "Inherited variable; value hidden"})
		} else if ok {
			evidence = append(evidence, project.Evidence{File: sources[key], Line: entry.line, Detail: "Local variable; value hidden"})
		}
		if ok {
			state = "configured"
			if entry.value == "" {
				state = "empty"
			}
		}
		p.Environment = append(p.Environment, project.EnvironmentVariable{Confidence: 1, Name: key, State: state, Evidence: evidence})
		if state != "configured" {
			p.Diagnostics = append(p.Diagnostics, project.Diagnostic{ID: "env-" + key, Summary: key + " is " + state, Expected: "A configured local value", Repair: "Set the variable in the local environment. Template entries may include optional variables; review before launch.", Evidence: evidence})
		}
	}
	return nil
}
