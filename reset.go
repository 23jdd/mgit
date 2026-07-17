package main

import (
	"flag"
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"os"
)

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
