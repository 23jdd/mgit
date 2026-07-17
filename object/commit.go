package object

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Signature struct {
	Name  string
	Email string
	When  time.Time
}

type Commit struct {
	Tree      string
	Parents   []string
	Author    Signature
	Committer Signature
	Message   string
}

func NewSignature(name, email string, when time.Time) Signature {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" {
		name = "mgit"
	}
	if email == "" {
		email = "mgit@example.local"
	}
	if when.IsZero() {
		when = time.Now()
	}
	return Signature{Name: name, Email: email, When: when}
}

func NewCommit(tree string, parents []string, author Signature, committer Signature, message string) (*Commit, error) {
	if err := ValidateHash(tree); err != nil {
		return nil, fmt.Errorf("无效 tree 哈希：%w", err)
	}
	for _, parent := range parents {
		if err := ValidateHash(parent); err != nil {
			return nil, fmt.Errorf("无效 parent 哈希 %q：%w", parent, err)
		}
	}
	message = strings.TrimRight(message, "\r\n")
	if message == "" {
		message = "commit"
	}
	return &Commit{Tree: tree, Parents: append([]string(nil), parents...), Author: author, Committer: committer, Message: message}, nil
}

func (c *Commit) Type() string {
	return "commit"
}

func (c *Commit) Payload() []byte {
	var payload bytes.Buffer
	payload.WriteString("tree ")
	payload.WriteString(c.Tree)
	payload.WriteByte('\n')
	for _, parent := range c.Parents {
		payload.WriteString("parent ")
		payload.WriteString(parent)
		payload.WriteByte('\n')
	}
	payload.WriteString("author ")
	payload.WriteString(formatSignature(c.Author))
	payload.WriteByte('\n')
	payload.WriteString("committer ")
	payload.WriteString(formatSignature(c.Committer))
	payload.WriteString("\n\n")
	payload.WriteString(c.Message)
	payload.WriteByte('\n')
	return payload.Bytes()
}

func (c *Commit) Size() int {
	return len(c.Payload())
}

func (c *Commit) Raw() []byte {
	return RawObject(c)
}

func (c *Commit) HashString() string {
	return HashObject(c)
}

func (c *Commit) Write() (string, error) {
	return WriteObject(c)
}

func CommitFromStored(stored *StoredObject) (*Commit, error) {
	if stored.ObjectType != "commit" {
		return nil, ErrUnexpectedType("commit", stored.ObjectType)
	}
	return ParseCommitPayload(stored.Payload)
}

func ReadCommit(hash string) (*Commit, error) {
	stored, err := ReadObject(hash)
	if err != nil {
		return nil, err
	}
	return CommitFromStored(stored)
}

func ParseCommitPayload(payload []byte) (*Commit, error) {
	raw := string(payload)
	parts := strings.SplitN(raw, "\n\n", 2)
	headers := strings.Split(parts[0], "\n")
	commit := &Commit{}
	for _, header := range headers {
		key, value, ok := strings.Cut(header, " ")
		if !ok {
			continue
		}
		switch key {
		case "tree":
			commit.Tree = value
		case "parent":
			commit.Parents = append(commit.Parents, value)
		case "author":
			commit.Author = parseSignature(value)
		case "committer":
			commit.Committer = parseSignature(value)
		}
	}
	if len(parts) == 2 {
		commit.Message = strings.TrimRight(parts[1], "\r\n")
	}
	if commit.Tree == "" {
		return nil, fmt.Errorf("无效 commit 对象：缺少 tree")
	}
	return commit, nil
}

func formatSignature(sig Signature) string {
	_, offsetSeconds := sig.When.Zone()
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	offsetHours := offsetSeconds / 3600
	offsetMinutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("%s <%s> %d %s%02d%02d", sig.Name, sig.Email, sig.When.Unix(), sign, offsetHours, offsetMinutes)
}

func parseSignature(raw string) Signature {
	namePart, rest, found := strings.Cut(raw, " <")
	if !found {
		return NewSignature(raw, "", time.Time{})
	}
	email, tail, found := strings.Cut(rest, "> ")
	if !found {
		return NewSignature(namePart, strings.TrimSuffix(rest, ">"), time.Time{})
	}
	fields := strings.Fields(tail)
	when := time.Time{}
	if len(fields) >= 2 {
		unixSeconds, err := strconv.ParseInt(fields[0], 10, 64)
		if err == nil {
			offset := parseTimezoneOffset(fields[1])
			when = time.Unix(unixSeconds, 0).In(time.FixedZone(fields[1], offset))
		}
	}
	return NewSignature(namePart, email, when)
}

func parseTimezoneOffset(value string) int {
	if len(value) != 5 {
		return 0
	}
	sign := 1
	if value[0] == '-' {
		sign = -1
	} else if value[0] != '+' {
		return 0
	}
	hours, err := strconv.Atoi(value[1:3])
	if err != nil {
		return 0
	}
	minutes, err := strconv.Atoi(value[3:5])
	if err != nil {
		return 0
	}
	return sign * ((hours * 3600) + (minutes * 60))
}
