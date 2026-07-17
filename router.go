package main

import (
	"fmt"
)

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
