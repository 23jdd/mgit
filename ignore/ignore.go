package ignore

import (
	"bufio"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

type Rule struct {
	Pattern  string
	Negate   bool
	DirOnly  bool
	Anchored bool
}

type Matcher struct {
	Root  string
	Rules []Rule
}

func Load(root string) (*Matcher, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	matcher := &Matcher{Root: absRoot}
	file, err := os.Open(filepath.Join(absRoot, ".gitignore"))
	if os.IsNotExist(err) {
		return matcher, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rule, ok := parseRule(scanner.Text())
		if ok {
			matcher.Rules = append(matcher.Rules, rule)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return matcher, nil
}

func (m *Matcher) Ignored(absPath string, isDir bool) bool {
	if !filepath.IsAbs(absPath) {
		resolved, err := filepath.Abs(absPath)
		if err == nil {
			absPath = resolved
		}
	}
	name := filepath.Base(absPath)
	if IsInternalName(name) {
		return true
	}
	if m == nil {
		return false
	}
	rel, err := filepath.Rel(m.Root, absPath)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.HasPrefix(rel, "../") || rel == ".." {
		return false
	}
	ignored := false
	for _, rule := range m.Rules {
		if rule.matches(rel, isDir) {
			ignored = !rule.Negate
		}
	}
	return ignored
}

func IsInternalName(name string) bool {
	switch name {
	case ".git", ".mygit", ".gocache", ".agents", ".codex":
		return true
	default:
		return false
	}
}

func parseRule(line string) (Rule, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return Rule{}, false
	}
	rule := Rule{}
	if strings.HasPrefix(line, "!") {
		rule.Negate = true
		line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
	}
	if line == "" {
		return Rule{}, false
	}
	if strings.HasSuffix(line, "/") {
		rule.DirOnly = true
		line = strings.TrimRight(line, "/")
	}
	if strings.HasPrefix(line, "/") {
		rule.Anchored = true
		line = strings.TrimLeft(line, "/")
	}
	line = filepath.ToSlash(filepath.Clean(line))
	if line == "." || line == "" {
		return Rule{}, false
	}
	if strings.Contains(line, "/") {
		rule.Anchored = true
	}
	rule.Pattern = line
	return rule, true
}

func (r Rule) matches(rel string, isDir bool) bool {
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return false
	}
	if r.DirOnly {
		return matchDirRule(r.Pattern, rel, r.Anchored, isDir)
	}
	if r.Anchored {
		return matchPattern(r.Pattern, rel)
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if matchPattern(r.Pattern, part) {
			return true
		}
	}
	return false
}

func matchDirRule(pattern string, rel string, anchored bool, isDir bool) bool {
	if anchored {
		if matchPattern(pattern, rel) {
			return isDir || rel == pattern
		}
		return strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/")+"/")
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		if matchPattern(pattern, part) {
			return i < len(parts)-1 || isDir
		}
	}
	return false
}

func matchPattern(pattern string, value string) bool {
	ok, err := pathpkg.Match(pattern, value)
	if err == nil && ok {
		return true
	}
	return pattern == value
}
