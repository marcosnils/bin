//go:build !windows

package providers

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/caarlos0/log"
)

// installAWSCLI extracts and installs the AWS CLI package to installDir.
// On Linux, archivePath is a .zip file containing the installer.
// On macOS, archivePath is a .pkg file.
// Returns the directory containing the aws and aws_completer binaries.
func installAWSCLI(archivePath, installDir, goos, version string) (string, error) {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create install directory %s: %w", installDir, err)
	}

	switch goos {
	case "linux":
		if err := installAWSCLILinux(archivePath, installDir); err != nil {
			return "", err
		}
		// Ensure the "current" symlink points to the version we just installed.
		// The AWS installer may skip this if the version directory already exists.
		if err := ensureCurrentSymlink(installDir, version); err != nil {
			return "", err
		}
		// Linux installer creates v2/current/bin/aws
		return filepath.Join(installDir, "v2", "current", "bin"), nil

	case "darwin":
		if err := installAWSCLIDarwin(archivePath, installDir); err != nil {
			return "", err
		}
		// macOS pkg has a flat layout: aws and aws_completer are at the root
		return installDir, nil

	default:
		return "", fmt.Errorf("unsupported OS for AWS CLI installation: %s", goos)
	}
}

// installAWSCLILinux extracts the zip and runs the bundled install script.
func installAWSCLILinux(zipPath, installDir string) error {
	// Extract zip to temp directory
	extractDir, err := os.MkdirTemp("", "awscli-extract-*")
	if err != nil {
		return fmt.Errorf("failed to create temp extraction directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	log.Infof("Extracting AWS CLI installer")
	if err := extractZip(zipPath, extractDir); err != nil {
		return fmt.Errorf("failed to extract AWS CLI zip: %w", err)
	}

	// The zip contains an "aws" directory with the install script
	installScript := filepath.Join(extractDir, "aws", "install")
	if _, err := os.Stat(installScript); err != nil {
		return fmt.Errorf("install script not found at %s: %w", installScript, err)
	}

	// Build install command arguments
	// --bin-dir points inside the install dir since we create our own wrapper scripts
	binDir := filepath.Join(installDir, "bin")
	args := []string{
		"--install-dir", installDir,
		"--bin-dir", binDir,
	}

	// Check if this is an update (install dir already has a version)
	if _, err := os.Stat(filepath.Join(installDir, "v2")); err == nil {
		args = append(args, "--update")
	}

	log.Infof("Running AWS CLI installer")
	cmd := exec.Command(installScript, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AWS CLI installer failed: %w", err)
	}

	return nil
}

// installAWSCLIDarwin extracts the .pkg and copies the AWS CLI files to installDir.
// The macOS .pkg has a flat structure: aws-cli.pkg/Payload/aws-cli/ contains
// the aws and aws_completer binaries alongside their bundled Python runtime.
func installAWSCLIDarwin(pkgPath, installDir string) error {
	expandDir, err := os.MkdirTemp("", "awscli-pkg-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(expandDir)

	expandTarget := filepath.Join(expandDir, "awscli")

	log.Infof("Expanding AWS CLI package")
	cmd := exec.Command("pkgutil", "--expand-full", pkgPath, expandTarget)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to expand .pkg: %w", err)
	}

	// The expanded .pkg contains: aws-cli.pkg/Payload/aws-cli/{aws,aws_completer,Python,...}
	payloadDir, err := findAWSPayload(expandTarget)
	if err != nil {
		return fmt.Errorf("failed to locate AWS CLI payload in expanded .pkg: %w", err)
	}

	log.Infof("Installing AWS CLI files to %s", installDir)
	if err := copyDir(payloadDir, installDir); err != nil {
		return fmt.Errorf("failed to copy AWS CLI files: %w", err)
	}

	return nil
}

// findAWSPayload searches the expanded pkg directory for the AWS CLI payload.
// It looks for the "aws" executable within the directory tree.
func findAWSPayload(expandedDir string) (string, error) {
	// Known .pkg payload path: aws-cli.pkg/Payload/aws-cli/
	// The aws binary lives directly in this directory (flat layout, no v2/current/bin/).
	candidate := filepath.Join(expandedDir, "aws-cli.pkg", "Payload", "aws-cli")
	if _, err := os.Stat(filepath.Join(candidate, "aws")); err == nil {
		return candidate, nil
	}

	// Fallback: walk the directory to find the aws executable
	var found string
	filepath.Walk(expandedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "aws" && !info.IsDir() && info.Mode()&0o111 != 0 {
			// Found an executable named "aws"; use its parent directory
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})

	if found != "" {
		return found, nil
	}

	return "", fmt.Errorf("could not find AWS CLI payload in %s", expandedDir)
}

// extractZip extracts a zip file to the destination directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)

		// Prevent zip slip
		cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid zip entry: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// copyDir recursively copies a directory tree, preserving symlinks.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			os.Remove(target) // remove existing if any
			return os.Symlink(link, target)
		}

		return copyFile(path, target, info.Mode())
	})
}

// ensureCurrentSymlink ensures the v2/current symlink points to the given version.
// This handles the case where the AWS installer skips because the version dir already exists.
func ensureCurrentSymlink(installDir, version string) error {
	if version == "" {
		return nil
	}

	versionDir := filepath.Join(installDir, "v2", version)
	if _, err := os.Stat(versionDir); err != nil {
		// Version directory doesn't exist; the installer should have created it.
		// If it didn't, something went wrong, but we can't fix it here.
		return nil
	}

	currentLink := filepath.Join(installDir, "v2", "current")

	// Read current symlink target
	target, err := os.Readlink(currentLink)
	if err == nil && target == version {
		return nil // already correct
	}

	// Remove existing symlink/file and create new one
	os.Remove(currentLink)
	return os.Symlink(version, currentLink)
}

// copyFile copies a single file.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
