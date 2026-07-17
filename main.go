package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
)

const myGitDir = ".mygit"

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("值不能为空")
	}
	*s = append(*s, value)
	return nil
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "错误：", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		printHelp()
		return nil
	}

	switch args[1] {
	case "init":
		return runInit()
	case "add":
		return runAdd(args[2:])
	case "ls-files":
		return runLsFiles(args[2:])
	case "hash-object":
		return runHashObject(args[2:])
	case "cat-file":
		return runCatFile(args[2:])
	case "write-tree":
		return runWriteTree(args[2:])
	case "commit-tree":
		return runCommitTree(args[2:])
	case "commit":
		return runCommit(args[2:])
	case "tag":
		return runTag(args[2:])
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("未知命令 %q，运行 mgit help 查看用法", args[1])
	}
}

func runInit() error {
	paths := []string{
		object.ObjectDir,
		filepath.Join(myGitDir, "refs", "heads"),
		filepath.Join(myGitDir, "refs", "tags"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("创建目录失败 %s：%w", path, err)
		}
	}

	headPath := filepath.Join(myGitDir, "HEAD")
	if _, err := os.Stat(headPath); os.IsNotExist(err) {
		if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
			return fmt.Errorf("写入 HEAD 失败：%w", err)
		}
	} else if err != nil {
		return fmt.Errorf("检查 HEAD 失败：%w", err)
	}

	abs, err := filepath.Abs(myGitDir)
	if err != nil {
		abs = myGitDir
	}
	fmt.Println("初始化空的 mgit 仓库：", abs)
	return nil
}

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit add <路径> [更多路径]")
		fmt.Fprintln(os.Stderr, "示例：mgit add .")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("add 需要至少一个路径")
	}

	_, added, err := idx.AddPaths(".", fs.Args())
	if err != nil {
		return err
	}
	fmt.Printf("已暂存 %d 个文件\n", len(added))
	return nil
}

func runLsFiles(args []string) error {
	fs := flag.NewFlagSet("ls-files", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showStage := fs.Bool("s", false, "显示 mode 和 hash")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit ls-files [-s]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("ls-files 不接收路径参数")
	}

	file, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return err
	}
	for _, entry := range file.Entries {
		if *showStage {
			fmt.Printf("%s %s\t%s\n", entry.Mode, entry.Hash, entry.Path)
		} else {
			fmt.Println(entry.Path)
		}
	}
	return nil
}

func runHashObject(args []string) error {
	fs := flag.NewFlagSet("hash-object", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	write := fs.Bool("w", false, "把对象写入 .mygit/objects")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit hash-object [-w] <文件>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("hash-object 需要一个文件路径")
	}

	content, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("读取文件失败：%w", err)
	}
	blob := object.NewBlob(content)
	if *write {
		hash, err := blob.Write()
		if err != nil {
			return err
		}
		fmt.Println(hash)
		return nil
	}
	fmt.Println(blob.HashString())
	return nil
}

func runCatFile(args []string) error {
	fs := flag.NewFlagSet("cat-file", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pretty := fs.Bool("p", false, "显示对象内容")
	showType := fs.Bool("t", false, "显示对象类型")
	showSize := fs.Bool("s", false, "显示对象大小")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit cat-file (-p|-t|-s) <对象哈希>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("cat-file 需要一个对象哈希")
	}
	if selectedCount(*pretty, *showType, *showSize) != 1 {
		fs.Usage()
		return fmt.Errorf("cat-file 必须且只能选择 -p、-t、-s 之一")
	}

	stored, err := object.ReadObject(fs.Arg(0))
	if err != nil {
		return err
	}

	switch {
	case *showType:
		fmt.Println(stored.ObjectType)
	case *showSize:
		fmt.Println(len(stored.Payload))
	case *pretty:
		return printObject(stored)
	}
	return nil
}

func runWriteTree(args []string) error {
	fs := flag.NewFlagSet("write-tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	worktree := fs.Bool("worktree", false, "直接从工作区目录写 tree，而不是读取 index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit write-tree")
		fmt.Fprintln(os.Stderr, "      mgit write-tree --worktree [目录]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *worktree {
		if fs.NArg() > 1 {
			fs.Usage()
			return fmt.Errorf("write-tree --worktree 最多接收一个目录")
		}
		dir := "."
		if fs.NArg() == 1 {
			dir = fs.Arg(0)
		}
		hash, _, err := object.WriteTreeFromDir(dir)
		if err != nil {
			return err
		}
		fmt.Println(hash)
		return nil
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("从 index 写 tree 时不接收目录参数")
	}
	hash, _, err := writeTreeFromIndex()
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

func runCommitTree(args []string) error {
	fs := flag.NewFlagSet("commit-tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	message := fs.String("m", "", "提交说明")
	var parents stringList
	fs.Var(&parents, "p", "父提交哈希，可重复传入")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit commit-tree <tree哈希> [-p 父提交哈希] -m <提交说明>")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("commit-tree 需要一个 tree 哈希")
	}

	treeHash := fs.Arg(0)
	if err := requireObjectType(treeHash, "tree"); err != nil {
		return err
	}
	for _, parent := range parents {
		if err := requireObjectType(parent, "commit"); err != nil {
			return err
		}
	}

	sig := defaultSignature()
	commit, err := object.NewCommit(treeHash, parents, sig, sig, *message)
	if err != nil {
		return err
	}
	hash, err := commit.Write()
	if err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

func runCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	message := fs.String("m", "", "提交说明")
	worktree := fs.Bool("worktree", false, "直接提交工作区目录，而不是读取 index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit commit [-m <提交说明>]")
		fmt.Fprintln(os.Stderr, "      mgit commit --worktree [-m <提交说明>] [目录]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	var treeHash string
	var err error
	if *worktree {
		if fs.NArg() > 1 {
			fs.Usage()
			return fmt.Errorf("commit --worktree 最多接收一个目录")
		}
		dir := "."
		if fs.NArg() == 1 {
			dir = fs.Arg(0)
		}
		treeHash, _, err = object.WriteTreeFromDir(dir)
	} else {
		if fs.NArg() != 0 {
			fs.Usage()
			return fmt.Errorf("从 index commit 时不接收目录参数")
		}
		treeHash, _, err = writeTreeFromIndex()
	}
	if err != nil {
		return err
	}

	parents := make([]string, 0, 1)
	parent, err := readHeadCommit()
	if err != nil {
		return err
	}
	if parent != "" {
		parents = append(parents, parent)
	}

	sig := defaultSignature()
	commit, err := object.NewCommit(treeHash, parents, sig, sig, *message)
	if err != nil {
		return err
	}
	hash, err := commit.Write()
	if err != nil {
		return err
	}
	if err := updateHead(hash); err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

func runTag(args []string) error {
	fs := flag.NewFlagSet("tag", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	annotated := fs.Bool("a", false, "创建注解标签对象")
	message := fs.String("m", "", "标签说明；提供后会自动创建注解标签")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit tag [-a] [-m <标签说明>] <标签名> <对象哈希>")
		fmt.Fprintln(os.Stderr, "      mgit tag")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return listTags()
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return fmt.Errorf("tag 需要标签名和对象哈希")
	}

	name := fs.Arg(0)
	if err := validateRefPart(name); err != nil {
		return err
	}
	objectHash := fs.Arg(1)
	stored, err := object.ReadObject(objectHash)
	if err != nil {
		return err
	}

	refHash := objectHash
	if *annotated || strings.TrimSpace(*message) != "" {
		tag, err := object.NewTag(name, objectHash, stored.ObjectType, defaultSignature(), *message)
		if err != nil {
			return err
		}
		refHash, err = tag.Write()
		if err != nil {
			return err
		}
	}

	if err := writeRef("refs/tags/"+name, refHash); err != nil {
		return err
	}
	fmt.Println(refHash)
	return nil
}

func writeTreeFromIndex() (string, *object.Tree, error) {
	file, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return "", nil, err
	}
	return object.WriteTreeFromFiles(file.ToObjectFiles())
}

func printObject(stored *object.StoredObject) error {
	switch stored.ObjectType {
	case "blob", "commit", "tag":
		fmt.Print(string(stored.Payload))
	case "tree":
		tree, err := object.TreeFromStored(stored)
		if err != nil {
			return err
		}
		for _, entry := range tree.Entries {
			fmt.Printf("%s %s %s\t%s\n", entry.Mode, entry.ObjectType, entry.Hash, entry.Name)
		}
	default:
		fmt.Print(string(stored.Payload))
	}
	return nil
}

func requireObjectType(hash string, expected string) error {
	stored, err := object.ReadObject(hash)
	if err != nil {
		return err
	}
	if stored.ObjectType != expected {
		return fmt.Errorf("对象 %s 类型不匹配：期望 %s，实际 %s", hash, expected, stored.ObjectType)
	}
	return nil
}

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
	if err := requireObjectType(hash, "commit"); err != nil {
		return err
	}
	refPath, directHash, err := resolveHead()
	if err != nil {
		return err
	}
	if directHash != "" {
		return os.WriteFile(filepath.Join(myGitDir, "HEAD"), []byte(hash+"\n"), 0o644)
	}
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		return fmt.Errorf("创建引用目录失败：%w", err)
	}
	return os.WriteFile(refPath, []byte(hash+"\n"), 0o644)
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

func listTags() error {
	dir := filepath.Join(myGitDir, "refs", "tags")
	items, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取标签目录失败：%w", err)
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		names = append(names, item.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return nil
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

func validateRefPart(part string) error {
	if part == "" || part == "." || part == ".." {
		return fmt.Errorf("无效引用片段：%q", part)
	}
	if strings.ContainsAny(part, `\:*?"<>| `) {
		return fmt.Errorf("引用名不能包含特殊字符或空格：%q", part)
	}
	return nil
}

func selectedCount(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}

func printHelp() {
	fmt.Println(`mgit - 用 Go 实现的迷你 Git

用法：
  mgit init
      初始化 .mygit 仓库

  mgit add <路径> [更多路径]
      把文件写成 blob，并记录到 .mygit/index

  mgit ls-files [-s]
      查看 index 中已暂存的路径；加 -s 显示 mode 和 hash

  mgit hash-object [-w] <文件>
      计算文件的 blob 哈希；加 -w 时写入对象库

  mgit cat-file (-p|-t|-s) <对象哈希>
      查看对象内容、类型或大小

  mgit write-tree
      根据 index 写成 tree 对象

  mgit write-tree --worktree [目录]
      直接把工作区目录写成 tree 对象

  mgit commit-tree <tree哈希> [-p 父提交哈希] -m <提交说明>
      基于 tree 创建 commit 对象，只打印哈希，不更新 HEAD

  mgit commit [-m <提交说明>]
      根据 index 创建 commit，并更新 HEAD 指向的分支

  mgit tag [-a] [-m <标签说明>] <标签名> <对象哈希>
      创建轻量标签；加 -a 或 -m 时创建注解 tag 对象

  mgit tag
      列出已有标签`)
}
