package mission

import "testing"

func TestPhasePlanActReview(t *testing.T) {
	var log Log
	if log.Snapshot(false, false).Phase != "idle" {
		t.Fatal(log.Snapshot(false, false).Phase)
	}
	log.Reset()
	if log.Snapshot(false, false).Phase != "idle" {
		t.Fatal("reset")
	}
	log.Append(Event{Kind: "plan", Title: "Fix auth", Detail: "read then edit", Root: "/proj"})
	if got := log.Snapshot(false, false).Phase; got != "plan" {
		t.Fatalf("after plan: %s", got)
	}
	log.StartLive("Bash", "git status", "Cursor", "/proj", "running", "Check git status")
	if got := log.Snapshot(false, false).Phase; got != "act" {
		t.Fatalf("live: %s", got)
	}
	log.ClearLive()
	log.Append(Event{Kind: "tool", Title: "git status", Result: "ok"})
	log.StartLive("Bash", "npm test", "Cursor", "/proj", "running", "Run npm test")
	log.FinishLive("error", "", "exit status 1", 40)
	if got := log.Snapshot(false, false).Phase; got != "failed" {
		t.Fatalf("failed: %s", got)
	}
}

func TestConsumeInterrupt(t *testing.T) {
	var log Log
	log.SetSteer("stop")
	log.ArmInterrupt()
	kill, steer := log.ConsumePre()
	if !kill || steer != "stop" {
		t.Fatalf("kill=%v steer=%q", kill, steer)
	}
	kill, steer = log.ConsumePre()
	if kill || steer != "" {
		t.Fatalf("second consume kill=%v steer=%q", kill, steer)
	}
}
