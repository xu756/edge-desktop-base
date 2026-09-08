package main

import (
	"changeme/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateRenamedProject(t *testing.T) {
	for _, newline := range []struct{ name, value string }{
		{"LF", "\n"}, {"CRLF", "\r\n"},
	} {
		t.Run(newline.name, func(t *testing.T) { testGenerateRenamedProject(t, newline.value) })
	}
}

func testGenerateRenamedProject(t *testing.T, newline string) {
	// Work on a fixture, never rewrite the actual project from a test.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, path := range []string{"build/windows/wails.exe.manifest", "build/windows/nsis/wails_tools.nsh", "build/windows/msix/app_manifest.xml", "build/windows/msix/template.xml", "build/ios/Info.plist", "build/ios/Info.dev.plist", "build/ios/project.pbxproj", "build/ios/LaunchScreen.storyboard", "build/android/app/build.gradle", "build/android/settings.gradle", "build/android/app/src/main/res/values/strings.xml", "build/config.yml", "build/windows/info.json", "build/darwin/Info.plist", "build/darwin/Info.dev.plist", "frontend/index.html", "build/linux/nfpm/nfpm.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n", newline))
		dest := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"server/desktop", "build/windows/nsis"} {
		if err := os.MkdirAll(filepath.Join(dir, p), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv("VERSION", "1.2.3-beta.1")
	t.Setenv("COMMIT", "test")
	t.Setenv("GITHUB_ENV", "")
	c := project.Load()
	c.Name = "New Product"
	c.BinaryName = "new-product"
	c.Identifier = "io.example.new-product"
	if err := generate(c, true); err != nil {
		t.Fatal(err)
	}
	checks := map[string][]string{
		"build/windows/wails.exe.manifest":     {`name="io.example.new-product" version="1.2.3.0"`, `name="Microsoft.Windows.Common-Controls" version="6.0.0.0"`},
		"build/ios/Info.dev.plist":             {"io.example.new-product.dev", "New Product (Dev)"},
		"build/ios/project.pbxproj":            {`PRODUCT_NAME = "new-product"`, `bin/new-product.a`},
		"build/android/app/build.gradle":       {`applicationId "io.example.new_product"`, `namespace 'com.wails.app'`},
		"build/windows/msix/app_manifest.xml":  {`Executable="new-product.exe"`, `Name="io.example.new-product"`},
		"server/desktop/version_generated.go":  {`"1.2.3-beta.1"`},
		"build/windows/nsis/app_generated.nsh": {`INFO_PRODUCTNAME "New Product"`, `INFO_PROJECTNAME "new-product"`, `INFO_PRODUCTVERSION "1.2.3"`, `APP_IDENTIFIER "io.example.new-product"`},
		"build/darwin/Info.plist":              {"<string>new-product</string>", "<string>New Product</string>", "<string>io.example.new-product</string>"},
		"build/linux/new-product.desktop":      {"Name=New Product", "Exec=new-product"},
		"build/linux/nfpm/nfpm.yaml":           {"./bin/new-product", "/usr/bin/new-product", "/usr/share/applications/new-product.desktop", `version: "1.2.3-beta.1"`, "mode: 0755", "mode: 0644"},
		"frontend/index.html":                  {"<title>New Product</title>"},
	}
	for path, needles := range checks {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range needles {
			if !strings.Contains(string(data), needle) {
				t.Errorf("%s missing %s", path, needle)
			}
		}
	}
	// Repeated replacement of separate fields must preserve the checkout's
	// newline style, including the YAML block that failed on Windows CI.
	for _, path := range []string{"build/config.yml", "build/linux/nfpm/nfpm.yaml", "build/darwin/Info.plist", "frontend/index.html"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if newline == "\r\n" {
			if !strings.Contains(text, "\r\n") || strings.Contains(strings.ReplaceAll(text, "\r\n", ""), "\n") {
				t.Errorf("%s: expected consistent CRLF", path)
			}
		} else if strings.Contains(text, "\r") {
			t.Errorf("%s: expected LF", path)
		}
	}
	// A second rename must not rely on original scaffold strings.
	c.Name = "Another Product"
	c.BinaryName = "another-product"
	c.Identifier = "io.example.another-product"
	if err := generate(c, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("build/linux/new-product.desktop"); !os.IsNotExist(err) {
		t.Fatal("stale desktop file remains")
	}
	data, err := os.ReadFile("build/ios/project.pbxproj")
	if err != nil || strings.Contains(string(data), "new-product") {
		t.Fatal("Xcode rename left stale references", err)
	}
	t.Setenv("VERSION", "")
	if err := generate(c, true); err == nil {
		t.Fatal("release accepted missing version")
	}
}
