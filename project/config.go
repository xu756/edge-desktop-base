// Package project embeds the developer-owned application identity, not user settings.
package project

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	modsemver "golang.org/x/mod/semver"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

//go:embed app.json
var source []byte

type Config struct {
	Name                 string   `json:"name"`
	BinaryName           string   `json:"binaryName"`
	Identifier           string   `json:"identifier"`
	ConfigDirName        string   `json:"configDirName"`
	LegacyConfigDirNames []string `json:"legacyConfigDirNames"`
	Description          string   `json:"description"`
	CompanyName          string   `json:"companyName"`
	Copyright            string   `json:"copyright"`
	UpdateRepositoryURL  string   `json:"updateRepositoryURL"`
	UpdateBranch         string   `json:"updateBranch"`
	DefaultAPIAddress    string   `json:"defaultAPIAddress"`
	DevVersion           string   `json:"devVersion"`
}

var slug = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
var identifier = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*(\.[a-zA-Z][a-zA-Z0-9-]*)+$`)
var repositorySegment = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
var gitBranch = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,127}$`)
var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func Parse(data []byte) (Config, error) {
	var c Config
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}
	// Also reject trailing JSON values.
	if !json.Valid(data) {
		return c, fmt.Errorf("invalid project JSON")
	}
	for _, value := range append([]string{c.BinaryName, c.ConfigDirName}, c.LegacyConfigDirNames...) {
		if !slug.MatchString(value) {
			return c, fmt.Errorf("invalid filename/directory slug: %q", value)
		}
	}
	for _, value := range []string{c.Name, c.Description, c.CompanyName, c.Copyright} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\"$\\/\r\n\x00<>:|?*") {
			return c, fmt.Errorf("invalid application metadata: %q", value)
		}
	}
	if !identifier.MatchString(c.Identifier) {
		return c, fmt.Errorf("invalid identifier")
	}
	if err := validateUpdateRepositoryURL(c.UpdateRepositoryURL); err != nil {
		return c, err
	}
	if !validGitBranch(c.UpdateBranch) {
		return c, fmt.Errorf("invalid updateBranch: %q", c.UpdateBranch)
	}
	if _, err := NumericVersion(c.DevVersion); err != nil {
		return c, err
	}
	host, _, err := net.SplitHostPort(c.DefaultAPIAddress)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return c, fmt.Errorf("defaultAPIAddress must use a loopback IP and port")
	}
	return c, nil
}

func validateUpdateRepositoryURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("updateRepositoryURL must be a public HTTPS CNB repository URL")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return fmt.Errorf("updateRepositoryURL must include the CNB repository path")
	}
	for _, part := range parts {
		if !repositorySegment.MatchString(part) || part == "." || part == ".." {
			return fmt.Errorf("invalid CNB repository path segment: %q", part)
		}
	}
	return nil
}

func validGitBranch(value string) bool {
	return gitBranch.MatchString(value) &&
		!strings.Contains(value, "..") &&
		!strings.Contains(value, "//") &&
		!strings.HasSuffix(value, "/") &&
		!strings.HasSuffix(value, ".")
}

// NumericVersion strips prerelease/build metadata for Windows version resources.
func NumericVersion(version string) (string, error) {
	if !semver.MatchString(version) || !modsemver.IsValid("v"+version) {
		return "", fmt.Errorf("invalid semantic version: %q", version)
	}
	numeric := strings.SplitN(strings.SplitN(version, "+", 2)[0], "-", 2)[0]
	for _, part := range strings.Split(numeric, ".") {
		if _, err := strconv.ParseUint(part, 10, 16); err != nil {
			return "", fmt.Errorf("version component exceeds Windows resource range: %s", part)
		}
	}
	return numeric, nil
}

func Load() Config {
	c, err := Parse(source)
	if err != nil {
		panic(fmt.Errorf("project/app.json: %w", err))
	}
	return c
}
