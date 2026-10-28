package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	//"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/agnosto/fansly-scraper/utils"
)

const (
	githubAPIURL = "https://api.github.com/repos/agnosto/fansly-scraper/releases/latest"
)

type GithubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func CheckUpdateAvailable(currentVersion string) (bool, string, error) {
	release, err := getLatestRelease()
	if err != nil {
		return false, "", fmt.Errorf("failed to get latest release: %w", err)
	}

	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}

	return release.TagName != currentVersion, release.TagName, nil
}

func CheckForUpdate(currentVersion string) error {
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if !strings.HasPrefix(currentVersion, "v") {
		currentVersion = "v" + currentVersion
	}

	if release.TagName == currentVersion {
		fmt.Println("You are already on the latest version.")
		return nil
	}

	fmt.Printf("New version available: %s\n", release.TagName)
	return updateBinary(release)
}

func getLatestRelease() (*GithubRelease, error) {
	resp, err := utils.HTTPClient.Get(githubAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github API returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release GithubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	if release.TagName == "" {
		return nil, fmt.Errorf("github release response contained no tag name")
	}

	return &release, nil
}

func updateBinary(release *GithubRelease) error {
	assetName := fmt.Sprintf("fansly-scraper_%s_%s_%s.tar.gz", strings.TrimPrefix(release.TagName, "v"), runtime.GOOS, runtime.GOARCH)

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no suitable binary found for your system")
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	fmt.Println("Downloading new version...")
	resp, err := utils.HTTPClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("update download returned status: %d", resp.StatusCode)
	}

	// A fresh directory next to the executable keeps cleanup away from user
	// files and lets the final rename stay on one filesystem.
	tempDir, err := os.MkdirTemp(filepath.Dir(execPath), ".fansly-scraper-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "update.tar.gz")
	outFile, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	_, err = io.Copy(outFile, resp.Body)
	if closeErr := outFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	// Extract the update
	err = extractUpdate(tempFile, tempDir)
	if err != nil {
		return err
	}

	// Find the new executable in the extracted files
	var newExePath string
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), "fansly-scraper") {
			newExePath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return err
	}

	if newExePath == "" {
		return fmt.Errorf("new executable not found in the update package")
	}

	// Rename the current executable to .old
	oldExePath := execPath + ".old"
	err = os.Rename(execPath, oldExePath)
	if err != nil {
		return err
	}

	// Move the new executable to replace the current one
	err = os.Rename(newExePath, execPath)
	if err != nil {
		// If moving the new executable fails, try to restore the old one
		os.Rename(oldExePath, execPath)
		return err
	}

	// Set the correct permissions for the new executable
	err = os.Chmod(execPath, 0755)
	if err != nil {
		return err
	}

	// Remove the old executable
	os.Remove(oldExePath)

	fmt.Println("Update successful. Please restart the application.")
	return nil
}

func extractUpdate(archivePath, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Guard against path traversal (zip-slip) from a malicious archive.
		cleanName := filepath.Clean(header.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("archive contains invalid path: %s", header.Name)
		}
		target := filepath.Join(destPath, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
