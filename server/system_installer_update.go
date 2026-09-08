package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	modsemver "golang.org/x/mod/semver"
)

type cnbInstallerUpdate struct {
	Version  string
	Tag      string
	Artifact cnbUpdateArtifact
}

// checkInstallerUpdate resolves the installer that belongs to the latest
// stable manifest. System-installed Linux/macOS builds use this path instead
// of replacing files owned by dpkg/pkg receipts.
func (p *cnbUpdaterProvider) checkInstallerUpdate(ctx context.Context, currentVersion, platform, arch string) (*cnbInstallerUpdate, error) {
	manifest, err := p.fetchLatestManifest(ctx)
	if err != nil || manifest == nil {
		return nil, err
	}
	if err := p.validateManifest(manifest); err != nil {
		return nil, err
	}
	currentVersion = strings.TrimPrefix(strings.TrimSpace(currentVersion), "v")
	if !validSemver(currentVersion) {
		return nil, fmt.Errorf("cnb: invalid current version %q", currentVersion)
	}
	if modsemver.Compare("v"+manifest.Version, "v"+currentVersion) <= 0 {
		return nil, nil
	}

	artifact, ok := findCNBArtifact(manifest.Artifacts, "installer", platform, arch)
	if !ok {
		return nil, fmt.Errorf("cnb: %s has no installer artifact for %s/%s", manifest.Tag, platform, arch)
	}
	expected := expectedInstallerFilename(p.app, platform, arch)
	if expected == "" || artifact.Filename != expected {
		return nil, fmt.Errorf("cnb: unexpected installer filename %q for %s/%s", artifact.Filename, platform, arch)
	}
	if _, err := decodeSHA256(artifact.SHA256); err != nil {
		return nil, fmt.Errorf("cnb: invalid checksum for %s: %w", artifact.Filename, err)
	}

	return &cnbInstallerUpdate{
		Version:  manifest.Version,
		Tag:      manifest.Tag,
		Artifact: artifact,
	}, nil
}

func expectedInstallerFilename(app, platform, arch string) string {
	base := app + "-" + platform + "-" + arch
	switch platform {
	case "windows":
		return base + "-installer.exe"
	case "linux":
		return base + ".deb"
	case "darwin":
		return base + ".pkg"
	default:
		return ""
	}
}

// downloadInstaller downloads to the user's Downloads directory, verifies the
// manifest SHA-256, then atomically exposes the final installer filename.
func (p *cnbUpdaterProvider) downloadInstaller(ctx context.Context, update *cnbInstallerUpdate) (string, error) {
	if update == nil {
		return "", errors.New("cnb: installer update is nil")
	}
	if update.Tag != p.app+"-v"+update.Version {
		return "", fmt.Errorf("cnb: installer tag %q does not match app/version", update.Tag)
	}

	dir, err := userDownloadsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create downloads directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "."+update.Artifact.Filename+"-*.part")
	if err != nil {
		return "", fmt.Errorf("create installer download: %w", err)
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.releaseDownloadURL(update.Tag, update.Artifact.Filename), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cnb: installer download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cnb: installer download: HTTP %d", resp.StatusCode)
	}

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body)
	if err != nil {
		return "", fmt.Errorf("cnb: installer download: %w", err)
	}
	if update.Artifact.Size > 0 && written != update.Artifact.Size {
		return "", fmt.Errorf("cnb: installer size mismatch: got %d, want %d", written, update.Artifact.Size)
	}
	want, err := decodeSHA256(update.Artifact.SHA256)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), hex.EncodeToString(want)) {
		return "", errors.New("cnb: installer SHA-256 verification failed")
	}
	if err := tmp.Sync(); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	finalPath := filepath.Join(dir, update.Artifact.Filename)
	_ = os.Remove(finalPath)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("finalise installer download: %w", err)
	}
	keep = true
	return finalPath, nil
}

func userDownloadsDir() (string, error) {
	if runtime.GOOS == "linux" {
		if tool, err := exec.LookPath("xdg-user-dir"); err == nil {
			if output, err := exec.Command(tool, "DOWNLOAD").Output(); err == nil {
				if value := strings.TrimSpace(string(output)); value != "" {
					return value, nil
				}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, "Downloads"), nil
}
