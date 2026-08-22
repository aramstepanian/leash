package policy

import "testing"

func TestOutcomeDangerous(t *testing.T) {
	a := Assess("Bash", "/proj", "/proj", map[string]any{"command": "rm -rf ./dist"}, nil)
	if a.Title != "Delete dist" {
		t.Fatalf("rm -rf title: %q", a.Title)
	}
	a = Assess("Bash", "/proj", "/proj", map[string]any{"command": "git push --force origin main"}, nil)
	if a.Title != "Force-push" {
		t.Fatalf("force-push title: %q", a.Title)
	}
	a = Assess("Read", "/proj", "/proj", map[string]any{"file_path": "/proj/.env"}, nil)
	if a.Title != "Read .env" {
		t.Fatalf("secret read title: %q", a.Title)
	}
	a = Assess("Write", "/proj", "/proj", map[string]any{"file_path": "/proj/src/main.ts"}, nil)
	if a.Title != "Write src/main.ts" {
		t.Fatalf("write title: %q", a.Title)
	}
}

func TestQuietInspection(t *testing.T) {
	if !Quiet("Read", "/proj/README.md") {
		t.Fatal("read should be quiet")
	}
	if Quiet("Read", "/proj/.env") {
		t.Fatal("secret read is a gate, not quiet")
	}
	if !Quiet("Bash", "git status") {
		t.Fatal("git status should be quiet")
	}
	if Quiet("Bash", "npm test") {
		t.Fatal("npm test is work")
	}
	if Quiet("Write", "/proj/a.ts") {
		t.Fatal("write is not quiet")
	}
}

func TestRemoveRule(t *testing.T) {
	rules := []Rule{
		{Tool: "Bash", Pattern: "npm test", Root: "/a"},
		{Tool: "Bash", Pattern: "npm test", Root: "/b"},
	}
	got := RemoveRule(rules, "Bash", "npm test", "/a")
	if len(got) != 1 || got[0].Root != "/b" {
		t.Fatalf("%+v", got)
	}
}
