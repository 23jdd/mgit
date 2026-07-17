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
