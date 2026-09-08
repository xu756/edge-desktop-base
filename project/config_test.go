package project

import (
	"encoding/json"
	"testing"
)

func TestProjectConfigValidation(t *testing.T) {
	if _, err := Parse(source); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"binaryName", "../bad"}, {"configDirName", "../bad"}, {"legacyConfigDirNames", []string{"../bad"}},
		{"name", `bad$injection`}, {"identifier", "bad id"},
		{"updateRepositoryURL", "http://cnb.cool/xu756/public"},
		{"updateRepositoryURL", "https://cnb.cool"},
		{"updateRepositoryURL", "https://user:password@cnb.cool/xu756/public"},
		{"updateBranch", "../main"}, {"updateBranch", "main..bad"},
		{"defaultAPIAddress", "0.0.0.0:19876"}, {"devVersion", "1.2.3-beta.01"}, {"devVersion", "65536.0.0"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			var fields map[string]any
			_ = json.Unmarshal(source, &fields)
			fields[tc.key] = tc.value
			data, _ := json.Marshal(fields)
			if _, err := Parse(data); err == nil {
				t.Fatal("accepted invalid config", tc)
			}
		})
	}
}

func TestPrereleaseNumericVersion(t *testing.T) {
	got, err := NumericVersion("1.2.3-beta.1+build.4")
	if err != nil || got != "1.2.3" {
		t.Fatal(got, err)
	}
}
