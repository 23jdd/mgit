package object

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type Tag struct {
	ObjectHash string
	ObjectType string
	Name       string
	Tagger     Signature
	Message    string
}

func NewTag(name, objectHash, objectType string, tagger Signature, message string) (*Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("标签名不能为空")
	}
	if err := ValidateHash(objectHash); err != nil {
		return nil, fmt.Errorf("无效对象哈希：%w", err)
	}
	objectType = strings.TrimSpace(objectType)
	if objectType == "" {
		return nil, fmt.Errorf("对象类型不能为空")
	}
	message = strings.TrimRight(message, "\r\n")
	if message == "" {
		message = name
	}
	return &Tag{ObjectHash: objectHash, ObjectType: objectType, Name: name, Tagger: tagger, Message: message}, nil
}

func (t *Tag) Type() string {
	return "tag"
}

func (t *Tag) Payload() []byte {
	var payload bytes.Buffer
	payload.WriteString("object ")
	payload.WriteString(t.ObjectHash)
	payload.WriteByte('\n')
	payload.WriteString("type ")
	payload.WriteString(t.ObjectType)
	payload.WriteByte('\n')
	payload.WriteString("tag ")
	payload.WriteString(t.Name)
	payload.WriteByte('\n')
	payload.WriteString("tagger ")
	payload.WriteString(formatSignature(t.Tagger))
	payload.WriteString("\n\n")
	payload.WriteString(t.Message)
	payload.WriteByte('\n')
	return payload.Bytes()
}

func (t *Tag) Size() int {
	return len(t.Payload())
}

func (t *Tag) Raw() []byte {
	return RawObject(t)
}

func (t *Tag) HashString() string {
	return HashObject(t)
}

func (t *Tag) Write() (string, error) {
	return WriteObject(t)
}

func TagFromStored(stored *StoredObject) (*Tag, error) {
	if stored.ObjectType != "tag" {
		return nil, ErrUnexpectedType("tag", stored.ObjectType)
	}
	return ParseTagPayload(stored.Payload)
}

func ReadTag(hash string) (*Tag, error) {
	stored, err := ReadObject(hash)
	if err != nil {
		return nil, err
	}
	return TagFromStored(stored)
}

func ParseTagPayload(payload []byte) (*Tag, error) {
	raw := string(payload)
	parts := strings.SplitN(raw, "\n\n", 2)
	headers := strings.Split(parts[0], "\n")
	tag := &Tag{}
	for _, header := range headers {
		key, value, ok := strings.Cut(header, " ")
		if !ok {
			continue
		}
		switch key {
		case "object":
			tag.ObjectHash = value
		case "type":
			tag.ObjectType = value
		case "tag":
			tag.Name = value
		case "tagger":
			tag.Tagger = parseSignature(value)
		}
	}
	if len(parts) == 2 {
		tag.Message = strings.TrimRight(parts[1], "\r\n")
	}
	if tag.ObjectHash == "" || tag.ObjectType == "" || tag.Name == "" {
		return nil, fmt.Errorf("无效 tag 对象：缺少必要字段")
	}
	if tag.Tagger.When.IsZero() {
		tag.Tagger.When = time.Now()
	}
	return tag, nil
}
