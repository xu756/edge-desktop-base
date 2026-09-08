// Package update owns desktop application self-update mechanics.
//
// Distribution policy:
//   - .deb / .pkg / platform installers are for first-time or manual installs.
//   - application updates consume only the platform runtime artifact from CNB.
//   - Check, download/stage and restart/apply are separate lifecycle phases.
//   - background checks never open the updater window.
//   - optional automatic download stops at the Ready state and never restarts.
//   - CNB manifests and SHA-256 verification stay inside this package/Wails.
//   - writable installs use Wails' updater restart helper directly.
//   - Linux /usr/bin installs request PolicyKit authorisation only when the user
//     chooses to apply the already-downloaded runtime; the DEB is not reinstalled.
package update
