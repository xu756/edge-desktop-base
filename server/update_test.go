package server

import "testing"

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
