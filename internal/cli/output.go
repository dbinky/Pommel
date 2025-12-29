package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// OutputFormatter handles formatted output to the console
type OutputFormatter struct {
	out    io.Writer
	errOut io.Writer
}

// NewOutputFormatter creates a new OutputFormatter with default stdout/stderr
func NewOutputFormatter() *OutputFormatter {
	return &OutputFormatter{
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

// NewOutputFormatterWithWriters creates an OutputFormatter with custom writers
func NewOutputFormatterWithWriters(out, errOut io.Writer) *OutputFormatter {
	return &OutputFormatter{
		out:    out,
		errOut: errOut,
	}
}

// Success prints a success message with a checkmark prefix
func (o *OutputFormatter) Success(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.out, "[OK] %s\n", msg)
}

// Info prints an informational message
func (o *OutputFormatter) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.out, "%s\n", msg)
}

// Warn prints a warning message with a warning prefix
func (o *OutputFormatter) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.errOut, "[WARN] %s\n", msg)
}

// Error prints an error message with an error prefix
func (o *OutputFormatter) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(o.errOut, "[ERROR] %s\n", msg)
}

// JSON outputs data as formatted JSON
func (o *OutputFormatter) JSON(data interface{}) error {
	encoder := json.NewEncoder(o.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Table prints data in a tabular format
// headers is a slice of column headers
// rows is a slice of row data, where each row is a slice of strings
func (o *OutputFormatter) Table(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(o.out, 0, 0, 2, ' ', 0)

	// Print headers
	if len(headers) > 0 {
		fmt.Fprintln(w, strings.Join(headers, "\t"))
		// Print separator
		separators := make([]string, len(headers))
		for i, h := range headers {
			separators[i] = strings.Repeat("-", len(h))
		}
		fmt.Fprintln(w, strings.Join(separators, "\t"))
	}

	// Print rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	w.Flush()
}

// DefaultOutput is the default output formatter instance
var DefaultOutput = NewOutputFormatter()

// Success prints a success message using the default formatter
func Success(format string, args ...interface{}) {
	DefaultOutput.Success(format, args...)
}

// Info prints an info message using the default formatter
func Info(format string, args ...interface{}) {
	DefaultOutput.Info(format, args...)
}

// Warn prints a warning message using the default formatter
func Warn(format string, args ...interface{}) {
	DefaultOutput.Warn(format, args...)
}

// Error prints an error message using the default formatter
func Error(format string, args ...interface{}) {
	DefaultOutput.Error(format, args...)
}

// JSON outputs data as JSON using the default formatter
func JSON(data interface{}) error {
	return DefaultOutput.JSON(data)
}

// Table prints a table using the default formatter
func Table(headers []string, rows [][]string) {
	DefaultOutput.Table(headers, rows)
}
