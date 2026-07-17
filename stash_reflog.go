package main

import (
	"encoding/json"
	"flag"
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"github.com/23jdd/mgit/repo"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var stashPath = repo.Path("stash.json")
var reflogPath = repo.Path("logs", "HEAD")

type stashEntry struct {
	Message string             `json:"message"`
	Base    string             `json:"base"`
	When    string             `json:"when"`
	Work    []object.FileEntry `json:"worktree"`
	Index   []idx.Entry        `json:"index"`
}

type stashFile struct {
	Entries []stashEntry `json:"entries"`
}

func runStash(args []string) error {
	if len(args) == 0 {
		return runStashPush(nil)
	}
	switch args[0] {
	case "push", "save":
		return runStashPush(args[1:])
	case "list":
		return runStashList(args[1:])
	case "apply":
		return runStashApply(args[1:], false)
	case "pop":
		return runStashApply(args[1:], true)
	case "drop":
		return runStashDrop(args[1:])
	default:
		return runStashPush(args)
	}
}

func runStashPush(args []string) error {
	fs := flag.NewFlagSet("stash push", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	message := fs.String("m", "", "stash 说明")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit stash [push [-m <说明>]]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("stash push 不接收路径参数")
	}

	indexFile, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return err
	}
	workFiles, err := trackedWorktreeFiles(indexFile.Entries)
	if err != nil {
		return err
	}
	if len(indexFile.Entries) == 0 && len(workFiles) == 0 {
		fmt.Println("没有可保存的已跟踪改动")
		return nil
	}
	base, err := readHeadCommit()
	if err != nil {
		return err
	}
	msg := strings.TrimSpace(*message)
	if msg == "" {
		msg = "WIP on " + currentBranchNameOrHead()
	}
	stash, err := loadStashFile()
	if err != nil {
		return err
	}
	entry := stashEntry{
		Message: msg,
		Base:    base,
		When:    time.Now().Format(time.RFC3339),
		Work:    workFiles,
		Index:   append([]idx.Entry(nil), indexFile.Entries...),
	}
	stash.Entries = append([]stashEntry{entry}, stash.Entries...)
	if err := saveStashFile(stash); err != nil {
		return err
	}
	if base != "" {
		baseFiles, err := commitFiles(base)
		if err != nil {
			return err
		}
		if err := writeMergedState(baseFiles); err != nil {
			return err
		}
	} else {
		if err := saveIndexFromFiles(nil); err != nil {
			return err
		}
	}
	fmt.Printf("保存工作区和 index 到 stash@{0}: %s\n", msg)
	return nil
}

func runStashList(args []string) error {
	fs := flag.NewFlagSet("stash list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit stash list")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("stash list 不接收参数")
	}
	stash, err := loadStashFile()
	if err != nil {
		return err
	}
	for i, entry := range stash.Entries {
		base := shortHash(entry.Base)
		if base == "" {
			base = "no-base"
		}
		fmt.Printf("stash@{%d}: %s %s: %s\n", i, entry.When, base, entry.Message)
	}
	return nil
}

func runStashApply(args []string, drop bool) error {
	fs := flag.NewFlagSet("stash apply", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit stash apply [stash@{n}|n]")
		fmt.Fprintln(os.Stderr, "      mgit stash pop [stash@{n}|n]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("stash apply/pop 最多接收一个 stash 编号")
	}
	stash, err := loadStashFile()
	if err != nil {
		return err
	}
	if len(stash.Entries) == 0 {
		return fmt.Errorf("没有 stash 可用")
	}
	index := 0
	if fs.NArg() == 1 {
		index, err = parseStashIndex(fs.Arg(0))
		if err != nil {
			return err
		}
	}
	if index < 0 || index >= len(stash.Entries) {
		return fmt.Errorf("stash 不存在：%d", index)
	}
	entry := stash.Entries[index]
	if err := writeMergedState(entry.Work); err != nil {
		return err
	}
	if err := idx.Save(idx.DefaultPath, &idx.File{Entries: append([]idx.Entry(nil), entry.Index...)}); err != nil {
		return err
	}
	if drop {
		stash.Entries = append(stash.Entries[:index], stash.Entries[index+1:]...)
		if err := saveStashFile(stash); err != nil {
			return err
		}
		fmt.Printf("应用并删除 stash@{%d}: %s\n", index, entry.Message)
		return nil
	}
	fmt.Printf("应用 stash@{%d}: %s\n", index, entry.Message)
	return nil
}

func runStashDrop(args []string) error {
	fs := flag.NewFlagSet("stash drop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit stash drop [stash@{n}|n]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("stash drop 最多接收一个 stash 编号")
	}
	stash, err := loadStashFile()
	if err != nil {
		return err
	}
	if len(stash.Entries) == 0 {
		return fmt.Errorf("没有 stash 可删除")
	}
	index := 0
	if fs.NArg() == 1 {
		index, err = parseStashIndex(fs.Arg(0))
		if err != nil {
			return err
		}
	}
	if index < 0 || index >= len(stash.Entries) {
		return fmt.Errorf("stash 不存在：%d", index)
	}
	entry := stash.Entries[index]
	stash.Entries = append(stash.Entries[:index], stash.Entries[index+1:]...)
	if err := saveStashFile(stash); err != nil {
		return err
	}
	fmt.Printf("删除 stash@{%d}: %s\n", index, entry.Message)
	return nil
}

func loadStashFile() (*stashFile, error) {
	data, err := os.ReadFile(stashPath)
	if os.IsNotExist(err) {
		return &stashFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 stash 失败：%w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &stashFile{}, nil
	}
	var file stashFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析 stash 失败：%w", err)
	}
	return &file, nil
}

func saveStashFile(file *stashFile) error {
	if err := os.MkdirAll(filepath.Dir(stashPath), 0o755); err != nil {
		return fmt.Errorf("创建 stash 目录失败：%w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 stash 失败：%w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(stashPath, data, 0o644)
}

func trackedWorktreeFiles(entries []idx.Entry) ([]object.FileEntry, error) {
	files := make([]object.FileEntry, 0, len(entries))
	for _, entry := range entries {
		path := filepath.FromSlash(entry.Path)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取工作区文件失败 %s：%w", entry.Path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取工作区文件失败 %s：%w", entry.Path, err)
		}
		blob := object.NewBlob(content)
		hash, err := blob.Write()
		if err != nil {
			return nil, err
		}
		files = append(files, object.FileEntry{Mode: firstNonEmpty(entry.Mode, "100644"), Path: entry.Path, Hash: hash})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func parseStashIndex(value string) (int, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "stash@{")
	value = strings.TrimSuffix(value, "}")
	var index int
	if _, err := fmt.Sscanf(value, "%d", &index); err != nil {
		return 0, fmt.Errorf("无效 stash 编号：%s", value)
	}
	return index, nil
}

func runReflog(args []string) error {
	fs := flag.NewFlagSet("reflog", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	maxCount := fs.Int("n", 0, "最多显示多少条 reflog；0 表示不限制")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit reflog [-n 数量]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("reflog 不接收位置参数")
	}
	data, err := os.ReadFile(reflogPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取 reflog 失败：%w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	printed := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		parts := strings.SplitN(lines[i], "\t", 4)
		if len(parts) != 4 {
			continue
		}
		fmt.Printf("%s HEAD@{%d}: %s\n", shortHash(parts[1]), printed, parts[3])
		printed++
		if *maxCount > 0 && printed >= *maxCount {
			break
		}
	}
	return nil
}

func appendReflog(oldHash string, newHash string, message string) error {
	if strings.TrimSpace(message) == "" {
		message = "update"
	}
	if oldHash == "" {
		oldHash = strings.Repeat("0", 40)
	}
	if err := os.MkdirAll(filepath.Dir(reflogPath), 0o755); err != nil {
		return fmt.Errorf("创建 reflog 目录失败：%w", err)
	}
	line := fmt.Sprintf("%s\t%s\t%s\t%s\n", oldHash, newHash, time.Now().Format(time.RFC3339), strings.ReplaceAll(message, "\n", " "))
	file, err := os.OpenFile(reflogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开 reflog 失败：%w", err)
	}
	defer file.Close()
	_, err = file.WriteString(line)
	return err
}
