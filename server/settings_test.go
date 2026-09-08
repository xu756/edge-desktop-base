package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsDefaultsAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "settings.json")
	s := newSettingsStore(path)
	if s.Error() != "" {
		t.Fatal(s.Error())
	}
	defaults := s.Get()
	if !defaults.CloseToTray || !defaults.AutoCheckUpdates || defaults.AutoDownloadUpdates || defaults.UpdateIntervalHours != 6 {
		t.Fatal(defaults)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutoStartShowWindow(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloseToTray(false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutoCheckUpdates(false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutoDownloadUpdates(true); err != nil {
		t.Fatal(err)
	}
	next := newSettingsStore(path)
	if next.Get().CloseToTray || !next.Get().AutoStartShowWindow || next.Get().AutoCheckUpdates || !next.Get().AutoDownloadUpdates {
		t.Fatal(next.Get())
	}
}

func TestSettingsLegacyAndMalformed(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"legacy", `{"closeToTray":false}`, true},
		{"broken", `{"closeToTray":false,`, false},
		{"invalid interval", `{"updateIntervalHours":0}`, false},
		{"wrong type", `{"autoStartShowWindow":"yes"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			s := newSettingsStore(path)
			if tc.valid {
				value := s.Get()
				if s.Error() != "" || value.CloseToTray || value.AutoStartShowWindow || !value.AutoCheckUpdates || value.AutoDownloadUpdates || value.UpdateIntervalHours != 6 {
					t.Fatal(value, s.Error())
				}
			} else {
				if s.Error() == "" {
					t.Fatal("expected load error")
				}
				if err := s.SetCloseToTray(false); err == nil {
					t.Fatal("must not overwrite invalid config")
				}
				data, err := os.ReadFile(path)
				if err != nil || string(data) != tc.data {
					t.Fatal("original config lost", err)
				}
				if !s.Get().CloseToTray {
					t.Fatal("partially applied corrupt config")
				}
			}
		})
	}
}

func TestSettingsWriteFailureKeepsMemory(t *testing.T) {
	s := newSettingsStore(filepath.Join(t.TempDir(), "settings.json"))
	before := s.Get()
	// Rename cannot replace a nonempty directory, even when tests run as root.
	s.path = t.TempDir()
	if err := os.WriteFile(filepath.Join(s.path, "occupied"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloseToTray(false); err == nil {
		t.Fatal("expected save failure")
	}
	if s.Get() != before {
		t.Fatal("memory changed after failed save")
	}
}

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
			if got := shouldStartHidden(tc.args, Settings{AutoStartShowWindow: tc.show}); got != tc.hidden {
				t.Fatalf("hidden=%v, want %v", got, tc.hidden)
			}
		})
	}
}

func TestSettingsMigration(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "legacy.json")
	data := []byte(`{"closeToTray":false,"autoStartShowWindow":true}`)
	if err := os.WriteFile(old, data, 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".config", "renamed-app", "settings.json")
	store := migratedSettingsStore(path, []string{filepath.Join(root, "missing"), old})
	if store.Error() != "" || store.Get().CloseToTray || !store.Get().AutoStartShowWindow || !store.Get().AutoCheckUpdates || store.Get().AutoDownloadUpdates {
		t.Fatal(store.Get(), store.Error())
	}
	if oldData, err := os.ReadFile(old); err != nil || string(oldData) != string(data) {
		t.Fatal("legacy file changed", err)
	}
	if err := store.SetCloseToTray(true); err != nil {
		t.Fatal(err)
	}
	if next := migratedSettingsStore(path, []string{old}); !next.Get().CloseToTray {
		t.Fatal("overwrote existing destination")
	}
}

func TestCorruptLegacySettingsPreserved(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join(root, "legacy.json")
	if err := os.WriteFile(old, []byte(`broken`), 0600); err != nil {
		t.Fatal(err)
	}
	store := migratedSettingsStore(filepath.Join(root, "new", "settings.json"), []string{old})
	if store.Error() == "" {
		t.Fatal("expected corrupt config error")
	}
	if err := store.SetCloseToTray(false); err == nil {
		t.Fatal("overwrote corrupt config")
	}
}

func TestUserSettingsUseHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-override"))
	t.Setenv("APPDATA", filepath.Join(home, "appdata"))
	store := NewSettingsStore()
	want := filepath.Join(home, ".config", appConfig.ConfigDirName, "settings.json")
	if store.Error() != "" || store.Path() != want {
		t.Fatal(store.Path(), store.Error())
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
}
