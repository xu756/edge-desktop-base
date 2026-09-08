package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

var (
	infoVersionPattern     = regexp.MustCompile(`(?m)^  version: "[^"]+"`)
	semanticVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
)

func main() {
	rawVersion := strings.TrimSpace(os.Getenv("VERSION"))
	if rawVersion == "" {
		panic("VERSION is required; release builds must be triggered by a v* git tag")
	}

	version := strings.TrimPrefix(rawVersion, "v")
	if !semanticVersionPattern.MatchString(version) {
		panic(fmt.Sprintf("invalid semantic version: %s", rawVersion))
	}

	commit := strings.TrimSpace(os.Getenv("COMMIT"))
	if commit == "" {
		commit = "unknown"
	}
	if strings.ContainsAny(commit, "\"\r\n") {
		panic("invalid commit metadata")
	}

	buildTime := time.Now().UTC().Format(time.RFC3339)
	content := fmt.Sprintf(`package server

const (
	AppName           = "Edge Desktop Base"
	AppDescription    = "Reusable Wails v3 desktop application foundation"
	AppIdentifier     = "io.github.xu756.edge-desktop-base"
	UpdateRepository  = "xu756/edgeinfer-node-test"
	DefaultAPIAddress = "127.0.0.1:19876"
)

var (
	Version   = %q
	Commit    = %q
	BuildTime = %q
)
`, version, commit, buildTime)

	if err := os.WriteFile("server/buildinfo.go", []byte(content), 0o644); err != nil {
		panic(err)
	}

	configPath := "build/config.yml"
	config, err := os.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	updated := infoVersionPattern.ReplaceAllString(string(config), `  version: "`+version+`"`)
	if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
		panic(err)
	}
}
