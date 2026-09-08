#!/bin/bash
# Package the existing signed bundle without modifying the ZIP updater payload.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: $0 path/to/app.app output.pkg" >&2
  exit 1
fi
app_path="$1"
output_path="$2"
app_name="$(basename "$app_path")"
identifier="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "$app_path/Contents/Info.plist")"
version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "$app_path/Contents/Info.plist")"
staging="$(mktemp -d)"
trap 'rm -rf "$staging"' EXIT
mkdir -p "$staging/payload"
ditto "$app_path" "$staging/payload/$app_name"

# Disable relocation: an old copy in Downloads must not become the install target.
pkgbuild --analyze --root "$staging/payload" "$staging/components.plist"
/usr/libexec/PlistBuddy -c 'Set :0:BundleIsRelocatable false' "$staging/components.plist"
/usr/libexec/PlistBuddy -c 'Set :0:BundleOverwriteAction upgrade' "$staging/components.plist"
pkgbuild \
  --root "$staging/payload" \
  --component-plist "$staging/components.plist" \
  --identifier "${identifier}.installer" \
  --version "$version" \
  --install-location /Applications \
  --ownership recommended \
  "$output_path"
