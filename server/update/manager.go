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
	CurrentVersion string
	HTTPClient    *http.Client
}

// Manager owns the update source and the small amount of platform-specific
// runtime replacement needed by system-installed builds.
type Manager struct {
	provider       *cnbProvider
	appName        string
	currentVersion string
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.CurrentVersion == "" {
		return nil, errors.New("update: current version is required")
	}
	provider, err := newCNBProvider(cfg.RepositoryURL, cfg.Branch, cfg.AppName, cfg.HTTPClient)
	if err != nil {
		return nil, err
	}
	return &Manager{
		provider:       provider,
		appName:        cfg.AppName,
		currentVersion: cfg.CurrentVersion,
	}, nil
}

// Provider exposes the CNB source through Wails' generic updater.Provider
// interface. Wails remains responsible for the normal writable-path update
// lifecycle (download, stage, swap and restart).
func (m *Manager) Provider() updater.Provider {
	return m.provider
}

// NeedsPrivilegedRuntimeUpdate reports whether the current executable lives in
// a system-owned path that Wails cannot replace as the desktop user.
func (m *Manager) NeedsPrivilegedRuntimeUpdate() bool {
	return needsPrivilegedRuntimeUpdate(m.appName)
}

// InstallHint is UI-facing information only. Update policy stays inside this
// package instead of leaking path checks into the server package.
func (m *Manager) InstallHint() string {
	return privilegedRuntimeUpdateHint(m.appName)
}

// PerformPrivilegedRuntimeUpdate downloads the same runtime artifact exposed
// by Provider(), verifies it, performs the minimal privileged swap and then
// asks the caller to quit so the new binary can be relaunched.
func (m *Manager) PerformPrivilegedRuntimeUpdate(ctx context.Context, quit func()) error {
	if !m.NeedsPrivilegedRuntimeUpdate() {
		return errors.New("update: privileged runtime update is not required")
	}
	return performPrivilegedRuntimeUpdate(ctx, m.provider, m.currentVersion, m.appName, quit)
}
