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
	for _, path := range []string{"build/config.yml", "build/windows/info.json", "build/darwin/Info.plist", "build/darwin/Info.dev.plist", "frontend/index.html", "build/linux/nfpm/nfpm.yaml"} {
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
	for _, p := range []string{"server", "build/windows/nsis"} {
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
		"server/version_generated.go":          {`"1.2.3-beta.1"`},
		"build/windows/nsis/app_generated.nsh": {`INFO_PRODUCTNAME "New Product"`, `INFO_PROJECTNAME "new-product"`, `INFO_PRODUCTVERSION "1.2.3"`, `APP_IDENTIFIER "io.example.new-product"`},
		"build/darwin/Info.plist":              {"<string>new-product</string>", "<string>New Product</string>", "<string>io.example.new-product</string>"},
		"build/linux/new-product.desktop":      {"Name=New Product", "Exec=new-product"},
		"build/linux/nfpm/nfpm.yaml":           {"./bin/new-product", "/usr/share/applications/new-product.desktop"},
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
	t.Setenv("VERSION", "")
	if err := generate(c, true); err == nil {
		t.Fatal("release accepted missing version")
	}
}
