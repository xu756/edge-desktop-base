// Package update owns desktop application self-update mechanics.
//
// Distribution policy:
//   - .deb / .pkg / platform installers are for first-time or manual installs.
//   - automatic updates consume only the platform runtime artifact from CNB.
//   - CNB manifests and SHA-256 verification stay inside this package.
//   - writable installs use Wails' updater directly.
//   - Linux /usr/bin installs use a minimal PolicyKit-authorised atomic runtime
//     replacement, then restart the application; the DEB is not reinstalled.
package update
