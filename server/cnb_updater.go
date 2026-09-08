package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
	modsemver "golang.org/x/mod/semver"
)

const cnbManifestSchemaVersion = 1

type cnbUpdateArtifact struct {
	Kind     string `json:"kind"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size,omitempty"`
}

type cnbUpdateManifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	App           string              `json:"app"`
	Version       string              `json:"version"`
	Tag           string              `json:"tag"`
	Channel       string              `json:"channel,omitempty"`
	Notes         string              `json:"notes,omitempty"`
	PublishedAt   time.Time           `json:"publishedAt,omitempty"`
	Artifacts     []cnbUpdateArtifact `json:"artifacts"`
}

type cnbUpdaterProvider struct {
	repositoryURL string
	branch        string
	app           string
	client        *http.Client
}

func newCNBUpdaterProvider(repositoryURL, branch, app string, client *http.Client) (*cnbUpdaterProvider, error) {
	repositoryURL = strings.TrimRight(strings.TrimSpace(repositoryURL), "/")
	branch = strings.TrimSpace(branch)
	app = strings.TrimSpace(app)
	if repositoryURL == "" || branch == "" || app == "" {
		return nil, errors.New("cnb: repository URL, branch and app are required")
	}
	u, err := url.Parse(repositoryURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("cnb: invalid repository URL %q", repositoryURL)
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	return &cnbUpdaterProvider{
		repositoryURL: repositoryURL,
		branch:        branch,
		app:           app,
		client:        client,
	}, nil
}

func (p *cnbUpdaterProvider) Name() string { return "cnb" }

func (p *cnbUpdaterProvider) Check(ctx context.Context, req updater.CheckRequest) (*updater.Release, error) {
	manifest, err := p.fetchLatestManifest(ctx)
	if err != nil || manifest == nil {
		return nil, err
	}
	if err := p.validateManifest(manifest); err != nil {
		return nil, err
	}
	if !validSemver(req.CurrentVersion) {
		return nil, fmt.Errorf("cnb: invalid current version %q", req.CurrentVersion)
	}
	if modsemver.Compare("v"+manifest.Version, "v"+strings.TrimPrefix(req.CurrentVersion, "v")) <= 0 {
		return nil, nil
	}

	artifact, ok := findCNBArtifact(manifest.Artifacts, "runtime", req.Platform, req.Arch)
	if !ok {
		return nil, fmt.Errorf("cnb: %s has no runtime artifact for %s/%s", manifest.Tag, req.Platform, req.Arch)
	}
	expected := expectedRuntimeFilename(p.app, req.Platform, req.Arch)
	if expected == "" || artifact.Filename != expected {
		return nil, fmt.Errorf("cnb: unexpected runtime filename %q for %s/%s", artifact.Filename, req.Platform, req.Arch)
	}
	digest, err := decodeSHA256(artifact.SHA256)
	if err != nil {
		return nil, fmt.Errorf("cnb: invalid checksum for %s: %w", artifact.Filename, err)
	}

	channel := manifest.Channel
	if channel == "" {
		channel = "stable"
	}
	return &updater.Release{
		Version:     manifest.Version,
		Channel:     channel,
		Name:        p.app + " v" + manifest.Version,
		Notes:       manifest.Notes,
		PublishedAt: manifest.PublishedAt,
		Artifact: updater.Artifact{
			Filename: artifact.Filename,
			Filetype: filetypeOf(artifact.Filename),
			Size:     artifact.Size,
			Platform: req.Platform,
			Arch:     req.Arch,
		},
		Verification: &updater.Verification{
			DigestAlgo: "sha256",
			Digest:     digest,
		},
		Metadata: map[string]any{
			"cnb.release.tag":     manifest.Tag,
			"cnb.release.pageURL": p.releasePageURL(manifest.Tag),
		},
	}, nil
}

func (p *cnbUpdaterProvider) Download(ctx context.Context, release *updater.Release, dst io.Writer, onProgress func(written, total int64)) error {
	if release == nil || release.Metadata == nil {
		return errors.New("cnb: release missing metadata")
	}
	tag, ok := release.Metadata["cnb.release.tag"].(string)
	if !ok || tag == "" {
		return errors.New("cnb: release metadata missing tag")
	}
	if tag != p.app+"-v"+release.Version {
		return fmt.Errorf("cnb: release tag %q does not match app/version", tag)
	}
	expected := expectedRuntimeFilename(p.app, release.Artifact.Platform, release.Artifact.Arch)
	if expected == "" || release.Artifact.Filename != expected {
		return fmt.Errorf("cnb: refusing unexpected runtime artifact %q", release.Artifact.Filename)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.releaseDownloadURL(tag, release.Artifact.Filename), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("cnb: download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cnb: download: HTTP %d", resp.StatusCode)
	}

	total := release.Artifact.Size
	if total <= 0 && resp.ContentLength > 0 {
		total = resp.ContentLength
	}
	var written int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			wn, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			if wn != n {
				return io.ErrShortWrite
			}
			written += int64(n)
			if onProgress != nil {
				onProgress(written, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if release.Artifact.Size > 0 && written != release.Artifact.Size {
		return fmt.Errorf("cnb: downloaded size mismatch: got %d, want %d", written, release.Artifact.Size)
	}
	return nil
}

func (p *cnbUpdaterProvider) fetchLatestManifest(ctx context.Context) (*cnbUpdateManifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.latestManifestURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cnb: fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("cnb: manifest HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var manifest cnbUpdateManifest
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("cnb: decode manifest: %w", err)
	}
	return &manifest, nil
}

func (p *cnbUpdaterProvider) validateManifest(manifest *cnbUpdateManifest) error {
	if manifest.SchemaVersion != cnbManifestSchemaVersion {
		return fmt.Errorf("cnb: unsupported manifest schema %d", manifest.SchemaVersion)
	}
	if manifest.App != p.app {
		return fmt.Errorf("cnb: manifest app %q does not match %q", manifest.App, p.app)
	}
	if !validSemver(manifest.Version) {
		return fmt.Errorf("cnb: invalid manifest version %q", manifest.Version)
	}
	expectedTag := p.app + "-v" + manifest.Version
	if manifest.Tag != expectedTag {
		return fmt.Errorf("cnb: manifest tag %q does not match %q", manifest.Tag, expectedTag)
	}
	return nil
}

func (p *cnbUpdaterProvider) latestManifestURL() string {
	return p.repositoryURL + "/-/git/raw/" + url.PathEscape(p.branch) + "/" + url.PathEscape(p.app) + "/latest.json"
}

func (p *cnbUpdaterProvider) releaseDownloadURL(tag, filename string) string {
	return p.repositoryURL + "/-/releases/download/" + url.PathEscape(tag) + "/" + url.PathEscape(filename)
}

func (p *cnbUpdaterProvider) releasePageURL(tag string) string {
	return p.repositoryURL + "/-/releases/tag/" + url.PathEscape(tag)
}

func (p *cnbUpdaterProvider) releasesPageURL() string {
	return p.repositoryURL + "/-/releases"
}

func findCNBArtifact(artifacts []cnbUpdateArtifact, kind, platform, arch string) (cnbUpdateArtifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Kind == kind && artifact.Platform == platform && artifact.Arch == arch {
			return artifact, true
		}
	}
	return cnbUpdateArtifact{}, false
}

func expectedRuntimeFilename(app, platform, arch string) string {
	base := app + "-" + platform + "-" + arch
	switch platform {
	case "windows":
		return base + ".exe"
	case "darwin":
		return base + ".zip"
	case "linux":
		return base
	default:
		return ""
	}
}

func decodeSHA256(value string) ([]byte, error) {
	digest, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if len(digest) != 32 {
		return nil, fmt.Errorf("expected 32 bytes, got %d", len(digest))
	}
	return digest, nil
}

func validSemver(version string) bool {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	return modsemver.IsValid("v" + version)
}

func filetypeOf(filename string) string {
	if index := strings.LastIndex(filename, "."); index >= 0 {
		return strings.ToLower(filename[index+1:])
	}
	return ""
}
