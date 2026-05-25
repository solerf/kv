package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Entry struct {
	Context   string
	Namespace string
}

type Config struct {
	Entries []Entry
}

func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not determine home directory: %w", err)
		}
		path = filepath.Join(home, "kv.context")
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("kv.context not found at %s: create this file with your contexts and namespaces", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening kv.context: %w", err)
	}
	defer f.Close()

	var entries []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		context, namespace, _ := strings.Cut(line, ":")
		entry := Entry{Context: context, Namespace: namespace}
		entries = append(entries, entry)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading kv.context: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("kv.context at %s is empty: add at least one context entry", path)
	}
	return &Config{Entries: entries}, nil
}
