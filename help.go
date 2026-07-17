package main

import (
	"fmt"
)

func printHelp() {
	fmt.Println(`mgit - 用 Go 实现的迷你 Git

用法：
  mgit init
      初始化 mgit 仓库；空目录使用 .git，已有真实 Git 仓库时使用 .mygit

  mgit add <路径> [更多路径]
      把文件写成 blob，并记录到 mgit index

  mgit config [--local|--global] [--list]
      查看或设置配置，例如 user.name、user.email

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
