package main

import (
	"flag"
	"fmt"
	"github.com/23jdd/mgit/object"
	"os"
	"strings"
)

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
