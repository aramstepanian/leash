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

func TestRestoreDoesNotFollowSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a.txt")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "evil")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(time.Minute)
	b := store.Begin(root, "b1")
	b.Touch([]string{target})

	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, target); err != nil {
		t.Fatal(err)
	}

	n, err := store.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("restored %d", n)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Fatalf("dest=%q", got)
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("restore left a symlink")
	}
	evil, _ := os.ReadFile(outside)
	if string(evil) != "secret" {
		t.Fatalf("wrote through symlink: %q", evil)
	}
}

func TestBurstIdleUsesLastTouch(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "a.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(80 * time.Millisecond)
	b1 := store.Begin(root, "1")
	time.Sleep(40 * time.Millisecond)
	b1.Touch([]string{f})
	b2 := store.Begin(root, "2")
	if b1 != b2 {
		t.Fatal("touch should extend the active burst")
	}
	time.Sleep(100 * time.Millisecond)
	b3 := store.Begin(root, "3")
	if b3 == b2 {
		t.Fatal("idle after last touch should start a new burst")
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

func TestTwoRootsKeepSeparateBursts(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	fileA := filepath.Join(rootA, "a.txt")
	fileB := filepath.Join(rootB, "b.txt")
	if err := os.WriteFile(fileA, []byte("a-old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("b-old"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(time.Minute)
	bA := store.Begin(rootA, "a")
	bA.Touch([]string{fileA})
	bB := store.Begin(rootB, "b")
	bB.Touch([]string{fileB})
	if bA == bB {
		t.Fatal("two projects must not share a burst")
	}

	if err := os.WriteFile(fileA, []byte("a-new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("b-new"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := store.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("first undo restored %d, want 1 (most recent root)", n)
	}
	gotB, _ := os.ReadFile(fileB)
	if string(gotB) != "b-old" {
		t.Fatalf("B should restore first: %q", gotB)
	}
	gotA, _ := os.ReadFile(fileA)
	if string(gotA) != "a-new" {
		t.Fatalf("A must stay changed until its own undo: %q", gotA)
	}

	n, err = store.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second undo restored %d", n)
	}
	gotA, _ = os.ReadFile(fileA)
	if string(gotA) != "a-old" {
		t.Fatalf("A after second undo: %q", gotA)
	}
}
