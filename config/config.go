package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrConfigNotFound is returned when the kv.context file does not exist;
// ErrConfigEmpty when it exists but contains no entries.
var (
	ErrConfigNotFound = errors.New("kv.context not found")
	ErrConfigEmpty    = errors.New("kv.context is empty")
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
		return nil, fmt.Errorf("%w at %s: create this file with your contexts and namespaces", ErrConfigNotFound, path)
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
		return nil, fmt.Errorf("%w (%s): add at least one context entry", ErrConfigEmpty, path)
	}
	return &Config{Entries: entries}, nil
}
