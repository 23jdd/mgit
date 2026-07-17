package object

import (
	"strings"
	"testing"
	"time"
)

func TestParseCommitPayload(t *testing.T) {
	payload := []byte(strings.Join([]string{
		"tree 0123456789abcdef0123456789abcdef01234567",
		"parent 1111111111111111111111111111111111111111",
		"parent 2222222222222222222222222222222222222222",
		"author Alice <alice@example.com> 1710000000 +0800",
		"committer Bob <bob@example.com> 1710000300 +0800",
		"",
		"subject",
		"body",
		"",
	}, "\n"))

	commit, err := ParseCommitPayload(payload)
	if err != nil {
		t.Fatalf("ParseCommitPayload returned error: %v", err)
	}
	if commit.Tree != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("unexpected tree hash %q", commit.Tree)
	}
	if len(commit.Parents) != 2 {
		t.Fatalf("expected 2 parents, got %d", len(commit.Parents))
	}
	if commit.Author.Name != "Alice" || commit.Author.Email != "alice@example.com" {
		t.Fatalf("unexpected author: %#v", commit.Author)
	}
	if commit.Message != "subject\nbody" {
		t.Fatalf("unexpected message %q", commit.Message)
	}
}

func TestNewCommitDefaultsMessage(t *testing.T) {
	sig := NewSignature("", "", time.Unix(1, 0))
	commit, err := NewCommit("0123456789abcdef0123456789abcdef01234567", nil, sig, sig, "\n")
	if err != nil {
		t.Fatalf("NewCommit returned error: %v", err)
	}
	if commit.Message != "commit" {
		t.Fatalf("expected default message, got %q", commit.Message)
	}
	if commit.Author.Name != "mgit" || commit.Author.Email != "mgit@example.local" {
		t.Fatalf("unexpected default signature: %#v", commit.Author)
	}
}

func TestParseTreePayloadRejectsTruncatedHash(t *testing.T) {
	_, err := ParseTreePayload([]byte("100644 file.txt\x00short"))
	if err == nil || !strings.Contains(err.Error(), "哈希长度不足") {
		t.Fatalf("expected truncated hash error, got %v", err)
	}
}
