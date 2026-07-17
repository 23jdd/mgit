package object

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const ObjectDir = ".git/objects"

type Object interface {
	Type() string
	Payload() []byte
}

type StoredObject struct {
	ObjectType string
	Payload    []byte
	Hash       string
}

func ErrUnexpectedType(expected, actual string) error {
	return fmt.Errorf("对象类型不匹配：期望 %s，实际 %s", expected, actual)
}

func RawObject(obj Object) []byte {
	payload := obj.Payload()
	header := fmt.Sprintf("%s %d\x00", obj.Type(), len(payload))

	raw := make([]byte, 0, len(header)+len(payload))
	raw = append(raw, []byte(header)...)
	raw = append(raw, payload...)
	return raw
}

func HashObject(obj Object) string {
	hash := sha1.Sum(RawObject(obj))
	return hex.EncodeToString(hash[:])
}

func WriteObject(obj Object) (string, error) {
	hash := HashObject(obj)
	objectPath := PathForHash(hash)

	if _, err := os.Stat(objectPath); err == nil {
		return hash, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("检查对象文件失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(objectPath), 0o755); err != nil {
		return "", fmt.Errorf("创建对象目录失败：%w", err)
	}

	compressed, err := compress(RawObject(obj))
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(objectPath, compressed, 0o444); err != nil {
		return "", fmt.Errorf("写入对象文件失败：%w", err)
	}
	return hash, nil
}

func ReadObject(hash string) (*StoredObject, error) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if err := ValidateHash(hash); err != nil {
		return nil, err
	}

	compressed, err := os.ReadFile(PathForHash(hash))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("对象不存在：%s", hash)
		}
		return nil, fmt.Errorf("读取对象文件失败：%w", err)
	}

	raw, err := decompress(compressed)
	if err != nil {
		return nil, err
	}

	stored, err := ParseRaw(raw)
	if err != nil {
		return nil, err
	}
	stored.Hash = hash

	actualHash := hashRaw(raw)
	if actualHash != hash {
		return nil, fmt.Errorf("对象哈希不匹配：期望 %s，实际 %s", hash, actualHash)
	}
	return stored, nil
}

func ParseRaw(raw []byte) (*StoredObject, error) {
	nulIndex := bytes.IndexByte(raw, 0)
	if nulIndex == -1 {
		return nil, errors.New("无效对象：缺少 NUL 分隔符")
	}

	header := string(raw[:nulIndex])
	payload := raw[nulIndex+1:]
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效对象头：%q", header)
	}

	expectedSize, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("无效对象大小 %q：%w", parts[1], err)
	}
	if expectedSize != len(payload) {
		return nil, fmt.Errorf("对象大小不匹配：头部=%d 实际=%d", expectedSize, len(payload))
	}

	data := make([]byte, len(payload))
	copy(data, payload)
	return &StoredObject{ObjectType: parts[0], Payload: data}, nil
}

func PathForHash(hash string) string {
	return filepath.Join(ObjectDir, hash[:2], hash[2:])
}

func ValidateHash(hash string) error {
	if len(hash) != sha1.Size*2 {
		return fmt.Errorf("无效 SHA-1 长度：期望 40，实际 %d", len(hash))
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return fmt.Errorf("无效 SHA-1 值：%w", err)
	}
	return nil
}

func hashRaw(raw []byte) string {
	hash := sha1.Sum(raw)
	return hex.EncodeToString(hash[:])
}

func compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zlib.NewWriter(&buffer)
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("压缩对象失败：%w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("结束压缩失败：%w", err)
	}
	return buffer.Bytes(), nil
}

func decompress(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("创建 zlib 读取器失败：%w", err)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("解压对象失败：%w", err)
	}
	return raw, nil
}
