package main

import (
	"fmt"
)

type command struct {
	Name        string
	Usage       []string
	Description string
	Run         func([]string) error
}

var commands = []command{
	{Name: "init", Usage: []string{"mgit init"}, Description: "初始化 mgit 仓库；空目录使用 .git，已有真实 Git 仓库时使用 .mygit", Run: func(args []string) error { return runInit() }},
	{Name: "add", Usage: []string{"mgit add <路径> [更多路径]"}, Description: "把文件写成 blob，并记录到 mgit index", Run: runAdd},
	{Name: "config", Usage: []string{"mgit config [--local|--global] [--list]"}, Description: "查看或设置配置，例如 user.name、user.email", Run: runConfig},
	{Name: "rm", Usage: []string{"mgit rm [--cached] [-r] [-f] <路径> [更多路径]"}, Description: "从 index 移除路径，并默认删除对应工作区文件", Run: runRm},
	{Name: "status", Usage: []string{"mgit status"}, Description: "查看 HEAD、index 和工作区之间的状态", Run: runStatus},
	{Name: "ls-files", Usage: []string{"mgit ls-files [-s]"}, Description: "查看 index 中已暂存的路径；加 -s 显示 mode 和 hash", Run: runLsFiles},
	{Name: "hash-object", Usage: []string{"mgit hash-object [-w] <文件>"}, Description: "计算文件的 blob 哈希；加 -w 时写入对象库", Run: runHashObject},
	{Name: "cat-file", Usage: []string{"mgit cat-file (-p|-t|-s) <对象哈希>"}, Description: "查看对象内容、类型或大小", Run: runCatFile},
	{Name: "diff", Usage: []string{"mgit diff [--staged] [commit|分支|标签]"}, Description: "查看工作区、index 或指定提交之间的文本差异", Run: runDiff},
	{Name: "write-tree", Usage: []string{"mgit write-tree", "mgit write-tree --worktree [目录]"}, Description: "根据 index 或工作区写成 tree 对象", Run: runWriteTree},
	{Name: "commit-tree", Usage: []string{"mgit commit-tree <tree哈希> [-p 父提交哈希] -m <提交说明>"}, Description: "基于 tree 创建 commit 对象，只打印哈希，不更新 HEAD", Run: runCommitTree},
	{Name: "commit", Usage: []string{"mgit commit [-m <提交说明>]", "mgit commit --worktree [-m <提交说明>] [目录]"}, Description: "创建 commit，并更新 HEAD 指向的分支", Run: runCommit},
	{Name: "reset", Usage: []string{"mgit reset [--soft|--mixed|--hard] [commit|分支|标签]"}, Description: "移动 HEAD，并按模式重置 index 或工作区", Run: runReset},
	{Name: "stash", Usage: []string{"mgit stash [push [-m <说明>]]", "mgit stash list|apply|pop|drop [stash@{n}|n]"}, Description: "保存、查看、应用或删除临时工作状态", Run: runStash},
	{Name: "merge", Usage: []string{"mgit merge <分支名|commit哈希> [-m <提交说明>]"}, Description: "合并分支或 commit，支持 fast-forward 和简单三方合并", Run: runMerge},
	{Name: "log", Usage: []string{"mgit log [--oneline] [-n 数量] [commit|分支|标签]"}, Description: "从 HEAD 或指定起点显示提交历史", Run: runLog},
	{Name: "reflog", Usage: []string{"mgit reflog [-n 数量]"}, Description: "查看 HEAD 最近移动记录", Run: runReflog},
	{Name: "branch", Usage: []string{"mgit branch", "mgit branch <分支名> [commit哈希]"}, Description: "列出本地分支，或基于当前 HEAD/指定 commit 创建分支", Run: runBranch},
	{Name: "checkout", Usage: []string{"mgit checkout <分支名|commit哈希>"}, Description: "切换分支或检出 commit，并恢复 index 与工作区文件", Run: runCheckout},
	{Name: "restore", Usage: []string{"mgit restore [--source <commit|分支|标签>] [--staged] [--worktree] <路径>"}, Description: "从 HEAD 或指定来源恢复路径；默认只恢复工作区", Run: runRestore},
	{Name: "tag", Usage: []string{"mgit tag [-a] [-m <标签说明>] <标签名> <对象哈希>", "mgit tag"}, Description: "创建轻量或注解标签，或列出已有标签", Run: runTag},
}

func run(args []string) error {
	if len(args) < 2 {
		printHelp()
		return nil
	}

	name := args[1]
	switch name {
	case "help", "-h", "--help":
		printHelp()
		return nil
	}

	cmd, ok := findCommand(name)
	if !ok {
		return fmt.Errorf("未知命令 %q，运行 mgit help 查看用法", name)
	}
	return cmd.Run(args[2:])
}

func findCommand(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd, true
		}
	}
	return command{}, false
}
