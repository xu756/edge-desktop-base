package server

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/updater"
	gh "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestUpdateAssetSelection(t *testing.T) {
	prefix := appConfig.BinaryName
	assets := []gh.ReleaseAsset{
		{Name: prefix + "-windows-amd64-installer.exe"},
		{Name: prefix + "-windows-arm64.exe"},
		{Name: prefix + "-windows-amd64.exe.sig"},
		{Name: prefix + "-v1.2.3-windows-amd64.exe"},
		{Name: "another-app-windows-amd64.exe"},
		{Name: prefix + "-windows-amd64.exe"},
		{Name: prefix + "-darwin-arm64.zip"},
		{Name: prefix + "-linux-amd64"},
		{Name: prefix + "-linux-amd64.deb"},
		{Name: prefix + "-darwin-arm64.pkg"},
	}
	for _, tc := range []struct {
		platform, arch string
		want           int
	}{
		{"windows", "amd64", 5}, {"darwin", "arm64", 6}, {"linux", "amd64", 7}, {"linux", "arm64", -1},
	} {
		got := matchUpdateAsset(updater.CheckRequest{Platform: tc.platform, Arch: tc.arch}, assets)
		if got != tc.want {
			t.Fatalf("%s/%s: %d != %d", tc.platform, tc.arch, got, tc.want)
		}
	}
	if got := matchUpdateAsset(updater.CheckRequest{Platform: "windows", Arch: "amd64"}, assets[:5]); got != -1 {
		t.Fatal("selected an installer, sidecar, legacy versioned asset, or unrelated asset")
	}
}

func TestOnlySystemInstallersAreNotUpdatePayloads(t *testing.T) {
	prefix := appConfig.BinaryName
	for _, tc := range []struct{ platform, arch, ext string }{
		{"linux", "amd64", ".deb"}, {"darwin", "arm64", ".pkg"},
	} {
		asset := gh.ReleaseAsset{Name: prefix + "-" + tc.platform + "-" + tc.arch + tc.ext}
		if got := matchUpdateAsset(updater.CheckRequest{Platform: tc.platform, Arch: tc.arch}, []gh.ReleaseAsset{asset}); got != -1 {
			t.Fatal("installer selected", asset.Name)
		}
	}
}

func TestSystemInstallUpdatePolicy(t *testing.T) {
	for _, tc := range []struct {
		platform, executable string
		managed              bool
	}{
		{"linux", "/usr/bin/my-app", true},
		{"linux", "/home/user/my-app", false},
		{"darwin", "/Applications/my-app.app/Contents/MacOS/my-app", true},
		{"darwin", "/Users/user/my-app.app/Contents/MacOS/my-app", false},
		{"windows", `C:\Users\user\AppData\Local\Programs\my-app\my-app.exe`, false},
	} {
		if got := systemInstallHint(tc.platform, tc.executable, "my-app") != ""; got != tc.managed {
			t.Fatalf("%s: managed=%v", tc.executable, got)
		}
	}
}
