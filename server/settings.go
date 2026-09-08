package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Settings struct {
	AutoCheckUpdates     bool `json:"autoCheckUpdates"`
	AutoDownloadUpdates  bool `json:"autoDownloadUpdates"`
	UpdateIntervalHours  int  `json:"updateIntervalHours"`
	CloseToTray          bool `json:"closeToTray"`
	AutoStartShowWindow  bool `json:"autoStartShowWindow"`
}

type SettingsStore struct {
	mu      sync.RWMutex
	path    string
	value   Settings
	loadErr error
}

func NewSettingsStore() *SettingsStore {
	home, err := os.UserHomeDir()
	if err != nil {
		return &SettingsStore{value: defaultSettings(), loadErr: err}
	}
	path := filepath.Join(home, ".config", appConfig.ConfigDirName, "settings.json")
	oldDir, _ := os.UserConfigDir()
	var candidates []string
	names := append([]string{appConfig.ConfigDirName}, appConfig.LegacyConfigDirNames...)
	for _, name := range names {
		if oldDir != "" {
			candidates = append(candidates, filepath.Join(oldDir, name, "settings.json"))
		}
		candidates = append(candidates, filepath.Join(home, ".config", name, "settings.json"))
	}
	return migratedSettingsStore(path, candidates)
}

func defaultSettings() Settings {
	return Settings{
		CloseToTray:         true,
		AutoCheckUpdates:    true,
		AutoDownloadUpdates: false,
		UpdateIntervalHours: 6,
	}
}

// Copy only on first use of the new path; never remove or overwrite old settings.
func migratedSettingsStore(path string, candidates []string) *SettingsStore {
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return newSettingsStore(path)
	}
	for _, old := range candidates {
		if filepath.Clean(old) == filepath.Clean(path) {
			continue
		}
		data, err := os.ReadFile(old)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err == nil {
			err = os.MkdirAll(filepath.Dir(path), 0700)
		}
		if err == nil {
			var f *os.File
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if errors.Is(err, os.ErrExist) {
				return newSettingsStore(path)
			}
			if err == nil {
				_, err = f.Write(data)
				if err == nil {
					err = f.Sync()
				}
				err = errors.Join(err, f.Close())
				if err != nil {
					_ = os.Remove(path)
				}
			}
		}
		if err != nil {
			return &SettingsStore{path: path, value: defaultSettings(), loadErr: fmt.Errorf("迁移配置 %s: %w", old, err)}
		}
		return newSettingsStore(path)
	}
	return newSettingsStore(path)
}

func newSettingsStore(path string) *SettingsStore {
	s := &SettingsStore{path: path, value: defaultSettings()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.loadErr = s.saveLocked(s.value)
		return s
	}
	if err != nil {
		s.loadErr = err
		return s
	}
	value := s.value
	if err = json.Unmarshal(data, &value); err != nil {
		s.loadErr = fmt.Errorf("读取配置 %s: %w", path, err)
		return s
	}
	if value.UpdateIntervalHours < 1 || value.UpdateIntervalHours > 168 {
		s.loadErr = errors.New("updateIntervalHours 必须介于 1 和 168 之间")
		return s
	}
	s.value = value
	return s
}

func (s *SettingsStore) Get() Settings { s.mu.RLock(); defer s.mu.RUnlock(); return s.value }
func (s *SettingsStore) Path() string  { return s.path }
func (s *SettingsStore) Error() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.loadErr != nil {
		return s.loadErr.Error()
	}
	return ""
}
func (s *SettingsStore) SetCloseToTray(enabled bool) error {
	return s.change(func(v *Settings) { v.CloseToTray = enabled })
}
func (s *SettingsStore) SetAutoStartShowWindow(enabled bool) error {
	return s.change(func(v *Settings) { v.AutoStartShowWindow = enabled })
}
func (s *SettingsStore) SetAutoCheckUpdates(enabled bool) error {
	return s.change(func(v *Settings) { v.AutoCheckUpdates = enabled })
}
func (s *SettingsStore) SetAutoDownloadUpdates(enabled bool) error {
	return s.change(func(v *Settings) { v.AutoDownloadUpdates = enabled })
}
func (s *SettingsStore) change(edit func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Preserve unreadable or malformed files for recovery instead of overwriting them.
	if s.loadErr != nil {
		return s.loadErr
	}
	next := s.value
	edit(&next)
	if err := s.saveLocked(next); err != nil {
		return err
	}
	s.value = next
	return nil
}
func (s *SettingsStore) saveLocked(value Settings) error {
	if s.path == "" {
		return errors.New("用户配置目录不可用")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.path)
}
