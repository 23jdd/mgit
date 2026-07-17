package main

import (
	"fmt"
	"github.com/23jdd/mgit/object"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func defaultSignature() object.Signature {
	name := firstNonEmpty(os.Getenv("MGIT_AUTHOR_NAME"), os.Getenv("GIT_AUTHOR_NAME"), os.Getenv("USERNAME"), "mgit")
	email := firstNonEmpty(os.Getenv("MGIT_AUTHOR_EMAIL"), os.Getenv("GIT_AUTHOR_EMAIL"), "mgit@example.local")
	return object.NewSignature(name, email, time.Now())
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func readHeadCommit() (string, error) {
	refPath, directHash, err := resolveHead()
	if err != nil {
		return "", err
	}
	if directHash != "" {
		return directHash, nil
	}
	content, err := os.ReadFile(refPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取 HEAD 引用失败：%w", err)
	}
	hash := strings.TrimSpace(string(content))
	if hash == "" {
		return "", nil
	}
	if err := requireObjectType(hash, "commit"); err != nil {
		return "", err
	}
	return hash, nil
}

func updateHead(hash string) error {
	return updateHeadWithMessage(hash, "update")
}

func updateHeadWithMessage(hash string, message string) error {
	if err := requireObjectType(hash, "commit"); err != nil {
		return err
	}
	oldHash, _ := readHeadCommit()
	refPath, directHash, err := resolveHead()
	if err != nil {
		return err
	}
	if directHash != "" {
		if err := os.WriteFile(filepath.Join(myGitDir, "HEAD"), []byte(hash+"\n"), 0o644); err != nil {
			return err
		}
		return appendReflog(oldHash, hash, message)
	}
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		return fmt.Errorf("创建引用目录失败：%w", err)
	}
	if err := os.WriteFile(refPath, []byte(hash+"\n"), 0o644); err != nil {
		return err
	}
	return appendReflog(oldHash, hash, message)
}

func resolveHead() (refPath string, directHash string, err error) {
	content, err := os.ReadFile(filepath.Join(myGitDir, "HEAD"))
	if os.IsNotExist(err) {
		return filepath.Join(myGitDir, "refs", "heads", "main"), "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("读取 HEAD 失败：%w", err)
	}
	value := strings.TrimSpace(string(content))
	if strings.HasPrefix(value, "ref: ") {
		refName := strings.TrimSpace(strings.TrimPrefix(value, "ref: "))
		if err := validateRefName(refName); err != nil {
			return "", "", err
		}
		return filepath.Join(myGitDir, filepath.FromSlash(refName)), "", nil
	}
	if value == "" {
		return filepath.Join(myGitDir, "refs", "heads", "main"), "", nil
	}
	if err := object.ValidateHash(value); err != nil {
		return "", "", fmt.Errorf("HEAD 内容无效：%w", err)
	}
	return "", value, nil
}

func writeRef(refName string, hash string) error {
	if err := validateRefName(refName); err != nil {
		return err
	}
	if err := object.ValidateHash(hash); err != nil {
		return err
	}
	path := filepath.Join(myGitDir, filepath.FromSlash(refName))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建引用目录失败：%w", err)
	}
	return os.WriteFile(path, []byte(hash+"\n"), 0o644)
}

func validateRefName(refName string) error {
	if refName == "" || strings.Contains(refName, "..") || strings.HasPrefix(refName, "/") || strings.HasSuffix(refName, "/") {
		return fmt.Errorf("无效引用名：%q", refName)
	}
	for _, part := range strings.Split(refName, "/") {
		if err := validateRefPart(part); err != nil {
			return err
		}
	}
	return nil
}
