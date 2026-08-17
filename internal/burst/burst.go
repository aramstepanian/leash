package burst

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/leashapp/leash/internal/policy"
)

const (
	maxFiles = 400
	maxBytes = 40 << 20 // 40MB
	maxFile  = 2 << 20  // 2MB per file
)

// Origin is the pre-burst version of a path.
type Origin struct {
	Rel     string
	Existed bool
	Mode    os.FileMode
	Data    []byte
}

// Burst is one agent episode: files touched until idle.
type Burst struct {
	ID        string    `json:"id"`
	Started   time.Time `json:"started"`
	Root      string    `json:"root"`
	FileCount int       `json:"fileCount"`
	mu        sync.Mutex
	files     map[string]*Origin
	bytes     int
}

type Store struct {
	mu     sync.Mutex
	active *Burst
	last   *Burst
	idle   time.Duration
}

func NewStore(idle time.Duration) *Store {
	if idle <= 0 {
		idle = 3 * time.Minute
	}
	return &Store{idle: idle}
}

func (s *Store) Active() *Burst {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

func (s *Store) Last() *Burst {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return s.active
	}
	return s.last
}

func (s *Store) Begin(root, id string) *Burst {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && sameRoot(s.active.Root, root) && time.Since(s.active.Started) < s.idle {
		return s.active
	}
	if s.active != nil {
		s.last = s.active
	}
	b := &Burst{
		ID:      id,
		Started: time.Now(),
		Root:    root,
		files:   map[string]*Origin{},
	}
	s.active = b
	return b
}

func (s *Store) CloseIfIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && time.Since(s.active.Started) >= s.idle {
		s.last = s.active
		s.active = nil
	}
}

func (s *Store) Undo() (int, error) {
	s.mu.Lock()
	b := s.active
	if b == nil {
		b = s.last
	}
	s.mu.Unlock()
	if b == nil {
		return 0, fmt.Errorf("nothing to undo")
	}
	n, err := b.Restore()
	if err != nil {
		return n, err
	}
	s.mu.Lock()
	if s.active == b {
		s.active = nil
	}
	s.last = nil
	s.mu.Unlock()
	return n, nil
}

// Touch records the original bytes of paths under root, once per path.
func (b *Burst) Touch(paths []string) {
	if b == nil || b.Root == "" {
		return
	}
	root, err := filepath.Abs(b.Root)
	if err != nil {
		return
	}
	for _, p := range paths {
		b.touchOne(root, p)
	}
}

func (b *Burst) touchOne(root, p string) {
	if p == "" {
		return
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	if policy.SkipSnapshot(rel) {
		return
	}

	info, err := os.Lstat(abs)
	if err == nil && info.IsDir() {
		_ = filepath.Walk(abs, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				if fi != nil && policy.SkipSnapshot(fi.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			b.touchFile(root, path)
			return nil
		})
		return
	}
	b.touchFile(root, abs)
}

func (b *Burst) touchFile(root, abs string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.files) >= maxFiles || b.bytes >= maxBytes {
		return
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	if policy.SkipSnapshot(rel) {
		return
	}
	if _, ok := b.files[rel]; ok {
		return
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			b.files[rel] = &Origin{Rel: rel, Existed: false}
			b.FileCount = len(b.files)
		}
		return
	}
	if !info.Mode().IsRegular() {
		return
	}
	if info.Size() > maxFile {
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	if b.bytes+len(data) > maxBytes {
		return
	}
	b.files[rel] = &Origin{Rel: rel, Existed: true, Mode: info.Mode(), Data: data}
	b.bytes += len(data)
	b.FileCount = len(b.files)
}

func (b *Burst) Restore() (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	var first error
	// Restore existing files first, then delete created ones.
	for _, o := range b.files {
		dest := filepath.Join(b.Root, o.Rel)
		if o.Existed {
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil && first == nil {
				first = err
				continue
			}
			mode := o.Mode
			if mode == 0 {
				mode = 0o644
			}
			if err := os.WriteFile(dest, o.Data, mode.Perm()); err != nil && first == nil {
				first = err
				continue
			}
			n++
			continue
		}
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) && first == nil {
			first = err
			continue
		}
		n++
	}
	return n, first
}

func (b *Burst) Files() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.files))
	for k := range b.files {
		out = append(out, k)
	}
	return out
}

func sameRoot(a, b string) bool {
	aa, _ := filepath.Abs(a)
	bb, _ := filepath.Abs(b)
	return aa == bb
}
