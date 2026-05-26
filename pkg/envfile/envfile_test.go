// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_BasicKeyValue(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("FOO=bar\nBAZ=qux\n")
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "bar", pairs["FOO"])
	assert.Equal(t, "qux", pairs["BAZ"])
}

func TestParse_DoubleQuoted(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`KEY="hello world"`)
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "hello world", pairs["KEY"])
}

func TestParse_DoubleQuotedEscapes(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`KEY="line1\nline2\ttab"`)
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "line1\nline2\ttab", pairs["KEY"])
}

func TestParse_SingleQuoted(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`KEY='literal \n no escape'`)
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, `literal \n no escape`, pairs["KEY"])
}

func TestParse_Comments(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("# comment\nFOO=bar # inline comment\n")
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "bar", pairs["FOO"])
}

func TestParse_BlankLines(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("\n\nFOO=bar\n\n")
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "bar", pairs["FOO"])
}

func TestParse_EmptyValue(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("EMPTY=\n")
	pairs, err := Parse(r)
	require.NoError(t, err)
	assert.Equal(t, "", pairs["EMPTY"])
}

func TestParse_MissingEquals(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("NOEQUALSSIGN\n")
	_, err := Parse(r)
	require.Error(t, err)
}

func TestParse_EmptyKey(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("=value\n")
	_, err := Parse(r)
	require.Error(t, err)
}

func TestParse_UnterminatedDoubleQuote(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`KEY="unterminated`)
	_, err := Parse(r)
	require.Error(t, err)
}

func TestParse_UnterminatedSingleQuote(t *testing.T) {
	t.Parallel()
	r := strings.NewReader(`KEY='unterminated`)
	_, err := Parse(r)
	require.Error(t, err)
}

func TestLoad_FileNotExist(t *testing.T) {
	t.Parallel()
	// Should not error when file does not exist.
	err := Load("/tmp/nonexistent-semrel-test-9999.env")
	require.NoError(t, err)
}

func TestLoad_SetsVarsFromFile(t *testing.T) {
	t.Setenv("SEMREL_TEST_LOAD_EXISTING", "original")

	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "SEMREL_TEST_LOAD_NEW=newval\nSEMREL_TEST_LOAD_EXISTING=overridden\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	err := Load(path)
	require.NoError(t, err)

	// New var should be set.
	assert.Equal(t, "newval", os.Getenv("SEMREL_TEST_LOAD_NEW"))
	// Existing var must NOT be overridden.
	assert.Equal(t, "original", os.Getenv("SEMREL_TEST_LOAD_EXISTING"))

	// Clean up new var.
	os.Unsetenv("SEMREL_TEST_LOAD_NEW")
}

func TestLoad_InvalidFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(path, []byte("BADLINE\n"), 0o600))
	err := Load(path)
	require.Error(t, err)
}
