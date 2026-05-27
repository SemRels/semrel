// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package envfile loads environment variables from a .env file.
// Only variables not already set in the process environment are applied
// (existing env vars always win).
//
// Supported syntax:
//
//	KEY=value            plain value
//	KEY="quoted value"   double-quoted (backslash escapes supported)
//	KEY='literal value'  single-quoted (no escapes)
//	# comment line       ignored
//	                     blank lines are ignored
package envfile

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Load reads the file at path and sets any variable not already present in the
// process environment. It is safe to call with a path that does not exist —
// in that case, Load is a no-op and returns nil.
func Load(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("envfile: open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	pairs, err := parse(f)
	if err != nil {
		return fmt.Errorf("envfile: parse %s: %w", path, err)
	}

	for k, v := range pairs {
		// Don't override variables already in the environment.
		if _, exists := os.LookupEnv(k); !exists {
			if setErr := os.Setenv(k, v); setErr != nil {
				return fmt.Errorf("envfile: setenv %s: %w", k, setErr)
			}
		}
	}
	return nil
}

// Parse reads environment variable pairs from r without setting them.
// Useful for inspection or testing.
func Parse(r io.Reader) (map[string]string, error) {
	return parse(r)
}

func parse(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// parseLine splits "KEY=VALUE" handling quoted values.
func parseLine(line string) (key, value string, err error) {
	eq := strings.IndexByte(line, '=')
	if eq < 0 {
		return "", "", fmt.Errorf("invalid line %q: missing '='", line)
	}

	key = strings.TrimSpace(line[:eq])
	if key == "" {
		return "", "", fmt.Errorf("empty key in line %q", line)
	}
	// Keys must be valid env var names.
	for _, ch := range key {
		if !isKeyChar(ch) {
			return "", "", fmt.Errorf("invalid character %q in key %q", ch, key)
		}
	}

	raw := strings.TrimSpace(line[eq+1:])
	value, err = unquote(raw)
	return key, value, err
}

func unquote(s string) (string, error) {
	if len(s) == 0 {
		return "", nil
	}

	switch s[0] {
	case '"':
		// Double-quoted: support \n \t \\ \"
		if len(s) < 2 || s[len(s)-1] != '"' {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		inner := s[1 : len(s)-1]
		var b strings.Builder
		i := 0
		for i < len(inner) {
			if inner[i] == '\\' && i+1 < len(inner) {
				i++
				switch inner[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case '"':
					b.WriteByte('"')
				case '\\':
					b.WriteByte('\\')
				default:
					b.WriteByte('\\')
					b.WriteByte(inner[i])
				}
			} else {
				b.WriteByte(inner[i])
			}
			i++
		}
		return b.String(), nil

	case '\'':
		// Single-quoted: no escape processing.
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		return s[1 : len(s)-1], nil

	default:
		// Unquoted: strip inline comment.
		if idx := strings.IndexByte(s, '#'); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
		return s, nil
	}
}

func isKeyChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') || r == '_'
}
