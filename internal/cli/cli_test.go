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

// =============================================================================
// Tests for global output functions (package-level wrappers)
// =============================================================================

func TestGlobalSuccessFunction(t *testing.T) {
	// Save the original default output formatter
	origDefault := DefaultOutput

	// Create a custom output with buffer to capture output
	var buf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&buf, &buf)

	// Cleanup
	defer func() {
		DefaultOutput = origDefault
	}()

	Success("test %s %d", "message", 42)

	assert.Contains(t, buf.String(), "[OK]")
	assert.Contains(t, buf.String(), "test message 42")
}

func TestGlobalInfoFunction(t *testing.T) {
	origDefault := DefaultOutput
	var buf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&buf, &buf)
	defer func() { DefaultOutput = origDefault }()

	Info("info %s", "text")

	assert.Contains(t, buf.String(), "info text")
}

func TestGlobalWarnFunction(t *testing.T) {
	origDefault := DefaultOutput
	var outBuf, errBuf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&outBuf, &errBuf)
	defer func() { DefaultOutput = origDefault }()

	Warn("warning %d", 123)

	assert.Contains(t, errBuf.String(), "[WARN]")
	assert.Contains(t, errBuf.String(), "warning 123")
}

func TestGlobalErrorFunction(t *testing.T) {
	origDefault := DefaultOutput
	var outBuf, errBuf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&outBuf, &errBuf)
	defer func() { DefaultOutput = origDefault }()

	Error("error occurred: %v", "details")

	assert.Contains(t, errBuf.String(), "[ERROR]")
	assert.Contains(t, errBuf.String(), "error occurred: details")
}

func TestGlobalJSONFunction(t *testing.T) {
	origDefault := DefaultOutput
	var buf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&buf, &buf)
	defer func() { DefaultOutput = origDefault }()

	data := map[string]int{"count": 5}
	err := JSON(data)
	require.NoError(t, err)

	var decoded map[string]int
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)
	assert.Equal(t, 5, decoded["count"])
}

func TestGlobalTableFunction(t *testing.T) {
	origDefault := DefaultOutput
	var buf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&buf, &buf)
	defer func() { DefaultOutput = origDefault }()

	Table([]string{"Col1", "Col2"}, [][]string{{"a", "b"}, {"c", "d"}})

	output := buf.String()
	assert.Contains(t, output, "Col1")
	assert.Contains(t, output, "Col2")
	assert.Contains(t, output, "a")
	assert.Contains(t, output, "d")
}

// =============================================================================
// Tests for version command functions
// =============================================================================

func TestPrintVersionJSON(t *testing.T) {
	// Capture stdout
	oldStdout := DefaultOutput.out

	var buf bytes.Buffer
	DefaultOutput = NewOutputFormatterWithWriters(&buf, &buf)
	defer func() { DefaultOutput.out = oldStdout }()

	info := VersionInfo{
		Version:   "1.2.3",
		Commit:    "abc123",
		Date:      "2024-01-15",
		GoVersion: "go1.21",
		OS:        "linux",
		Arch:      "amd64",
	}

	err := printVersionJSON(info)
	require.NoError(t, err)
	// Note: printVersionJSON uses fmt.Println directly, so we test it indirectly
}

func TestPrintVersionText(t *testing.T) {
	info := VersionInfo{
		Version:   "2.0.0",
		Commit:    "def456",
		Date:      "2024-06-01",
		GoVersion: "go1.22",
		OS:        "darwin",
		Arch:      "arm64",
	}

	err := printVersionText(info)
	require.NoError(t, err)
	// This prints to stdout, but we verify no error is returned
}

// =============================================================================
// Tests for root command functions
// =============================================================================

func TestExecuteFunction(t *testing.T) {
	// This tests the Execute function - we set up args for --help to avoid side effects
	// Save original args
	oldArgs := rootCmd.Args

	// Set args for help to get a clean execution
	rootCmd.SetArgs([]string{"--help"})

	// Execute returns nil for help
	// We just verify it doesn't panic
	_ = Execute()

	// Restore
	rootCmd.Args = oldArgs
}

func TestRegisterConfigCommand(t *testing.T) {
	// First check if config is already registered
	var alreadyRegistered bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "config [get|set] [key] [value]" {
			alreadyRegistered = true
			break
		}
	}

	// Only register if not already registered
	if !alreadyRegistered {
		RegisterConfigCommand()
	}

	// Verify config command is now registered
	var found bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "config [get|set] [key] [value]" {
			found = true
			break
		}
	}
	assert.True(t, found, "config command should be registered after RegisterConfigCommand")
}

// =============================================================================
// CLI Error Tests
// =============================================================================

func TestCLIError_ErrorMethod(t *testing.T) {
	testCases := []struct {
		name       string
		err        *CLIError
		wantMsg    string
		wantSuggest bool
	}{
		{
			name: "with suggestion",
			err: &CLIError{
				Message:    "Something went wrong",
				Suggestion: "Try again later",
			},
			wantMsg:    "Something went wrong",
			wantSuggest: true,
		},
		{
			name: "without suggestion",
			err: &CLIError{
				Message: "Error occurred",
			},
			wantMsg:    "Error occurred",
			wantSuggest: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			errStr := tc.err.Error()
			assert.Contains(t, errStr, tc.wantMsg)
			if tc.wantSuggest {
				assert.Contains(t, errStr, "Suggestion:")
			} else {
				assert.NotContains(t, errStr, "Suggestion:")
			}
		})
	}
}

func TestCLIError_Unwrap(t *testing.T) {
	cause := assert.AnError
	err := &CLIError{
		Message: "Wrapper error",
		Cause:   cause,
	}

	unwrapped := err.Unwrap()
	assert.Equal(t, cause, unwrapped)
}

func TestCLIError_UnwrapNil(t *testing.T) {
	err := &CLIError{
		Message: "No cause",
	}

	unwrapped := err.Unwrap()
	assert.Nil(t, unwrapped)
}

func TestErrDaemonNotRunning(t *testing.T) {
	err := ErrDaemonNotRunning()
	assert.Contains(t, err.Message, "not running")
	assert.NotEmpty(t, err.Suggestion)
}

func TestErrDaemonHealthTimeout(t *testing.T) {
	err := ErrDaemonHealthTimeout()
	assert.Contains(t, err.Message, "health check")
	assert.NotEmpty(t, err.Suggestion)
}

func TestErrDaemonConnectionFailed(t *testing.T) {
	cause := assert.AnError
	err := ErrDaemonConnectionFailed(cause)
	assert.Contains(t, err.Message, "Cannot connect")
	assert.NotEmpty(t, err.Suggestion)
	assert.Equal(t, cause, err.Cause)
}

func TestErrConfigNotFound(t *testing.T) {
	err := ErrConfigNotFound()
	assert.Contains(t, err.Message, "not found")
	assert.NotEmpty(t, err.Suggestion)
}

func TestErrInvalidProjectRoot(t *testing.T) {
	err := ErrInvalidProjectRoot("/invalid/path")
	assert.Contains(t, err.Message, "/invalid/path")
	assert.NotEmpty(t, err.Suggestion)
}

func TestErrEmptyQuery(t *testing.T) {
	err := ErrEmptyQuery()
	assert.Contains(t, err.Message, "empty")
	assert.NotEmpty(t, err.Suggestion)
}

func TestErrInvalidLevel(t *testing.T) {
	validLevels := []string{"method", "class", "file"}
	err := ErrInvalidLevel("block", validLevels)
	assert.Contains(t, err.Message, "block")
	assert.Contains(t, err.Suggestion, "method")
}

func TestErrReindexFailed(t *testing.T) {
	cause := assert.AnError
	err := ErrReindexFailed(cause)
	assert.Contains(t, err.Message, "reindex")
	assert.NotEmpty(t, err.Suggestion)
	assert.Equal(t, cause, err.Cause)
}

func TestErrNoSearchResults(t *testing.T) {
	err := ErrNoSearchResults("test query")
	assert.Contains(t, err.Message, "test query")
	assert.NotEmpty(t, err.Suggestion)
}
