package main

import (
	"flag"
	"fmt"
	"github.com/23jdd/mgit/ignore"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"github.com/23jdd/mgit/repo"
	"os"
	"path/filepath"
	"sort"
)

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
	if err := repo.Mark(); err != nil {
		return fmt.Errorf("写入 mgit 标记失败：%w", err)
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

func runRm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cached := fs.Bool("cached", false, "只从 index 移除，不删除工作区文件")
	recursive := fs.Bool("r", false, "允许移除目录下的已跟踪文件")
	force := fs.Bool("f", false, "兼容 Git 的强制删除参数；当前实现不额外检查脏状态")
	_ = force
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit rm [--cached] [-r] [-f] <路径> [更多路径]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("rm 需要至少一个路径")
	}

	indexFile, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return err
	}
	pathspecs, err := normalizeWorktreePaths("rm", fs.Args())
	if err != nil {
		return err
	}

	matchedSpecs := make([]bool, len(pathspecs))
	removed := make([]idx.Entry, 0)
	kept := indexFile.Entries[:0]
	for _, entry := range indexFile.Entries {
		matched := false
		for i, pathspec := range pathspecs {
			if restorePathMatches(entry.Path, pathspec) {
				if entry.Path != pathspec && !*recursive && pathspec != "." {
					return fmt.Errorf("路径是目录或前缀，移除多个文件时请加 -r：%s", pathspec)
				}
				matched = true
				matchedSpecs[i] = true
				break
			}
		}
		if matched {
			removed = append(removed, entry)
			continue
		}
		kept = append(kept, entry)
	}
	for i, matched := range matchedSpecs {
		if !matched {
			return fmt.Errorf("index 中没有路径：%s", pathspecs[i])
		}
	}
	indexFile.Entries = kept
	if err := idx.Save(idx.DefaultPath, indexFile); err != nil {
		return err
	}
	if !*cached {
		for _, entry := range removed {
			if err := removeWorktreePath(entry.Path); err != nil {
				return err
			}
		}
	}
	if *cached {
		fmt.Printf("已从 index 移除 %d 个路径\n", len(removed))
	} else {
		fmt.Printf("已移除 %d 个路径\n", len(removed))
	}
	return nil
}

func removeWorktreePath(path string) error {
	worktreePath := filepath.FromSlash(path)
	if err := os.Remove(worktreePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除工作区文件失败 %s：%w", path, err)
	}
	removeEmptyParents(filepath.Dir(worktreePath))
	return nil
}

func removeEmptyParents(dir string) {
	for dir != "." && dir != "" {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit status")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("status 不接收参数")
	}

	branch := currentBranchName()
	if branch == "" {
		branch = "HEAD detached"
	}
	fmt.Printf("位于分支 %s\n", branch)

	headHash, err := readHeadCommit()
	if err != nil {
		return err
	}
	headMap := map[string]diffEntry{}
	if headHash != "" {
		headMap, err = commitDiffMap(headHash)
		if err != nil {
			return err
		}
	}
	indexMap, err := indexDiffMap()
	if err != nil {
		return err
	}
	worktreeMap, err := worktreeDiffMap(indexMap)
	if err != nil {
		return err
	}
	staged := collectStatusChanges(headMap, indexMap)
	unstaged := collectStatusChanges(indexMap, worktreeMap)
	untracked, err := collectUntrackedFiles(indexMap)
	if err != nil {
		return err
	}

	if len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0 {
		fmt.Println("工作区干净")
		return nil
	}
	if len(staged) > 0 {
		fmt.Println()
		fmt.Println("要提交的变更：")
		printStatusChanges(staged)
	}
	if len(unstaged) > 0 {
		fmt.Println()
		fmt.Println("尚未暂存的变更：")
		printStatusChanges(unstaged)
	}
	if len(untracked) > 0 {
		fmt.Println()
		fmt.Println("未跟踪文件：")
		for _, path := range untracked {
			fmt.Printf("  %s\n", path)
		}
	}
	return nil
}

type statusChange struct {
	Kind string
	Path string
}

func collectStatusChanges(oldMap map[string]diffEntry, newMap map[string]diffEntry) []statusChange {
	seen := map[string]bool{}
	for path := range oldMap {
		seen[path] = true
	}
	for path := range newMap {
		seen[path] = true
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	changes := make([]statusChange, 0)
	for _, path := range paths {
		oldEntry := oldMap[path]
		newEntry := newMap[path]
		switch {
		case !oldEntry.Ok && newEntry.Ok:
			changes = append(changes, statusChange{Kind: "新增", Path: path})
		case oldEntry.Ok && !newEntry.Ok:
			changes = append(changes, statusChange{Kind: "删除", Path: path})
		case oldEntry.Ok && newEntry.Ok && oldEntry.Hash != newEntry.Hash:
			changes = append(changes, statusChange{Kind: "修改", Path: path})
		}
	}
	return changes
}

func printStatusChanges(changes []statusChange) {
	for _, change := range changes {
		fmt.Printf("  %-6s %s\n", change.Kind+":", change.Path)
	}
}

func collectUntrackedFiles(indexMap map[string]diffEntry) ([]string, error) {
	matcher, err := ignore.Load(".")
	if err != nil {
		return nil, fmt.Errorf("读取 .gitignore 失败：%w", err)
	}
	untracked := make([]string, 0)
	if err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if matcher.Ignored(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		rel = normalizeWorktreePath(rel)
		if _, tracked := indexMap[rel]; !tracked {
			untracked = append(untracked, rel)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("扫描工作区失败：%w", err)
	}
	sort.Strings(untracked)
	return untracked, nil
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
	write := fs.Bool("w", false, "把对象写入 mgit 对象库")
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
	if err := updateHeadWithMessage(hash, "commit: "+firstLine(commit.Message)); err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}
