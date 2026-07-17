# mgit

mgit 是一个用 Go 实现的迷你 Git，用来学习 Git 对象、index、commit、branch、merge、stash、reflog 等核心机制。仓库数据保存在当前目录的 `.git` 中，不依赖系统 Git。

## Quick Start

```powershell
go build -o mgit.exe .
mgit init
mgit add .
mgit status
$commit = mgit commit -m "first commit"
mgit log --oneline
```

## Commands

| 命令 | 说明 |
| --- | --- |
| `mgit init` | 初始化 `.git` 目录 |
| `mgit add <路径>` | 写入 blob，并更新 `.git/index` |
| `mgit rm [--cached] [-r] <路径>` | 从 index 移除路径，默认也删除工作区文件 |
| `mgit status` | 查看 HEAD、index、工作区状态 |
| `mgit ls-files [-s]` | 查看 index 中的路径；`-s` 显示 mode/hash |
| `mgit diff [--staged] [commit]` | 查看工作区、index 或 commit 之间的文本差异 |
| `mgit hash-object [-w] <文件>` | 计算 blob 哈希；`-w` 写入对象库 |
| `mgit cat-file (-p|-t|-s) <hash>` | 查看对象内容、类型或大小 |
| `mgit write-tree [--worktree]` | 从 index 或工作区写 tree 对象 |
| `mgit commit [-m msg]` | 从 index 创建 commit 并更新 HEAD |
| `mgit commit-tree <tree> [-p parent] -m msg` | 只创建 commit 对象，不更新 HEAD |
| `mgit branch [name] [commit]` | 列出分支或创建分支 |
| `mgit checkout <branch|commit>` | 切换分支或检出 commit |
| `mgit restore [--source rev] [--staged] [--worktree] <路径>` | 从 commit 恢复路径 |
| `mgit reset [--soft|--mixed|--hard] [rev]` | 移动 HEAD，并按模式重置 index/工作区 |
| `mgit merge <branch|commit> [-m msg]` | fast-forward 或简单三方合并 |
| `mgit stash push/list/apply/pop/drop` | 保存和恢复临时工作状态 |
| `mgit log [--oneline] [-n N] [rev]` | 查看提交历史 |
| `mgit reflog [-n N]` | 查看 HEAD 移动记录 |
| `mgit tag [-a] [-m msg] <name> <hash>` | 创建轻量或注解标签；无参数时列出标签 |

## Common Flow

```powershell
mgit init
mgit add .
mgit commit -m "first commit"
mgit branch dev
mgit checkout dev
mgit status
mgit diff
mgit merge main
```

## .gitignore

`mgit add .`、`mgit status`、`mgit write-tree --worktree` 和 `mgit commit --worktree` 会读取仓库根目录的 `.gitignore`。

支持：空行、`#` 注释、`!` 反选、目录规则、根路径规则，以及 `*` / `?` 通配。

```gitignore
*.log
build/
/temp.txt
!important.log
```

内置跳过：`.git`。

## Storage

```text
.git/
  HEAD
  index
  objects/
    前 2 位哈希/
      后 38 位哈希
  refs/
    heads/
    tags/
  logs/
    HEAD
```

对象格式与 Git 的基础对象格式一致：

```text
<type> <size>\0<payload>
```

`.git/index` 使用 JSON 保存，方便阅读和调试。

## Notes

- 当前只支持普通文件，路径统一使用 `/`。
- `merge` 支持 fast-forward 和简单三方合并；冲突时写入冲突标记并停止。
- `reset --hard` 会删除已跟踪但目标 commit 不存在的文件，不清理未跟踪文件。
- `stash` 只保存当前 index 中已跟踪路径的工作区快照。
- 提交和注解标签会读取 `MGIT_AUTHOR_NAME`、`MGIT_AUTHOR_EMAIL`、`GIT_AUTHOR_NAME`、`GIT_AUTHOR_EMAIL`。