package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

const (
	settingsFile = "consola.json"
	maxRecent    = 8
)

// settings remembers the teacher's choices between runs. It holds nothing
// secret and lives next to the identity.
type settings struct {
	RecentChannels []string `json:"recent_channels"`
	ConsoleName    string   `json:"console_name"`
}

func loadSettings(dir string) settings {
	var s settings
	data, err := os.ReadFile(filepath.Join(dir, settingsFile))
	if err == nil {
		_ = json.Unmarshal(data, &s) // a damaged file just starts empty
	} else if !errors.Is(err, fs.ErrNotExist) {
		return settings{}
	}
	return s
}

func saveSettings(dir string, s settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, settingsFile), data, 0o600)
}

// remember puts channel first in the recent list.
func (s *settings) remember(channel, name string) {
	s.RecentChannels = slices.DeleteFunc(s.RecentChannels, func(c string) bool { return c == channel })
	s.RecentChannels = append([]string{channel}, s.RecentChannels...)
	if len(s.RecentChannels) > maxRecent {
		s.RecentChannels = s.RecentChannels[:maxRecent]
	}
	s.ConsoleName = name
}
