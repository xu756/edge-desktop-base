package desktop

import (
	"changeme/server/settings"
	"testing"
)

func TestAutostartArguments(t *testing.T) {
	for _, show := range []bool{true, false} {
		opts := applicationAutostartOptions(show)
		if opts.Identifier != AppIdentifier || opts.Arguments[0] != "--autostart" {
			t.Fatal(opts)
		}
		if show && len(opts.Arguments) != 1 {
			t.Fatal(opts)
		}
		if !show && (len(opts.Arguments) != 2 || opts.Arguments[1] != "--hidden") {
			t.Fatal(opts)
		}
	}
}

func TestStartupWindowVisibility(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         []string
		show, hidden bool
	}{
		{"manual stays visible", nil, false, false},
		{"explicit hidden", []string{"--hidden"}, true, true},
		{"autostart background", []string{"--autostart"}, false, true},
		{"autostart visible", []string{"--autostart"}, true, false},
		{"saved preference overrides stale hidden argument", []string{"--autostart", "--hidden"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldStartHidden(tc.args, settings.Settings{AutoStartShowWindow: tc.show}); got != tc.hidden {
				t.Fatalf("hidden=%v, want %v", got, tc.hidden)
			}
		})
	}
}
