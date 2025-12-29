package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has correct use
	assert.Equal(t, "pm", rootCmd.Use)
	assert.Contains(t, rootCmd.Short, "Pommel")
	assert.Contains(t, rootCmd.Long, "semantic code search")
}

func TestRootCommandFlags(t *testing.T) {
	// Test --json flag exists
	jsonFlag := rootCmd.PersistentFlags().Lookup("json")
	require.NotNil(t, jsonFlag)
	assert.Equal(t, "false", jsonFlag.DefValue)

	// Test --verbose flag exists
	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)
	assert.Equal(t, "false", verboseFlag.DefValue)

	// Test --project flag exists
	projectFlag := rootCmd.PersistentFlags().Lookup("project")
	require.NotNil(t, projectFlag)
	assert.Equal(t, "p", projectFlag.Shorthand)
}

func TestVersionCommand(t *testing.T) {
	// Verify version command is registered
	versionCmd := rootCmd.Commands()
	var found bool
	for _, cmd := range versionCmd {
		if cmd.Use == "version" {
			found = true
			break
		}
	}
	assert.True(t, found, "version command should be registered")
}

func TestVersionInfo(t *testing.T) {
	// Test VersionInfo struct
	info := VersionInfo{
		Version:   "1.0.0",
		Commit:    "abc123",
		Date:      "2024-01-01",
		GoVersion: "go1.21",
		OS:        "darwin",
		Arch:      "arm64",
	}

	// Verify JSON marshaling
	data, err := json.Marshal(info)
	require.NoError(t, err)

	var decoded VersionInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Version, decoded.Version)
	assert.Equal(t, info.Commit, decoded.Commit)
	assert.Equal(t, info.Date, decoded.Date)
	assert.Equal(t, info.GoVersion, decoded.GoVersion)
	assert.Equal(t, info.OS, decoded.OS)
	assert.Equal(t, info.Arch, decoded.Arch)
}

func TestOutputFormatterSuccess(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	formatter.Success("test message")
	assert.Contains(t, buf.String(), "[OK]")
	assert.Contains(t, buf.String(), "test message")
}

func TestOutputFormatterInfo(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	formatter.Info("info message")
	assert.Equal(t, "info message\n", buf.String())
}

func TestOutputFormatterWarn(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&outBuf, &errBuf)

	formatter.Warn("warning message")
	assert.Contains(t, errBuf.String(), "[WARN]")
	assert.Contains(t, errBuf.String(), "warning message")
}

func TestOutputFormatterError(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&outBuf, &errBuf)

	formatter.Error("error message")
	assert.Contains(t, errBuf.String(), "[ERROR]")
	assert.Contains(t, errBuf.String(), "error message")
}

func TestOutputFormatterJSON(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	data := map[string]string{"key": "value"}
	err := formatter.JSON(data)
	require.NoError(t, err)

	var decoded map[string]string
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "value", decoded["key"])
}

func TestOutputFormatterTable(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	headers := []string{"Name", "Value"}
	rows := [][]string{
		{"foo", "bar"},
		{"baz", "qux"},
	}

	formatter.Table(headers, rows)

	output := buf.String()
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Value")
	assert.Contains(t, output, "foo")
	assert.Contains(t, output, "bar")
	assert.Contains(t, output, "baz")
	assert.Contains(t, output, "qux")
}

func TestOutputFormatterTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	formatter.Table(nil, nil)
	assert.Empty(t, buf.String())
}

func TestOutputFormatterFormatStrings(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	formatter.Success("count: %d", 42)
	assert.Contains(t, buf.String(), "count: 42")

	buf.Reset()
	formatter.Info("name: %s", "test")
	assert.Contains(t, buf.String(), "name: test")
}

func TestDefaultOutputFormatter(t *testing.T) {
	// Just verify the default formatter is initialized
	assert.NotNil(t, DefaultOutput)
}

func TestGlobalFlagAccessors(t *testing.T) {
	// Save original values
	origJSON := jsonOutput
	origVerbose := verbose
	origProject := projectRoot

	// Cleanup
	defer func() {
		jsonOutput = origJSON
		verbose = origVerbose
		projectRoot = origProject
	}()

	// Test accessors
	jsonOutput = true
	assert.True(t, IsJSONOutput())

	jsonOutput = false
	assert.False(t, IsJSONOutput())

	verbose = true
	assert.True(t, IsVerbose())

	verbose = false
	assert.False(t, IsVerbose())

	projectRoot = "/test/path"
	assert.Equal(t, "/test/path", GetProjectRoot())
}

func TestTableSeparatorLength(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewOutputFormatterWithWriters(&buf, &buf)

	headers := []string{"Short", "LongerHeader"}
	rows := [][]string{{"a", "b"}}

	formatter.Table(headers, rows)

	lines := strings.Split(buf.String(), "\n")
	require.Len(t, lines, 4) // header, separator, row, empty

	// Verify separator line exists
	assert.Contains(t, lines[1], "-----")
	assert.Contains(t, lines[1], "------------")
}
