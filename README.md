# mgit

mgit 是一个用 Go 实现的迷你 Git，用来学习 Git 对象模型、对象存储格式、index 暂存区、提交对象、分支切换和标签引用。

当前支持：

- `init`：初始化 `.mygit` 仓库目录。
- `add`：把文件写成 blob，并记录到 `.mygit/index`。
- `ls-files`：查看 index 中已经暂存的文件。
- `hash-object`：按 Git blob 格式计算文件 SHA-1，可选择写入对象库。
- `cat-file`：查看对象类型、大小或内容。
- `write-tree`：默认根据 index 写成 tree 对象，也可以用 `--worktree` 直接扫描目录。
- `commit-tree`：基于已有 tree 创建 commit 对象。
- `commit`：根据 index 创建 commit，并更新 HEAD 指向的分支。
- `reset`：移动 HEAD，并按 `--soft`、`--mixed` 或 `--hard` 重置 index/工作区。
- `merge`：合并分支、标签或 commit，支持 fast-forward 和简单三方自动合并。
- `log`：从 `HEAD`、分支、标签或 commit 显示提交历史。
- `branch`：列出本地分支，或基于当前 HEAD/指定 commit 创建分支。
- `checkout`：切换到分支，或检出 commit 到 detached HEAD。
- `restore`：从 `HEAD` 或指定 commit/分支/标签恢复路径到工作区或 index。
- `tag`：创建轻量标签或注解标签，也可以列出标签。

## 使用

初始化仓库：

```powershell
go run . init
```

暂存文件，相当于迷你版 `git add .`：

```powershell
go run . add .
go run . ls-files
go run . ls-files -s
```

写入和查看 blob：

```powershell
$blob = go run . hash-object -w README.md
go run . cat-file -t $blob
go run . cat-file -s $blob
go run . cat-file -p $blob
```

根据 index 写入 tree：

```powershell
$tree = go run . write-tree
go run . cat-file -p $tree
```

也可以绕过 index，直接从工作区目录写 tree：

```powershell
$tree = go run . write-tree --worktree .
```

创建 commit：

```powershell
go run . add .
$commit = go run . commit -m "first commit"
go run . cat-file -p $commit
```

也可以只基于 tree 创建 commit 对象，不更新 HEAD：

```powershell
$tree = go run . write-tree
$commit = go run . commit-tree $tree -m "manual commit"
```

重置到指定提交：

```powershell
# 默认 --mixed：移动 HEAD，并把 index 重置到目标 commit，工作区不变
go run . reset $commit

# 只移动 HEAD，不改 index 和工作区
go run . reset --soft $commit

# 移动 HEAD，并把 index 和工作区都重置到目标 commit
go run . reset --hard $commit
```

`reset` 默认目标是 `HEAD`，所以 `go run . reset --hard` 可以把 index 和工作区恢复到当前 HEAD。`--hard` 会删除当前 index 中已跟踪、但目标 commit 中不存在的文件；不会主动清理未跟踪文件。

合并分支或 commit：

```powershell
# 把 dev 合并到当前分支
go run . merge dev

# 合并指定 commit，并自定义提交说明
go run . merge $commit -m "merge experiment"
```

`merge` 会先寻找共同祖先：如果可以 fast-forward，就直接移动当前分支；否则会做三方合并并创建双父提交。双方修改不同路径时会自动合并；同一路径双方都修改且内容不同，会在工作区写入冲突标记并停止提交。

查看提交历史：

```powershell
# 从 HEAD 开始显示完整日志
go run . log

# 一行一个提交
go run . log --oneline

# 限制显示数量
go run . log -n 3

# 从指定分支、标签或 commit 开始显示
go run . log dev
```

`log` 会遍历 merge commit 的所有 parent，并自动去重；完整格式会显示 commit、Merge、Author、Date 和提交说明。

创建和查看分支：

```powershell
# 列出分支，当前分支前会显示 *
go run . branch

# 基于当前 HEAD 创建分支
go run . branch dev

# 基于指定 commit 创建分支
go run . branch experiment $commit
```

切换分支或检出 commit：

```powershell
# 切换到分支，并恢复该分支 commit 对应的文件与 index
go run . checkout dev

# 检出指定 commit，HEAD 会进入 detached 状态
go run . checkout $commit
```

`checkout` 会写回目标 commit 中记录的文件，并同步 `.mygit/index`；为了避免误删，它不会主动删除工作区中不属于目标 tree 的额外文件。

恢复文件：

```powershell
# 从 HEAD 恢复 README.md 到工作区，默认不改 index
go run . restore README.md

# 从指定 commit 恢复 README.md 到工作区
go run . restore --source $commit README.md

# 只恢复 index，相当于取消暂存到 HEAD 版本
go run . restore --staged README.md

# 同时恢复工作区和 index
go run . restore --worktree --staged README.md

# 恢复整个目录
go run . restore object
```

`restore` 只覆盖源 commit 中存在的目标路径；为了避免误删，它不会删除目标之外的额外工作区文件。

创建标签：

```powershell
# 轻量标签，refs/tags/v0.1 直接指向 commit
go run . tag v0.1 $commit

# 注解标签，会创建 tag 对象，refs/tags/v0.1.0 指向该 tag 对象
go run . tag -a -m "release v0.1.0" v0.1.0 $commit

# 列出标签
go run . tag
```

也可以先构建二进制：

```powershell
go build .
.\mgit.exe init
.\mgit.exe add .
$commit = .\mgit.exe commit -m "first commit"
.\mgit.exe branch dev $commit
.\mgit.exe checkout dev
.\mgit.exe reset --hard $commit
.\mgit.exe tag v0.1 $commit
```

## index 格式

`.mygit/index` 使用 JSON 保存暂存条目，方便阅读和调试：

```json
{
  "entries": [
    {
      "mode": "100644",
      "hash": "<blob哈希>",
      "path": "README.md"
    }
  ]
}
```

当前 index 只记录普通文件，路径统一使用 `/`。执行 `add .` 时会跳过 `.git`、`.mygit`、`.gocache`、`.agents` 和 `.codex`。

## 对象目录

mgit 不写入真实 `.git`，而是使用项目内的 `.mygit`：

```text
.mygit/
  HEAD
  index
  objects/
    前 2 位哈希/
      后 38 位哈希
  refs/
    heads/
      main
      dev
    tags/
      v0.1
```

对象内容使用和 Git 相同的基本格式：

```text
<type> <size>\0<payload>
```

写入磁盘前会使用 zlib 压缩。

## 身份信息

提交和注解标签会读取以下环境变量作为作者信息：

- `MGIT_AUTHOR_NAME`
- `MGIT_AUTHOR_EMAIL`
- `GIT_AUTHOR_NAME`
- `GIT_AUTHOR_EMAIL`

没有设置时，会使用当前 Windows 用户名和 `mgit@example.local`。