package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/leashapp/leash/internal/policy"
)

type File struct {
	Port        int           `json:"port"`
	Token       string        `json:"token"`
	WatchRoot   string        `json:"watchRoot"`
	AlwaysAllow []policy.Rule `json:"alwaysAllow"`
}

func Dir() string {
	if d := os.Getenv("LEASH_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".leash"
	}
	return filepath.Join(home, ".leash")
}

func Path() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (File, error) {
	var f File
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return defaultFile(), nil
		}
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, err
	}
	if f.Port == 0 {
		f.Port = 17332
	}
	if f.Token == "" {
		f.Token = newToken()
	}
	if f.AlwaysAllow == nil {
		f.AlwaysAllow = []policy.Rule{}
	}
	return f, nil
}

func Save(f File) error {
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0o600)
}

func defaultFile() File {
	return File{
		Port:        17332,
		Token:       newToken(),
		AlwaysAllow: []policy.Rule{},
	}
}

func Ensure() (File, error) {
	f, err := Load()
	if err != nil {
		return f, err
	}
	if _, err := os.Stat(Path()); os.IsNotExist(err) {
		if err := Save(f); err != nil {
			return f, err
		}
	}
	return f, nil
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "leash-dev-token"
	}
	return hex.EncodeToString(b)
}
