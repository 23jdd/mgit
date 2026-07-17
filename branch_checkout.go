package main

import (
	"flag"
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runBranch(args []string) error {
	fs := flag.NewFlagSet("branch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit branch")
		fmt.Fprintln(os.Stderr, "      mgit branch <分支名> [commit哈希]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return listBranches()
	}
	if fs.NArg() > 2 {
		fs.Usage()
		return fmt.Errorf("branch 最多接收分支名和一个 commit 哈希")
	}

	name := fs.Arg(0)
	if err := validateBranchName(name); err != nil {
		return err
	}

	commitHash := ""
	if fs.NArg() == 2 {
		resolved, err := resolveCommitish(fs.Arg(1))
		if err != nil {
			return err
		}
		commitHash = resolved
	} else {
		current, err := readHeadCommit()
		if err != nil {
			return err
		}
		if current == "" {
			return fmt.Errorf("当前 HEAD 还没有提交，创建分支时请显式传入 commit 哈希")
		}
		commitHash = current
	}

	refName := "refs/heads/" + name
	refPath := filepath.Join(myGitDir, filepath.FromSlash(refName))
	if _, err := os.Stat(refPath); err == nil {
		return fmt.Errorf("分支已存在：%s", name)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("检查分支失败：%w", err)
	}
	if err := writeRef(refName, commitHash); err != nil {
		return err
	}
	fmt.Println(name)
	return nil
}

func runCheckout(args []string) error {
	fs := flag.NewFlagSet("checkout", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit checkout <分支名|commit哈希>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("checkout 需要一个分支名或 commit 哈希")
	}

	target := fs.Arg(0)
	branchRef := "refs/heads/" + target
	branchPath := filepath.Join(myGitDir, filepath.FromSlash(branchRef))
	if data, err := os.ReadFile(branchPath); err == nil {
		commitHash := strings.TrimSpace(string(data))
		if err := checkoutCommit(commitHash); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(myGitDir, "HEAD"), []byte("ref: "+branchRef+"\n"), 0o644); err != nil {
			return fmt.Errorf("更新 HEAD 失败：%w", err)
		}
		fmt.Printf("切换到分支 %s\n", target)
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("读取分支失败：%w", err)
	}

	commitHash, err := resolveCommitish(target)
	if err != nil {
		return err
	}
	if err := checkoutCommit(commitHash); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(myGitDir, "HEAD"), []byte(commitHash+"\n"), 0o644); err != nil {
		return fmt.Errorf("更新 HEAD 失败：%w", err)
	}
	fmt.Printf("检出 commit %s，当前处于 detached HEAD\n", commitHash)
	return nil
}

func listBranches() error {
	dir := filepath.Join(myGitDir, "refs", "heads")
	items := make([]string, 0)
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if errorsIsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		items = append(items, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取分支失败：%w", err)
	}
	sort.Strings(items)
	current := currentBranchName()
	for _, name := range items {
		prefix := "  "
		if name == current {
			prefix = "* "
		}
		fmt.Println(prefix + name)
	}
	return nil
}

func currentBranchName() string {
	content, err := os.ReadFile(filepath.Join(myGitDir, "HEAD"))
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(string(content))
	if !strings.HasPrefix(value, "ref: refs/heads/") {
		return ""
	}
	return strings.TrimPrefix(value, "ref: refs/heads/")
}

func resolveCommitish(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("commit 标识不能为空")
	}
	if value == "HEAD" {
		commitHash, err := readHeadCommit()
		if err != nil {
			return "", err
		}
		if commitHash == "" {
			return "", fmt.Errorf("HEAD 还没有提交")
		}
		return commitHash, nil
	}
	for _, refName := range []string{"refs/heads/" + value, "refs/tags/" + value} {
		path := filepath.Join(myGitDir, filepath.FromSlash(refName))
		if data, err := os.ReadFile(path); err == nil {
			return peelToCommit(strings.TrimSpace(string(data)))
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("读取引用失败：%w", err)
		}
	}
	return peelToCommit(value)
}

func peelToCommit(hash string) (string, error) {
	stored, err := object.ReadObject(hash)
	if err != nil {
		return "", err
	}
	switch stored.ObjectType {
	case "commit":
		return hash, nil
	case "tag":
		tag, err := object.TagFromStored(stored)
		if err != nil {
			return "", err
		}
		return peelToCommit(tag.ObjectHash)
	default:
		return "", fmt.Errorf("对象 %s 不是 commit：实际类型 %s", hash, stored.ObjectType)
	}
}

func checkoutCommit(commitHash string) error {
	commit, err := object.ReadCommit(commitHash)
	if err != nil {
		return err
	}
	files, err := object.ListTreeFiles(commit.Tree)
	if err != nil {
		return err
	}
	entries := make([]idx.Entry, 0, len(files))
	for _, file := range files {
		blob, err := object.ReadBlob(file.Hash)
		if err != nil {
			return err
		}
		path := filepath.FromSlash(file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("创建目录失败：%w", err)
		}
		if err := os.WriteFile(path, blob.Content, 0o644); err != nil {
			return fmt.Errorf("写入工作区文件失败 %s：%w", file.Path, err)
		}
		entries = append(entries, idx.Entry{Mode: file.Mode, Hash: file.Hash, Path: file.Path})
	}
	return idx.Save(idx.DefaultPath, &idx.File{Entries: entries})
}

func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("分支名不能为空")
	}
	return validateRefName("refs/heads/" + name)
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
