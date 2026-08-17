package policy

import (
	"os"
	"path/filepath"
	"strings"
)

const maxWatchRoots = 16

// MatchRoot picks the most specific watched folder that contains cwd.
// If none match, cwd itself is the project for this call.
func MatchRoot(cwd string, roots []string) string {
	abs := absPath(cwd)
	if abs == "" {
		if len(roots) > 0 {
			return absPath(roots[0])
		}
		return cwd
	}
	best := ""
	bestLen := -1
	for _, r := range roots {
		rr := absPath(r)
		if rr == "" {
			continue
		}
		if containsPath(rr, abs) && len(rr) > bestLen {
			best = rr
			bestLen = len(rr)
		}
	}
	if best != "" {
		return best
	}
	return abs
}

// RememberRoot prepends cwd onto the watch list. Skips home and /.
func RememberRoot(roots []string, cwd string) []string {
	abs := absPath(cwd)
	if abs == "" || abs == string(filepath.Separator) {
		return roots
	}
	if home, err := os.UserHomeDir(); err == nil && absPath(home) == abs {
		return roots
	}
	out := make([]string, 0, len(roots)+1)
	out = append(out, abs)
	for _, r := range roots {
		rr := absPath(r)
		if rr == "" || rr == abs {
			continue
		}
		out = append(out, rr)
	}
	if len(out) > maxWatchRoots {
		out = out[:maxWatchRoots]
	}
	return out
}

func RemoveRoot(roots []string, path string) []string {
	abs := absPath(path)
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		rr := absPath(r)
		if rr == "" || rr == abs {
			continue
		}
		out = append(out, rr)
	}
	return out
}

func SameRoot(a, b string) bool {
	return absPath(a) != "" && absPath(a) == absPath(b)
}

func absPath(p string) string {
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

func containsPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
