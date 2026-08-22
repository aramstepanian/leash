package policy

import (
	"os"
	"path/filepath"
	"strings"
)

const MaxWatchRoots = 16

// ContainingRoot returns the most specific watch root that contains cwd.
// Equal roots win over parents. Empty if none match.
func ContainingRoot(cwd string, roots []string) string {
	cwd = cleanAbs(cwd)
	if cwd == "" {
		return ""
	}
	best := ""
	bestLen := -1
	for _, r := range roots {
		r = cleanAbs(r)
		if r == "" {
			continue
		}
		if !Contains(r, cwd) {
			continue
		}
		if n := len(r); n > bestLen {
			best = r
			bestLen = n
		}
	}
	return best
}

// MatchRoot is ContainingRoot, or cwd itself when the path is not under any watch.
func MatchRoot(cwd string, roots []string) string {
	if r := ContainingRoot(cwd, roots); r != "" {
		return r
	}
	return cleanAbs(cwd)
}

// Contains reports whether path is inside root (or is root).
func Contains(root, path string) bool {
	root = cleanAbs(root)
	path = cleanAbs(path)
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func SameRoot(a, b string) bool {
	aa := cleanAbs(a)
	bb := cleanAbs(b)
	if aa == "" || bb == "" {
		return false
	}
	return aa == bb
}

// AddRoot appends path if it is not already in the list. At capacity, the oldest entry is dropped.
func AddRoot(roots []string, path string) []string {
	path = cleanAbs(path)
	if path == "" {
		return roots
	}
	for _, r := range roots {
		if SameRoot(r, path) {
			return roots
		}
	}
	if len(roots) >= MaxWatchRoots {
		roots = append([]string{}, roots[1:]...)
	}
	return append(roots, path)
}

// RememberRoot auto-adds a project folder from a hook cwd.
// Skips /, $HOME, paths already inside a watched folder, and the cap.
func RememberRoot(roots []string, cwd string) []string {
	cwd = cleanAbs(cwd)
	if cwd == "" || cwd == string(filepath.Separator) {
		return roots
	}
	if home, err := os.UserHomeDir(); err == nil && SameRoot(cwd, home) {
		return roots
	}
	if ContainingRoot(cwd, roots) != "" {
		return roots
	}
	if len(roots) >= MaxWatchRoots {
		return roots
	}
	return append(append([]string{}, roots...), cwd)
}

func RemoveRoot(roots []string, path string) []string {
	path = cleanAbs(path)
	if path == "" {
		return roots
	}
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		if !SameRoot(r, path) {
			out = append(out, r)
		}
	}
	return out
}

func cleanAbs(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}
