package update

import (
	"context"
	"errors"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// Config contains the application-specific update settings. The CNB
// repository itself is public; credentials are only required by the release
// publisher, never by desktop clients.
type Config struct {
	RepositoryURL string
	Branch        string
	AppName       string
	HTTPClient    *http.Client
}

// Manager owns the update source and the small amount of platform-specific
// runtime replacement needed by system-installed builds.
type Manager struct {
	provider *cnbProvider
	appName  string
}

func NewManager(cfg Config) (*Manager, error) {
	provider, err := newCNBProvider(cfg.RepositoryURL, cfg.Branch, cfg.AppName, cfg.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &Manager{provider: provider, appName: cfg.AppName}, nil
}

// Provider exposes the CNB source through Wails' generic updater.Provider
// interface. Wails is responsible for Check and DownloadAndInstall on every
// platform, including SHA-256 verification and staging.
func (m *Manager) Provider() updater.Provider {
	return m.provider
}

// NeedsPrivilegedRuntimeUpdate reports whether applying the staged runtime
// requires elevated permissions. The update may still be downloaded and
// verified without privileges.
func (m *Manager) NeedsPrivilegedRuntimeUpdate() bool {
	return needsPrivilegedRuntimeUpdate(m.appName)
}

// InstallHint is UI-facing information only. Update policy stays inside this
// package instead of leaking path checks into the server package.
func (m *Manager) InstallHint() string {
	return privilegedRuntimeUpdateHint(m.appName)
}

// ApplyStagedRuntimeUpdate applies an artifact that Wails already downloaded
// and verified. This is only needed for system-owned runtime locations such as
// /usr/bin on Linux; normal writable installs use updater.Restart directly.
func (m *Manager) ApplyStagedRuntimeUpdate(ctx context.Context, stagedPath string, quit func()) error {
	if !m.NeedsPrivilegedRuntimeUpdate() {
		return errors.New("update: privileged runtime update is not required")
	}
	if stagedPath == "" {
		return errors.New("update: staged runtime path is empty")
	}
	return applyPrivilegedRuntimeUpdate(ctx, stagedPath, m.appName, quit)
}
