package ignore

import "testing"

func TestParseRule(t *testing.T) {
	tests := []struct {
		line string
		want Rule
		ok   bool
	}{
		{line: "", ok: false},
		{line: "# comment", ok: false},
		{line: "build/", want: Rule{Pattern: "build", DirOnly: true}, ok: true},
		{line: "!/keep.txt", want: Rule{Pattern: "keep.txt", Negate: true, Anchored: true}, ok: true},
		{line: "src/*.log", want: Rule{Pattern: "src/*.log", Anchored: true}, ok: true},
	}
	for _, tt := range tests {
		got, ok := parseRule(tt.line)
		if ok != tt.ok {
			t.Fatalf("parseRule(%q) ok = %v, want %v", tt.line, ok, tt.ok)
		}
		if got != tt.want {
			t.Fatalf("parseRule(%q) = %#v, want %#v", tt.line, got, tt.want)
		}
	}
}

func TestRuleMatches(t *testing.T) {
	tests := []struct {
		rule  Rule
		rel   string
		isDir bool
		want  bool
	}{
		{rule: Rule{Pattern: "*.log"}, rel: "app/server.log", want: true},
		{rule: Rule{Pattern: "build", DirOnly: true}, rel: "src/build/file.txt", want: true},
		{rule: Rule{Pattern: "build", DirOnly: true}, rel: "build", isDir: true, want: true},
		{rule: Rule{Pattern: "build", DirOnly: true}, rel: "build", isDir: false, want: false},
		{rule: Rule{Pattern: "src/*.log", Anchored: true}, rel: "src/app.log", want: true},
		{rule: Rule{Pattern: "src/*.log", Anchored: true}, rel: "nested/src/app.log", want: false},
	}
	for _, tt := range tests {
		if got := tt.rule.matches(tt.rel, tt.isDir); got != tt.want {
			t.Fatalf("%#v.matches(%q, %v) = %v, want %v", tt.rule, tt.rel, tt.isDir, got, tt.want)
		}
	}
}
