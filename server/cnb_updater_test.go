package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

func TestCNBProviderCheckAndDownload(t *testing.T) {
	payload := []byte("new runtime binary")
	digest := sha256.Sum256(payload)
	manifest := fmt.Sprintf(`{
  "schemaVersion": 1,
  "app": "my-app",
  "version": "1.2.3",
  "tag": "my-app-v1.2.3",
  "channel": "stable",
  "artifacts": [
    {
      "kind": "runtime",
      "platform": "windows",
      "arch": "amd64",
      "filename": "my-app-windows-amd64.exe",
      "sha256": %q,
      "size": %d
    },
    {
      "kind": "installer",
      "platform": "windows",
      "arch": "amd64",
      "filename": "my-app-windows-amd64-installer.exe",
      "sha256": %q,
      "size": 1
    }
  ]
}`, hex.EncodeToString(digest[:]), len(payload), hex.EncodeToString(digest[:]))

	mux := http.NewServeMux()
	mux.HandleFunc("/xu756/public/-/git/raw/main/my-app/latest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(manifest))
	})
	mux.HandleFunc("/xu756/public/-/releases/download/my-app-v1.2.3/my-app-windows-amd64.exe", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	provider, err := newCNBUpdaterProvider(server.URL+"/xu756/public", "main", "my-app", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	release, err := provider.Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "1.2.2",
		Platform:       "windows",
		Arch:           "amd64",
	})
	if err != nil {
		t.Fatal(err)
	}
	if release == nil || release.Version != "1.2.3" || release.Artifact.Filename != "my-app-windows-amd64.exe" {
		t.Fatalf("unexpected release: %#v", release)
	}
	if release.Verification == nil || release.Verification.DigestAlgo != "sha256" || !bytes.Equal(release.Verification.Digest, digest[:]) {
		t.Fatalf("unexpected verification: %#v", release.Verification)
	}

	var downloaded bytes.Buffer
	var lastWritten, lastTotal int64
	if err := provider.Download(context.Background(), release, &downloaded, func(written, total int64) {
		lastWritten, lastTotal = written, total
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(downloaded.Bytes(), payload) {
		t.Fatal("downloaded payload mismatch")
	}
	if lastWritten != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("unexpected progress: %d/%d", lastWritten, lastTotal)
	}

	upToDate, err := provider.Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "1.2.3",
		Platform:       "windows",
		Arch:           "amd64",
	})
	if err != nil || upToDate != nil {
		t.Fatalf("expected up to date, got %#v, %v", upToDate, err)
	}
}

func TestCNBProviderRejectsCrossAppManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
  "schemaVersion": 1,
  "app": "another-app",
  "version": "9.9.9",
  "tag": "another-app-v9.9.9",
  "artifacts": []
}`))
	}))
	defer server.Close()

	provider, err := newCNBUpdaterProvider(server.URL+"/xu756/public", "main", "my-app", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Check(context.Background(), updater.CheckRequest{
		CurrentVersion: "1.0.0",
		Platform:       "windows",
		Arch:           "amd64",
	}); err == nil {
		t.Fatal("accepted manifest for another application")
	}
}

func TestExpectedRuntimeFilename(t *testing.T) {
	for _, tc := range []struct {
		platform, arch, want string
	}{
		{"windows", "amd64", "my-app-windows-amd64.exe"},
		{"linux", "amd64", "my-app-linux-amd64"},
		{"darwin", "arm64", "my-app-darwin-arm64.zip"},
		{"freebsd", "amd64", ""},
	} {
		if got := expectedRuntimeFilename("my-app", tc.platform, tc.arch); got != tc.want {
			t.Fatalf("%s/%s: %q != %q", tc.platform, tc.arch, got, tc.want)
		}
	}
}
