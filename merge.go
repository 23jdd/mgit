package main

import (
	"bytes"
	"flag"
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
