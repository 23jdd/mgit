# mgit

mgit 是一个用 Go 写的迷你 Git，用来学习对象库、index、commit、branch、checkout、merge、stash、reflog 等核心机制。

## 快速开始

```powershell
go build -o mgit.exe .
mgit init
mgit config user.name "Your Name"
mgit config user.email "you@example.com"
mgit add .
mgit status
$commit = mgit commit -m "first commit"
mgit log --oneline
```

也可以安装到 Go bin：

```powershell
go install .
mgit help
```

## 重要说明

mgit 默认在当前目录写入自己的仓库数据：

- 普通空目录：使用 `.git`
- 已经是真实 Git 仓库的目录：自动改用 `.mygit`
- 也可以用环境变量 `MGIT_DIR` 指定目录

这样可以避免 mgit 覆盖真实 Git 的 `.git/index`、`.git/objects` 和 refs。本项目源码本身就是 Git 仓库，所以在这里运行 `mgit add .` 会使用 `.mygit`。

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `mgit init` | 初始化 mgit 仓库目录 |
| `mgit add <路径>` | 写入 blob，并更新 index |
| `mgit config [--list]` | 查看或设置配置，例如 `user.name`、`user.email` |
| `mgit rm [--cached] [-r] <路径>` | 从 index 移除路径，默认也删除工作区文件 |
| `mgit status` | 查看 HEAD、index、工作区状态 |
| `mgit ls-files [-s]` | 查看 index 中的路径；`-s` 显示 mode/hash |
| `mgit diff [--staged] [rev]` | 查看工作区、index 或提交之间的文本差异 |
| `mgit commit [-m msg]` | 从 index 创建 commit 并更新 HEAD |
| `mgit branch [name] [rev]` | 列出分支或创建分支 |
| `mgit checkout <branch|rev>` | 切换分支或检出提交，并恢复 index/工作区 |
| `mgit restore [--source rev] [--staged] [--worktree] <路径>` | 从提交恢复路径 |
| `mgit reset [--soft|--mixed|--hard] [rev]` | 移动 HEAD，并按模式重置 index/工作区 |
| `mgit merge <branch|rev> [-m msg]` | fast-forward 或简单三方合并 |
| `mgit stash push/list/apply/pop/drop` | 保存和恢复临时工作状态 |
| `mgit log [--oneline] [-n N] [rev]` | 查看提交历史 |
| `mgit reflog [-n N]` | 查看 HEAD 移动记录 |
| `mgit tag [-a] [-m msg] <name> <hash>` | 创建轻量或注解标签；无参数时列出标签 |

更多参数运行：

```powershell
mgit help
```


## Config

```powershell
mgit config user.name "Alice"
mgit config user.email "alice@example.com"
mgit config user.name
mgit config --list
mgit config --unset user.email
```

默认写入当前 mgit 仓库的 `config`。加 `--global` 时写入用户目录下的 `.mgitconfig`。提交和注解标签会优先读取环境变量，其次读取 `user.name`、`user.email`。
## .gitignore

`mgit add .`、`mgit status`、`mgit write-tree --worktree` 和 `mgit commit --worktree` 会读取仓库根目录的 `.gitignore`。

支持空行、`#` 注释、`!` 反选、目录规则、根路径规则，以及 `*` / `?` 通配。

内置跳过：`.git`、`.mygit`、`.gocache`、`.agents`、`.codex`。

## 存储结构

```text
.git/ 或 .mygit/
  HEAD
  index
  objects/
  refs/
    heads/
    tags/
  logs/
    HEAD
```

对象格式与 Git 基础对象格式一致：

```text
<type> <size>\0<payload>
```

`index` 使用 JSON 保存，方便阅读和调试。
