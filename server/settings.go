package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	CloseToTray bool `json:"closeToTray"`
}

type SettingsStore struct {
	mu    sync.RWMutex
	path  string
	value Settings
}

func NewSettingsStore() *SettingsStore {
	store := &SettingsStore{value: Settings{CloseToTray: true}}
	if configDir, err := os.UserConfigDir(); err == nil {
		store.path = filepath.Join(configDir, "edge-desktop-base", "settings.json")
		store.load()
	}
	return store
}

func (s *SettingsStore) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *SettingsStore) SetCloseToTray(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value.CloseToTray = enabled
	return s.saveLocked()
}

func (s *SettingsStore) load() {
	if s.path == "" {
		return
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &s.value)
}

func (s *SettingsStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
