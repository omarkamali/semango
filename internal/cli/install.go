package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/omarkamali/semango/internal/util"
	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"
)

func NewInstallCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install semango to a PATH accessible location",
		Long:  `Copies the current semango binary to a standard system path (e.g., /usr/local/bin) so it can be run from anywhere.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Get current executable path
			exePath, err := os.Executable()
			if err != nil {
				return util.WrapError(err, "failed to get current executable path")
			}

			// 2. Determine default install path
			var defaultInstallDir string
			if runtime.GOOS == "windows" {
				// A common PATH location for user-installed binaries on Windows
				defaultInstallDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local", "Microsoft", "WindowsApps")
			} else {
				defaultInstallDir = "/usr/local/bin"
			}

			targetPath := filepath.Join(defaultInstallDir, "semango")
			if runtime.GOOS == "windows" {
				targetPath += ".exe"
			}

			// 3. Choice to override
			fmt.Printf("Default install location: %s\n", targetPath)
			fmt.Print("Press Enter to accept or type a new path: ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				targetPath = input
			}

			// 4. Check for existing version
			if _, err := os.Stat(targetPath); err == nil {
				oldVersion := getBinaryVersion(targetPath)
				fmt.Printf("Found existing semango at %s (version: %s)\n", targetPath, oldVersion)

				// Prepare versions for semver comparison
				vNew := version
				if !strings.HasPrefix(vNew, "v") && vNew != "dev" {
					vNew = "v" + vNew
				}
				vOld := oldVersion
				if !strings.HasPrefix(vOld, "v") && vOld != "dev" && vOld != "unknown" {
					vOld = "v" + vOld
				}

				if vNew == "dev" || vOld == "unknown" || vOld == "dev" {
					fmt.Print("Version information is incomplete. Override existing binary? (y/N): ")
					if !askConfirm(reader) {
						fmt.Println("Installation cancelled.")
						return nil
					}
				} else {
					if !semver.IsValid(vNew) || !semver.IsValid(vOld) {
						fmt.Print("Version information is not in standard semver format. Override existing binary? (y/N): ")
						if !askConfirm(reader) {
							fmt.Println("Installation cancelled.")
							return nil
						}
					} else {
						cmp := semver.Compare(vNew, vOld)
						if cmp > 0 {
							fmt.Printf("Installing newer version %s (currently %s)...\n", version, oldVersion)
						} else if cmp < 0 {
							fmt.Printf("Warning: Existing version %s is newer than this version %s.\n", oldVersion, version)
							fmt.Print("Override anyway? (y/N): ")
							if !askConfirm(reader) {
								fmt.Println("Installation cancelled.")
								return nil
							}
						} else {
							fmt.Printf("Version %s is already installed.\n", version)
							fmt.Print("Override anyway? (y/N): ")
							if !askConfirm(reader) {
								fmt.Println("Installation cancelled.")
								return nil
							}
						}
					}
				}
			} else {
				fmt.Printf("Install semango to %s? (y/N): ", targetPath)
				if !askConfirm(reader) {
					fmt.Println("Installation cancelled.")
					return nil
				}
			}

			// 5. Perform Install
			if err := copyBinary(exePath, targetPath); err != nil {
				return util.WrapError(err, fmt.Sprintf("failed to install semango to %s. You might need to run this command with sudo or check permissions", targetPath))
			}

			fmt.Printf("Successfully installed semango to %s\n", targetPath)
			return nil
		},
	}

	return cmd
}

func getBinaryVersion(path string) string {
	cmd := exec.Command(path, "version")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	// Expected output: "Semango 0.1.0" (first line)
	line := strings.TrimSpace(string(out))
	fields := strings.Fields(line)
	// We want the part after "Semango"
	for i, f := range fields {
		if f == "Semango" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return "unknown"
}

func askConfirm(reader *bufio.Reader) bool {
	res, _ := reader.ReadString('\n')
	res = strings.TrimSpace(res)
	res = strings.ToLower(res)
	return res == "y" || res == "yes"
}

func copyBinary(src, dst string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	// Try to create/open the destination file
	destination, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err = io.Copy(destination, source); err != nil {
		return err
	}

	return nil
}
