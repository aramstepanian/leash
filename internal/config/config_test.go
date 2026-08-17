package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leashapp/leash/internal/policy"
)

func TestEnsurePersistsToken(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEASH_HOME", dir)
	f, err := Ensure()
	if err != nil {
		t.Fatal(err)
	}
	if f.Token == "" {
		t.Fatal("expected token")
	}
	if f.Port != 17332 {
		t.Fatalf("port %d", f.Port)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != f.Token {
		t.Fatalf("token not persisted: %q vs %q", loaded.Token, f.Token)
	}
}

func TestAlwaysAllowCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEASH_HOME", dir)
	var rules []policy.Rule
	for i := 0; i < maxAlways+50; i++ {
		rules = append(rules, policy.Rule{Tool: "Bash", Pattern: "cmd"})
	}
	if err := Save(File{Port: 17332, Token: "t", AlwaysAllow: rules}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.AlwaysAllow) != maxAlways {
		t.Fatalf("got %d rules", len(loaded.AlwaysAllow))
	}
}
