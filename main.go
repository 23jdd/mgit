package main

import (
	"bytes"
	"encoding/json"
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
	case "rm":
		return runRm(args[2:])
	case "status":
		return runStatus(args[2:])
	case "ls-files":
		return runLsFiles(args[2:])
	case "hash-object":
		return runHashObject(args[2:])
	case "cat-file":
		return runCatFile(args[2:])
	case "diff":
		return runDiff(args[2:])
	case "write-tree":
		return runWriteTree(args[2:])
	case "commit-tree":
		return runCommitTree(args[2:])
	case "commit":
		return runCommit(args[2:])
	case "reset":
		return runReset(args[2:])
	case "stash":
		return runStash(args[2:])
	case "merge":
		return runMerge(args[2:])
	case "log":
		return runLog(args[2:])
	case "reflog":
		return runReflog(args[2:])
	case "branch":
		return runBranch(args[2:])
	case "checkout":
		return runCheckout(args[2:])
	case "restore":
		return runRestore(args[2:])
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
	pathspecs := make([]string, 0, fs.NArg())
	for _, arg := range fs.Args() {
		pathspec := normalizeWorktreePath(arg)
		if pathspec == "" {
			return fmt.Errorf("rm 路径不能为空")
		}
		pathspecs = append(pathspecs, pathspec)
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
	untracked := make([]string, 0)
	if err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() && shouldSkipWorktreeName(name) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if shouldSkipWorktreeName(name) {
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

func shouldSkipWorktreeName(name string) bool {
	switch name {
	case ".git", ".mygit", ".gocache", ".agents", ".codex":
		return true
	default:
		return false
	}
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
	if err := updateHeadWithMessage(hash, "commit: "+firstLine(commit.Message)); err != nil {
		return err
	}
	fmt.Println(hash)
	return nil
}

type diffEntry struct {
	Mode    string
	Hash    string
	Content []byte
	Ok      bool
}

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	staged := fs.Bool("staged", false, "比较 HEAD/指定 commit 与 index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit diff [--staged] [commit|分支|标签]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("diff 最多接收一个 commit、分支或标签")
	}

	if *staged {
		base := "HEAD"
		if fs.NArg() == 1 {
			base = fs.Arg(0)
		}
		baseHash, err := resolveCommitish(base)
		if err != nil {
			return err
		}
		baseMap, err := commitDiffMap(baseHash)
		if err != nil {
			return err
		}
		indexMap, err := indexDiffMap()
		if err != nil {
			return err
		}
		return printDiffMaps(baseMap, indexMap, "a", "b")
	}

	indexMap, err := indexDiffMap()
	if err != nil {
		return err
	}
	worktreeMap, err := worktreeDiffMap(indexMap)
	if err != nil {
		return err
	}
	if fs.NArg() == 1 {
		baseHash, err := resolveCommitish(fs.Arg(0))
		if err != nil {
			return err
		}
		baseMap, err := commitDiffMap(baseHash)
		if err != nil {
			return err
		}
		return printDiffMaps(baseMap, worktreeMap, "a", "b")
	}
	return printDiffMaps(indexMap, worktreeMap, "a", "b")
}

func commitDiffMap(commitHash string) (map[string]diffEntry, error) {
	files, err := commitFiles(commitHash)
	if err != nil {
		return nil, err
	}
	result := make(map[string]diffEntry, len(files))
	for _, file := range files {
		blob, err := object.ReadBlob(file.Hash)
		if err != nil {
			return nil, err
		}
		result[file.Path] = diffEntry{Mode: file.Mode, Hash: file.Hash, Content: blob.Content, Ok: true}
	}
	return result, nil
}

func indexDiffMap() (map[string]diffEntry, error) {
	file, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return nil, err
	}
	result := make(map[string]diffEntry, len(file.Entries))
	for _, entry := range file.Entries {
		blob, err := object.ReadBlob(entry.Hash)
		if err != nil {
			return nil, err
		}
		result[entry.Path] = diffEntry{Mode: entry.Mode, Hash: entry.Hash, Content: blob.Content, Ok: true}
	}
	return result, nil
}

func worktreeDiffMap(indexMap map[string]diffEntry) (map[string]diffEntry, error) {
	result := make(map[string]diffEntry, len(indexMap))
	for path, entry := range indexMap {
		content, err := os.ReadFile(filepath.FromSlash(path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取工作区文件失败 %s：%w", path, err)
		}
		blob := object.NewBlob(content)
		result[path] = diffEntry{Mode: entry.Mode, Hash: blob.HashString(), Content: content, Ok: true}
	}
	return result, nil
}

func printDiffMaps(oldMap map[string]diffEntry, newMap map[string]diffEntry, oldPrefix string, newPrefix string) error {
	paths := make([]string, 0, len(oldMap)+len(newMap))
	seen := map[string]bool{}
	for path := range oldMap {
		seen[path] = true
	}
	for path := range newMap {
		seen[path] = true
	}
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		oldEntry := oldMap[path]
		newEntry := newMap[path]
		if oldEntry.Ok && newEntry.Ok && oldEntry.Hash == newEntry.Hash {
			continue
		}
		printFileDiff(path, oldEntry, newEntry, oldPrefix, newPrefix)
	}
	return nil
}

func printFileDiff(path string, oldEntry diffEntry, newEntry diffEntry, oldPrefix string, newPrefix string) {
	fmt.Printf("diff --mgit %s/%s %s/%s\n", oldPrefix, path, newPrefix, path)
	if !oldEntry.Ok {
		fmt.Printf("new file mode %s\n", firstNonEmpty(newEntry.Mode, "100644"))
	} else if !newEntry.Ok {
		fmt.Printf("deleted file mode %s\n", firstNonEmpty(oldEntry.Mode, "100644"))
	}
	if oldEntry.Ok {
		fmt.Printf("--- %s/%s\n", oldPrefix, path)
	} else {
		fmt.Println("--- /dev/null")
	}
	if newEntry.Ok {
		fmt.Printf("+++ %s/%s\n", newPrefix, path)
	} else {
		fmt.Println("+++ /dev/null")
	}
	oldLines := splitDiffLines(oldEntry.Content)
	newLines := splitDiffLines(newEntry.Content)
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	for i := 0; i < max; i++ {
		var oldLine, newLine string
		oldOK := i < len(oldLines)
		newOK := i < len(newLines)
		if oldOK {
			oldLine = oldLines[i]
		}
		if newOK {
			newLine = newLines[i]
		}
		switch {
		case oldOK && newOK && oldLine == newLine:
			fmt.Printf(" %s\n", oldLine)
		case oldOK && newOK:
			fmt.Printf("-%s\n", oldLine)
			fmt.Printf("+%s\n", newLine)
		case oldOK:
			fmt.Printf("-%s\n", oldLine)
		case newOK:
			fmt.Printf("+%s\n", newLine)
		}
	}
}

func splitDiffLines(content []byte) []string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
func runReset(args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	soft := fs.Bool("soft", false, "只移动 HEAD，不改 index 和工作区")
	mixed := fs.Bool("mixed", false, "移动 HEAD，并把 index 重置到目标 commit；默认模式")
	hard := fs.Bool("hard", false, "移动 HEAD，并把 index 和工作区都重置到目标 commit")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit reset [--soft|--mixed|--hard] [commit|分支|标签]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("reset 最多接收一个 commit、分支或标签")
	}
	if selectedCount(*soft, *mixed, *hard) > 1 {
		fs.Usage()
		return fmt.Errorf("reset 只能选择 --soft、--mixed、--hard 之一")
	}

	mode := "mixed"
	switch {
	case *soft:
		mode = "soft"
	case *hard:
		mode = "hard"
	case *mixed:
		mode = "mixed"
	}

	target := "HEAD"
	if fs.NArg() == 1 {
		target = fs.Arg(0)
	}
	commitHash, err := resolveCommitish(target)
	if err != nil {
		return err
	}

	files, err := commitFiles(commitHash)
	if err != nil {
		return err
	}
	switch mode {
	case "mixed":
		if err := saveIndexFromFiles(files); err != nil {
			return err
		}
	case "hard":
		if err := writeMergedState(files); err != nil {
			return err
		}
	}
	if err := updateHeadWithMessage(commitHash, "reset: moving to "+target); err != nil {
		return err
	}
	fmt.Printf("已 %s reset 到 %s\n", mode, commitHash)
	return nil
}

func commitFiles(commitHash string) ([]object.FileEntry, error) {
	commit, err := object.ReadCommit(commitHash)
	if err != nil {
		return nil, err
	}
	return object.ListTreeFiles(commit.Tree)
}

func saveIndexFromFiles(files []object.FileEntry) error {
	entries := make([]idx.Entry, 0, len(files))
	for _, file := range files {
		entries = append(entries, idx.Entry{Mode: file.Mode, Hash: file.Hash, Path: file.Path})
	}
	return idx.Save(idx.DefaultPath, &idx.File{Entries: entries})
}

const stashPath = ".mygit/stash.json"
const reflogPath = ".mygit/logs/HEAD"

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
func runLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	oneline := fs.Bool("oneline", false, "每个提交只显示一行")
	maxCount := fs.Int("n", 0, "最多显示多少个提交；0 表示不限制")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit log [--oneline] [-n 数量] [commit|分支|标签]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("log 最多接收一个起点")
	}

	start := "HEAD"
	if fs.NArg() == 1 {
		start = fs.Arg(0)
	}
	commitHash, err := resolveCommitish(start)
	if err != nil {
		return err
	}
	return printLog(commitHash, *oneline, *maxCount)
}

func printLog(start string, oneline bool, maxCount int) error {
	queue := []string{start}
	seen := map[string]bool{}
	printed := 0
	for len(queue) > 0 {
		hash := queue[0]
		queue = queue[1:]
		if seen[hash] {
			continue
		}
		seen[hash] = true

		commit, err := object.ReadCommit(hash)
		if err != nil {
			return err
		}
		printLogCommit(hash, commit, oneline)
		printed++
		if maxCount > 0 && printed >= maxCount {
			break
		}
		for _, parent := range commit.Parents {
			if !seen[parent] {
				queue = append(queue, parent)
			}
		}
	}
	return nil
}

func printLogCommit(hash string, commit *object.Commit, oneline bool) {
	if oneline {
		fmt.Printf("%s %s\n", shortHash(hash), firstLine(commit.Message))
		return
	}

	fmt.Printf("commit %s\n", hash)
	if len(commit.Parents) > 1 {
		shortParents := make([]string, 0, len(commit.Parents))
		for _, parent := range commit.Parents {
			shortParents = append(shortParents, shortHash(parent))
		}
		fmt.Printf("Merge: %s\n", strings.Join(shortParents, " "))
	}
	fmt.Printf("Author: %s <%s>\n", commit.Author.Name, commit.Author.Email)
	fmt.Printf("Date:   %s\n\n", commit.Author.When.Format("2006-01-02 15:04:05 -0700"))
	for _, line := range strings.Split(commit.Message, "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
}

func shortHash(hash string) string {
	if len(hash) <= 12 {
		return hash
	}
	return hash[:12]
}

func firstLine(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "(no message)"
	}
	line, _, _ := strings.Cut(message, "\n")
	return line
}
func normalizeMergeArgs(args []string) []string {
	reordered := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-m" {
			reordered = append(reordered, arg)
			if i+1 < len(args) {
				reordered = append(reordered, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-m=") {
			reordered = append(reordered, arg)
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(reordered, positionals...)
}

type mergeEntry struct {
	Entry object.FileEntry
	Ok    bool
}

func runMerge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	message := fs.String("m", "", "合并提交说明")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit merge <分支名|commit哈希> [-m <提交说明>]")
	}
	if err := fs.Parse(normalizeMergeArgs(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("merge 需要一个分支名或 commit 哈希")
	}

	ours, err := readHeadCommit()
	if err != nil {
		return err
	}
	if ours == "" {
		return fmt.Errorf("当前 HEAD 还没有提交，不能 merge")
	}
	theirs, err := resolveCommitish(fs.Arg(0))
	if err != nil {
		return err
	}
	if ours == theirs {
		fmt.Println("已经是最新，无需合并")
		return nil
	}

	base, err := findMergeBase(ours, theirs)
	if err != nil {
		return err
	}
	if base == theirs {
		fmt.Println("已经是最新，无需合并")
		return nil
	}
	if base == ours {
		if err := checkoutCommit(theirs); err != nil {
			return err
		}
		if err := updateHeadWithMessage(theirs, "merge: fast-forward "+fs.Arg(0)); err != nil {
			return err
		}
		fmt.Printf("快进到 %s\n", theirs)
		return nil
	}

	merged, conflicts, err := mergeCommits(base, ours, theirs)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		for _, conflict := range conflicts {
			fmt.Printf("冲突：%s\n", conflict)
		}
		return fmt.Errorf("自动合并失败，请解决冲突后重新 add/commit")
	}

	if err := writeMergedState(merged); err != nil {
		return err
	}
	treeHash, _, err := object.WriteTreeFromFiles(merged)
	if err != nil {
		return err
	}
	mergeMessage := strings.TrimSpace(*message)
	if mergeMessage == "" {
		mergeMessage = fmt.Sprintf("Merge %s into %s", fs.Arg(0), currentBranchNameOrHead())
	}
	sig := defaultSignature()
	commit, err := object.NewCommit(treeHash, []string{ours, theirs}, sig, sig, mergeMessage)
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

func findMergeBase(ours string, theirs string) (string, error) {
	oursAncestors, err := collectAncestors(ours)
	if err != nil {
		return "", err
	}
	queue := []string{theirs}
	seen := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true
		if oursAncestors[current] {
			return current, nil
		}
		commit, err := object.ReadCommit(current)
		if err != nil {
			return "", err
		}
		queue = append(queue, commit.Parents...)
	}
	return "", fmt.Errorf("找不到共同祖先，暂不支持无共同历史的合并")
}

func collectAncestors(start string) (map[string]bool, error) {
	ancestors := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if ancestors[current] {
			continue
		}
		ancestors[current] = true
		commit, err := object.ReadCommit(current)
		if err != nil {
			return nil, err
		}
		queue = append(queue, commit.Parents...)
	}
	return ancestors, nil
}

func mergeCommits(baseHash string, oursHash string, theirsHash string) ([]object.FileEntry, []string, error) {
	base, err := commitFileMap(baseHash)
	if err != nil {
		return nil, nil, err
	}
	ours, err := commitFileMap(oursHash)
	if err != nil {
		return nil, nil, err
	}
	theirs, err := commitFileMap(theirsHash)
	if err != nil {
		return nil, nil, err
	}

	paths := unionPaths(base, ours, theirs)
	merged := make([]object.FileEntry, 0, len(paths))
	conflicts := make([]string, 0)
	for _, path := range paths {
		baseEntry := base[path]
		oursEntry := ours[path]
		theirsEntry := theirs[path]
		result, conflict, err := mergePath(path, baseEntry, oursEntry, theirsEntry)
		if err != nil {
			return nil, nil, err
		}
		if conflict {
			conflicts = append(conflicts, path)
			continue
		}
		if result.Ok {
			merged = append(merged, result.Entry)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Path < merged[j].Path
	})
	return merged, conflicts, nil
}

func mergePath(path string, base mergeEntry, ours mergeEntry, theirs mergeEntry) (mergeEntry, bool, error) {
	if sameEntry(ours, theirs) {
		return ours, false, nil
	}
	if sameEntry(base, ours) {
		return theirs, false, nil
	}
	if sameEntry(base, theirs) {
		return ours, false, nil
	}
	if !base.Ok {
		if !ours.Ok {
			return theirs, false, nil
		}
		if !theirs.Ok {
			return ours, false, nil
		}
	}
	if err := writeConflictFile(path, ours, theirs); err != nil {
		return mergeEntry{}, false, err
	}
	return mergeEntry{}, true, nil
}

func commitFileMap(commitHash string) (map[string]mergeEntry, error) {
	commit, err := object.ReadCommit(commitHash)
	if err != nil {
		return nil, err
	}
	files, err := object.ListTreeFiles(commit.Tree)
	if err != nil {
		return nil, err
	}
	result := make(map[string]mergeEntry, len(files))
	for _, file := range files {
		result[file.Path] = mergeEntry{Entry: file, Ok: true}
	}
	return result, nil
}

func unionPaths(maps ...map[string]mergeEntry) []string {
	seen := map[string]bool{}
	for _, fileMap := range maps {
		for path := range fileMap {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func sameEntry(a mergeEntry, b mergeEntry) bool {
	if a.Ok != b.Ok {
		return false
	}
	if !a.Ok && !b.Ok {
		return true
	}
	return a.Entry.Mode == b.Entry.Mode && a.Entry.Hash == b.Entry.Hash
}

func writeMergedState(files []object.FileEntry) error {
	tracked := map[string]bool{}
	currentIndex, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return err
	}
	for _, entry := range currentIndex.Entries {
		tracked[entry.Path] = true
	}
	entries := make([]idx.Entry, 0, len(files))
	for _, file := range files {
		tracked[file.Path] = false
		if err := restoreWorktreeFile(file); err != nil {
			return err
		}
		entries = append(entries, idx.Entry{Mode: file.Mode, Hash: file.Hash, Path: file.Path})
	}
	for path, shouldRemove := range tracked {
		if shouldRemove {
			if err := os.Remove(filepath.FromSlash(path)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("删除已跟踪文件失败 %s：%w", path, err)
			}
		}
	}
	return idx.Save(idx.DefaultPath, &idx.File{Entries: entries})
}

func writeConflictFile(path string, ours mergeEntry, theirs mergeEntry) error {
	oursContent, err := mergeEntryContent(ours)
	if err != nil {
		return err
	}
	theirsContent, err := mergeEntryContent(theirs)
	if err != nil {
		return err
	}
	var buffer bytes.Buffer
	buffer.WriteString("<<<<<<< HEAD\n")
	buffer.Write(oursContent)
	if len(oursContent) > 0 && oursContent[len(oursContent)-1] != '\n' {
		buffer.WriteByte('\n')
	}
	buffer.WriteString("=======\n")
	buffer.Write(theirsContent)
	if len(theirsContent) > 0 && theirsContent[len(theirsContent)-1] != '\n' {
		buffer.WriteByte('\n')
	}
	buffer.WriteString(">>>>>>> MERGE_HEAD\n")
	worktreePath := filepath.FromSlash(path)
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return fmt.Errorf("创建冲突文件目录失败：%w", err)
	}
	return os.WriteFile(worktreePath, buffer.Bytes(), 0o644)
}

func mergeEntryContent(entry mergeEntry) ([]byte, error) {
	if !entry.Ok {
		return nil, nil
	}
	blob, err := object.ReadBlob(entry.Entry.Hash)
	if err != nil {
		return nil, err
	}
	return blob.Content, nil
}

func currentBranchNameOrHead() string {
	name := currentBranchName()
	if name != "" {
		return name
	}
	return "HEAD"
}
func runRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	source := fs.String("source", "HEAD", "从指定 commit、分支或标签恢复")
	staged := fs.Bool("staged", false, "恢复 index")
	worktree := fs.Bool("worktree", false, "恢复工作区")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit restore [--source <commit|分支|标签>] [--staged] [--worktree] <路径> [更多路径]")
		fmt.Fprintln(os.Stderr, "示例：mgit restore README.md")
		fmt.Fprintln(os.Stderr, "      mgit restore --staged README.md")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("restore 需要至少一个路径")
	}

	restoreWorktree := *worktree
	restoreIndex := *staged
	if !restoreWorktree && !restoreIndex {
		restoreWorktree = true
	}

	commitHash, err := resolveCommitish(*source)
	if err != nil {
		return err
	}
	restored, err := restoreFromCommit(commitHash, fs.Args(), restoreWorktree, restoreIndex)
	if err != nil {
		return err
	}
	if restoreWorktree && restoreIndex {
		fmt.Printf("已恢复 %d 个文件到工作区和 index\n", restored)
	} else if restoreIndex {
		fmt.Printf("已恢复 %d 个文件到 index\n", restored)
	} else {
		fmt.Printf("已恢复 %d 个文件到工作区\n", restored)
	}
	return nil
}

func restoreFromCommit(commitHash string, paths []string, restoreWorktree bool, restoreIndex bool) (int, error) {
	commit, err := object.ReadCommit(commitHash)
	if err != nil {
		return 0, err
	}
	files, err := object.ListTreeFiles(commit.Tree)
	if err != nil {
		return 0, err
	}
	selected, pathspecs, err := selectRestoreFiles(files, paths)
	if err != nil {
		return 0, err
	}
	if len(selected) == 0 {
		return 0, fmt.Errorf("没有匹配到可恢复的文件")
	}

	if restoreWorktree {
		for _, file := range selected {
			if err := restoreWorktreeFile(file); err != nil {
				return 0, err
			}
		}
	}
	if restoreIndex {
		if err := restoreIndexEntries(selected, pathspecs); err != nil {
			return 0, err
		}
	}
	return len(selected), nil
}

func selectRestoreFiles(files []object.FileEntry, paths []string) ([]object.FileEntry, []string, error) {
	pathspecs := make([]string, 0, len(paths))
	for _, path := range paths {
		pathspec := normalizeWorktreePath(path)
		if pathspec == "" {
			return nil, nil, fmt.Errorf("restore 路径不能为空")
		}
		pathspecs = append(pathspecs, pathspec)
	}

	selected := make([]object.FileEntry, 0)
	matched := make([]bool, len(pathspecs))
	for _, file := range files {
		for i, pathspec := range pathspecs {
			if restorePathMatches(file.Path, pathspec) {
				selected = append(selected, file)
				matched[i] = true
				break
			}
		}
	}
	for i, ok := range matched {
		if !ok {
			return nil, nil, fmt.Errorf("源 commit 中没有路径：%s", paths[i])
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Path < selected[j].Path
	})
	return selected, pathspecs, nil
}

func restoreWorktreeFile(file object.FileEntry) error {
	blob, err := object.ReadBlob(file.Hash)
	if err != nil {
		return err
	}
	path := filepath.FromSlash(file.Path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建目录失败：%w", err)
	}
	if err := os.WriteFile(path, blob.Content, 0o644); err != nil {
		return fmt.Errorf("恢复工作区文件失败 %s：%w", file.Path, err)
	}
	return nil
}

func restoreIndexEntries(files []object.FileEntry, pathspecs []string) error {
	indexFile, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return err
	}
	kept := indexFile.Entries[:0]
	for _, entry := range indexFile.Entries {
		matched := false
		for _, pathspec := range pathspecs {
			if restorePathMatches(entry.Path, pathspec) {
				matched = true
				break
			}
		}
		if !matched {
			kept = append(kept, entry)
		}
	}
	indexFile.Entries = kept
	for _, file := range files {
		indexFile.Entries = append(indexFile.Entries, idx.Entry{Mode: file.Mode, Hash: file.Hash, Path: file.Path})
	}
	indexFile.Sort()
	return idx.Save(idx.DefaultPath, indexFile)
}

func normalizeWorktreePath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return "."
	}
	return strings.Trim(path, "/")
}

func restorePathMatches(filePath string, pathspec string) bool {
	filePath = normalizeWorktreePath(filePath)
	pathspec = normalizeWorktreePath(pathspec)
	if pathspec == "." {
		return true
	}
	return filePath == pathspec || strings.HasPrefix(filePath, pathspec+"/")
}
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

  mgit rm [--cached] [-r] [-f] <路径> [更多路径]
      从 index 移除路径，并默认删除对应工作区文件

  mgit status
      查看 HEAD、index 和工作区之间的状态

  mgit ls-files [-s]
      查看 index 中已暂存的路径；加 -s 显示 mode 和 hash

  mgit hash-object [-w] <文件>
      计算文件的 blob 哈希；加 -w 时写入对象库

  mgit cat-file (-p|-t|-s) <对象哈希>
      查看对象内容、类型或大小

  mgit diff [--staged] [commit|分支|标签]
      查看工作区、index 或指定提交之间的文本差异

  mgit write-tree
      根据 index 写成 tree 对象

  mgit write-tree --worktree [目录]
      直接把工作区目录写成 tree 对象

  mgit commit-tree <tree哈希> [-p 父提交哈希] -m <提交说明>
      基于 tree 创建 commit 对象，只打印哈希，不更新 HEAD

  mgit commit [-m <提交说明>]
      根据 index 创建 commit，并更新 HEAD 指向的分支

  mgit reset [--soft|--mixed|--hard] [commit|分支|标签]
      移动 HEAD，并按模式重置 index 或工作区

  mgit stash [push [-m <说明>]]
      保存当前已跟踪文件的工作区和 index 状态

  mgit stash list|apply|pop|drop [stash@{n}|n]
      查看、应用、弹出或删除 stash

  mgit merge <分支名|commit哈希> [-m <提交说明>]
      合并分支或 commit，支持 fast-forward 和简单三方合并

  mgit log [--oneline] [-n 数量] [commit|分支|标签]
      从 HEAD 或指定起点显示提交历史

  mgit reflog [-n 数量]
      查看 HEAD 最近移动记录

  mgit branch
      列出本地分支

  mgit branch <分支名> [commit哈希]
      基于当前 HEAD 或指定 commit 创建分支

  mgit checkout <分支名|commit哈希>
      切换分支或检出 commit，并恢复 index 与工作区文件

  mgit restore [--source <commit|分支|标签>] [--staged] [--worktree] <路径>
      从 HEAD 或指定来源恢复路径；默认只恢复工作区

  mgit tag [-a] [-m <标签说明>] <标签名> <对象哈希>
      创建轻量标签；加 -a 或 -m 时创建注解 tag 对象

  mgit tag
      列出已有标签`)
}
