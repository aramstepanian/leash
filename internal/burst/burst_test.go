package burst

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTouchAndUndo(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(src, "a.txt")
	if err := os.WriteFile(a, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(time.Minute)
	b := store.Begin(root, "b1")
	b.Touch([]string{a, filepath.Join(src, "new.txt")})

	if err := os.WriteFile(a, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "new.txt"), []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := store.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("restored %d, want 2", n)
	}
	got, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("a.txt=%q", got)
	}
	if _, err := os.Stat(filepath.Join(src, "new.txt")); !os.IsNotExist(err) {
		t.Fatal("new.txt should be removed")
	}
}

func TestSkipNodeModules(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "node_modules", "pkg", "index.js")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewStore(time.Minute).Begin(root, "b")
	b.Touch([]string{p})
	if b.FileCount != 0 {
		t.Fatalf("should skip node_modules, got %d", b.FileCount)
	}
}

func TestNothingToUndo(t *testing.T) {
	_, err := NewStore(time.Minute).Undo()
	if err == nil {
		t.Fatal("expected error")
	}
}
