package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCNBInstallerUpdateSelection(t *testing.T) {
	payload := []byte("deb package")
	digest := sha256.Sum256(payload)
	manifest := fmt.Sprintf(`{
  "schemaVersion": 1,
  "app": "my-app",
  "version": "1.2.3",
  "tag": "my-app-v1.2.3",
  "channel": "stable",
  "artifacts": [
    {
      "kind": "installer",
      "platform": "linux",
      "arch": "amd64",
      "filename": "my-app-linux-amd64.deb",
      "sha256": %q,
      "size": %d
    },
    {
      "kind": "installer",
      "platform": "darwin",
      "arch": "arm64",
      "filename": "my-app-darwin-arm64.pkg",
      "sha256": %q,
      "size": %d
    }
  ]
}`, hex.EncodeToString(digest[:]), len(payload), hex.EncodeToString(digest[:]), len(payload))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xu756/public/-/git/raw/main/my-app/latest.json" {
			_, _ = w.Write([]byte(manifest))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	provider, err := newCNBUpdaterProvider(server.URL+"/xu756/public", "main", "my-app", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	update, err := provider.checkInstallerUpdate(context.Background(), "1.2.2", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if update == nil || update.Version != "1.2.3" || update.Artifact.Filename != "my-app-linux-amd64.deb" {
		t.Fatalf("unexpected installer update: %#v", update)
	}

	upToDate, err := provider.checkInstallerUpdate(context.Background(), "1.2.3", "linux", "amd64")
	if err != nil || upToDate != nil {
		t.Fatalf("expected up to date, got %#v, %v", upToDate, err)
	}
}

func TestExpectedInstallerFilename(t *testing.T) {
	for _, tc := range []struct {
		platform, arch, want string
	}{
		{"windows", "amd64", "my-app-windows-amd64-installer.exe"},
		{"linux", "amd64", "my-app-linux-amd64.deb"},
		{"darwin", "arm64", "my-app-darwin-arm64.pkg"},
		{"freebsd", "amd64", ""},
	} {
		if got := expectedInstallerFilename("my-app", tc.platform, tc.arch); got != tc.want {
			t.Fatalf("%s/%s: %q != %q", tc.platform, tc.arch, got, tc.want)
		}
	}
}
