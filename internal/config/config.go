package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/leashapp/leash/internal/atomicfile"
	"github.com/leashapp/leash/internal/policy"
)

const maxAlways = 200

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
	normalize(&f)
	return f, nil
}

func Save(f File) error {
	normalize(&f)
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(Path(), data, 0o600)
}

func Ensure() (File, error) {
	f, err := Load()
	if err != nil {
		return f, err
	}
	if f.Token == "" {
		f.Token = newToken()
	}
	if err := Save(f); err != nil {
		return f, err
	}
	return f, nil
}

func normalize(f *File) {
	if f.Port == 0 {
		f.Port = 17332
	}
	if f.AlwaysAllow == nil {
		f.AlwaysAllow = []policy.Rule{}
	}
	if len(f.AlwaysAllow) > maxAlways {
		f.AlwaysAllow = f.AlwaysAllow[len(f.AlwaysAllow)-maxAlways:]
	}
}

func defaultFile() File {
	return File{
		Port:        17332,
		Token:       newToken(),
		AlwaysAllow: []policy.Rule{},
	}
}

func newToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "leash-dev-token"
	}
	return hex.EncodeToString(b)
}
