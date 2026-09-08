package server

import (
	"github.com/wailsapp/wails/v3/pkg/updater"
	gh "github.com/wailsapp/wails/v3/pkg/updater/providers/github"
	"testing"
)

func TestUpdateAssetSelection(t *testing.T) {
	prefix := appConfig.BinaryName + "-v1.2.3"
	assets := []gh.ReleaseAsset{
		{Name: prefix + "-windows-amd64-installer.exe"},
		{Name: prefix + "-windows-arm64.exe"},
		{Name: prefix + "-windows-amd64.exe.sig"},
		{Name: "another-app-v1.2.3-windows-amd64.exe"},
		{Name: prefix + "-windows-amd64.exe"},
		{Name: prefix + "-darwin-arm64.zip"},
		{Name: prefix + "-linux-amd64"},
	}
	for _, tc := range []struct {
		platform, arch string
		want           int
	}{
		{"windows", "amd64", 4}, {"darwin", "arm64", 5}, {"linux", "amd64", 6}, {"linux", "arm64", -1},
	} {
		got := matchUpdateAsset(updater.CheckRequest{Platform: tc.platform, Arch: tc.arch}, assets)
		if got != tc.want {
			t.Fatalf("%s/%s: %d != %d", tc.platform, tc.arch, got, tc.want)
		}
	}
	if got := matchUpdateAsset(updater.CheckRequest{Platform: "windows", Arch: "amd64"}, assets[:4]); got != -1 {
		t.Fatal("selected an installer or unrelated asset")
	}
}
