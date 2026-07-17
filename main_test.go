package main

import (
	"strings"
	"testing"
)

func TestCommandTableHasUniqueNames(t *testing.T) {
	seen := map[string]bool{}
	for _, cmd := range commands {
		if cmd.Name == "" {
			t.Fatal("command name must not be empty")
		}
		if seen[cmd.Name] {
			t.Fatalf("duplicate command %q", cmd.Name)
		}
		seen[cmd.Name] = true
		if len(cmd.Usage) == 0 {
			t.Fatalf("command %q has no usage", cmd.Name)
		}
		if cmd.Run == nil {
			t.Fatalf("command %q has no runner", cmd.Name)
		}
	}

	if _, ok := findCommand("commit"); !ok {
		t.Fatal("commit command not found")
	}
	if _, ok := findCommand("missing"); ok {
		t.Fatal("unexpected command found")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"mgit", "does-not-exist"})
	if err == nil || !strings.Contains(err.Error(), "未知命令") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestNormalizeWorktreePath(t *testing.T) {
	tests := map[string]string{
		".":              ".",
		"./README.md":    "README.md",
		"dir//file.txt":  "dir/file.txt",
		"/dir/file.txt/": "dir/file.txt",
	}
	for input, want := range tests {
		if got := normalizeWorktreePath(input); got != want {
			t.Fatalf("normalizeWorktreePath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeWorktreePathsRejectsEmpty(t *testing.T) {
	if _, err := normalizeWorktreePaths("restore", []string{""}); err == nil {
		t.Fatal("expected empty path error")
	}
}

func TestRestorePathMatches(t *testing.T) {
	tests := []struct {
		filePath string
		pathspec string
		want     bool
	}{
		{filePath: "a/b.txt", pathspec: "a", want: true},
		{filePath: "a/b.txt", pathspec: "a/b.txt", want: true},
		{filePath: "a/b.txt", pathspec: ".", want: true},
		{filePath: "ab/c.txt", pathspec: "a", want: false},
	}
	for _, tt := range tests {
		if got := restorePathMatches(tt.filePath, tt.pathspec); got != tt.want {
			t.Fatalf("restorePathMatches(%q, %q) = %v, want %v", tt.filePath, tt.pathspec, got, tt.want)
		}
	}
}

func TestParseStashIndex(t *testing.T) {
	tests := map[string]int{
		"0":          0,
		" 2 ":        2,
		"stash@{12}": 12,
	}
	for input, want := range tests {
		got, err := parseStashIndex(input)
		if err != nil {
			t.Fatalf("parseStashIndex(%q) returned error: %v", input, err)
		}
		if got != want {
			t.Fatalf("parseStashIndex(%q) = %d, want %d", input, got, want)
		}
	}

	for _, input := range []string{"", "-1", "1x", "stash@{1", "stash@{x}"} {
		if _, err := parseStashIndex(input); err == nil {
			t.Fatalf("parseStashIndex(%q) expected error", input)
		}
	}
}
