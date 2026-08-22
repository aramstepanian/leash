package mission

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxEvents = 200

type Event struct {
	ID         string    `json:"id"`
	At         time.Time `json:"at"`
	Kind       string    `json:"kind"`
	Agent      string    `json:"agent,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail,omitempty"`
	Result     string    `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
	DurationMs int       `json:"durationMs,omitempty"`
	Paths      []string  `json:"paths,omitempty"`
	Root       string    `json:"root,omitempty"`
}

type Live struct {
	Tool       string    `json:"tool"`
	Detail     string    `json:"detail"`
	Agent      string    `json:"agent,omitempty"`
	Root       string    `json:"root,omitempty"`
	Started    time.Time `json:"started"`
	Status     string    `json:"status"`
	DurationMs int       `json:"durationMs,omitempty"`
	Result     string    `json:"result,omitempty"`
	Error      string    `json:"error,omitempty"`
}

type Failed struct {
	Tool   string `json:"tool"`
	Detail string `json:"detail"`
	Error  string `json:"error"`
	Agent  string `json:"agent,omitempty"`
}

type Snapshot struct {
	Phase    string  `json:"phase"`
	Title    string  `json:"title"`
	Goal     string  `json:"goal,omitempty"`
	Agent    string  `json:"agent,omitempty"`
	Root     string  `json:"root,omitempty"`
	Live     *Live   `json:"live,omitempty"`
	Failed   *Failed `json:"failed,omitempty"`
	Steer    string  `json:"steer,omitempty"`
	Timeline []Event `json:"timeline"`
}

type Log struct {
	mu        sync.Mutex
	events    []Event
	live      *Live
	failed    *Failed
	goal      string
	title     string
	agent     string
	root      string
	steer     string
	interrupt bool
	lastAct   time.Time
}

func (l *Log) Snapshot(waiting, hasBurst bool) Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	tl := append([]Event{}, l.events...)
	st := Snapshot{
		Phase:    l.phase(waiting, hasBurst),
		Title:    l.title,
		Goal:     l.goal,
		Agent:    l.agent,
		Root:     l.root,
		Steer:    l.steer,
		Timeline: tl,
	}
	if st.Title == "" {
		st.Title = folderName(l.root)
	}
	if l.live != nil {
		cp := *l.live
		if cp.Status == "running" || cp.Status == "waiting" {
			cp.DurationMs = int(time.Since(cp.Started) / time.Millisecond)
		}
		st.Live = &cp
	}
	if l.failed != nil {
		cp := *l.failed
		st.Failed = &cp
	}
	if st.Timeline == nil {
		st.Timeline = []Event{}
	}
	return st
}

func (l *Log) phase(waiting, hasBurst bool) string {
	if waiting || (l.live != nil && (l.live.Status == "running" || l.live.Status == "waiting")) {
		return "act"
	}
	if l.failed != nil {
		return "failed"
	}
	if !l.lastAct.IsZero() && time.Since(l.lastAct) < 8*time.Second {
		return "act"
	}
	if hasBurst {
		return "review"
	}
	if l.goal != "" && !hasToolAfterPlan(l.events) {
		return "plan"
	}
	if len(l.events) == 0 {
		return "idle"
	}
	return "review"
}

func hasToolAfterPlan(events []Event) bool {
	planAt := -1
	for i, e := range events {
		if e.Kind == "plan" {
			planAt = i
		}
	}
	if planAt < 0 {
		return len(events) > 0
	}
	for _, e := range events[planAt+1:] {
		if e.Kind == "tool" || e.Kind == "gate" || e.Kind == "diff" {
			return true
		}
	}
	return false
}

func (l *Log) SetSteer(text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.steer = strings.TrimSpace(text)
}

func (l *Log) ArmInterrupt() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.interrupt = true
}

func (l *Log) ConsumePre() (kill bool, steer string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	steer = l.steer
	if l.interrupt {
		l.interrupt = false
		l.steer = ""
		return true, steer
	}
	l.steer = ""
	return false, steer
}

func (l *Log) PeekSteer() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.steer
}

func (l *Log) ClearFailed() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failed = nil
}

func (l *Log) Failed() *Failed {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failed == nil {
		return nil
	}
	cp := *l.failed
	return &cp
}

func (l *Log) Append(ev Event) Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ev.At.IsZero() {
		ev.At = time.Now().UTC().Truncate(time.Second)
	} else {
		ev.At = ev.At.UTC().Truncate(time.Second)
	}
	if ev.Agent != "" {
		l.agent = ev.Agent
	}
	if ev.Root != "" {
		l.root = ev.Root
		if l.title == "" {
			l.title = folderName(ev.Root)
		}
	}
	switch ev.Kind {
	case "plan":
		l.goal = ev.Detail
		if ev.Title != "" && ev.Title != "Plan" {
			l.title = ev.Title
		}
	case "tool", "gate", "diff", "error", "interrupt":
		l.lastAct = time.Now()
	}
	l.events = append(l.events, ev)
	if len(l.events) > maxEvents {
		l.events = l.events[len(l.events)-maxEvents:]
	}
	return ev
}

func (l *Log) StartLive(tool, detail, agent, root, status string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.live = &Live{
		Tool:    tool,
		Detail:  detail,
		Agent:   agent,
		Root:    root,
		Started: now.UTC().Truncate(time.Second),
		Status:  status,
	}
	l.lastAct = now
	if agent != "" {
		l.agent = agent
	}
	if root != "" {
		l.root = root
	}
}

func (l *Log) FinishLive(result, output, errText string, durationMs int) *Live {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.live == nil {
		return nil
	}
	l.live.Status = result
	l.live.Result = clip(output, 2000)
	l.live.Error = clip(errText, 2000)
	if durationMs > 0 {
		l.live.DurationMs = durationMs
	} else {
		l.live.DurationMs = int(time.Since(l.live.Started) / time.Millisecond)
	}
	cp := *l.live
	if result == "error" {
		l.failed = &Failed{Tool: cp.Tool, Detail: cp.Detail, Error: firstNonEmpty(errText, output), Agent: cp.Agent}
	} else {
		l.failed = nil
	}
	return &cp
}

func (l *Log) ClearLive() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.live = nil
}

func folderName(root string) string {
	base := filepath.Base(strings.TrimSpace(root))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "Mission"
	}
	return base
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
