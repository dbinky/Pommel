package cli

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// VersionInfo contains build information for the version command
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Display the version, build commit, build date, Go version, and platform information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := VersionInfo{
			Version:   Version,
			Commit:    BuildCommit,
			Date:      BuildDate,
			GoVersion: runtime.Version(),
			OS:        runtime.GOOS,
			Arch:      runtime.GOARCH,
		}

		if IsJSONOutput() {
			return printVersionJSON(info)
		}
		return printVersionText(info)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func printVersionJSON(info VersionInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version info: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printVersionText(info VersionInfo) error {
	fmt.Printf("Pommel %s\n", info.Version)
	fmt.Printf("  Commit:     %s\n", info.Commit)
	fmt.Printf("  Built:      %s\n", info.Date)
	fmt.Printf("  Go version: %s\n", info.GoVersion)
	fmt.Printf("  OS/Arch:    %s/%s\n", info.OS, info.Arch)
	return nil
}
