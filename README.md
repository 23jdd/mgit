# mgit

mgit 是一个用 Go 实现的迷你 Git，用来学习 Git 对象模型、对象存储格式、index 暂存区、提交对象、分支切换和标签引用。

当前支持：

- `init`：初始化 `.git` 仓库目录。
- `add`：把文件写成 blob，并记录到 `.git/index`。
- `.gitignore`：`add`、`status` 和工作区 tree 扫描会跳过忽略路径。
- `rm`：从 index 移除路径，并默认删除对应工作区文件。
- `status`：查看 HEAD、index 和工作区之间的状态。
- `ls-files`：查看 index 中已经暂存的文件。
- `hash-object`：按 Git blob 格式计算文件 SHA-1，可选择写入对象库。
- `cat-file`：查看对象类型、大小或内容。
- `diff`：查看工作区、index 或提交之间的文本差异。
- `write-tree`：默认根据 index 写成 tree 对象，也可以用 `--worktree` 直接扫描目录。
- `commit-tree`：基于已有 tree 创建 commit 对象。
- `commit`：根据 index 创建 commit，并更新 HEAD 指向的分支。
- `reset`：移动 HEAD，并按 `--soft`、`--mixed` 或 `--hard` 重置 index/工作区。
- `stash`：保存、查看、应用或弹出已跟踪文件的临时工作状态。
- `merge`：合并分支、标签或 commit，支持 fast-forward 和简单三方自动合并。
- `log`：从 `HEAD`、分支、标签或 commit 显示提交历史。
- `reflog`：查看 HEAD 的最近移动记录。
- `branch`：列出本地分支，或基于当前 HEAD/指定 commit 创建分支。
- `checkout`：切换到分支，或检出 commit 到 detached HEAD。
- `restore`：从 `HEAD` 或指定 commit/分支/标签恢复路径到工作区或 index。
- `tag`：创建轻量标签或注解标签，也可以列出标签。

## 使用

初始化仓库：

```powershell
mgit init
```

暂存文件，相当于迷你版 `git add .`：

```powershell
mgit add .
mgit ls-files
mgit ls-files -s
```

忽略文件：

```powershell
# .gitignore 示例
*.log
build/
/temp.txt
!important.log
```

`mgit add .`、`mgit status` 的未跟踪文件扫描、`mgit write-tree --worktree` 和 `mgit commit --worktree` 都会读取仓库根目录的 `.gitignore`。当前支持空行、`#` 注释、`!` 反选、目录规则、根路径规则和 `*` / `?` 通配；`.git`、`.gocache`、`.agents`、`.codex` 始终会被跳过。

查看状态：

```powershell
# 查看 HEAD、index 和工作区之间的差异状态
mgit status
```

`status` 会分组显示“要提交的变更”“尚未暂存的变更”和“未跟踪文件”。

移除已跟踪路径：

```powershell
# 从 index 移除并删除工作区文件
mgit rm README.md

# 只从 index 移除，保留工作区文件
mgit rm --cached README.md

# 移除目录下已跟踪文件
mgit rm -r docs
```

`rm` 只处理 index 中已经跟踪的路径；删除目录或路径前缀下的多个文件时需要 `-r`。

写入和查看 blob：

```powershell
$blob = mgit hash-object -w README.md
mgit cat-file -t $blob
mgit cat-file -s $blob
mgit cat-file -p $blob
```

根据 index 写入 tree：

```powershell
$tree = mgit write-tree
mgit cat-file -p $tree
```

也可以绕过 index，直接从工作区目录写 tree：

```powershell
$tree = mgit write-tree --worktree .
```

创建 commit：

```powershell
mgit add .
$commit = mgit commit -m "first commit"
mgit cat-file -p $commit
```

也可以只基于 tree 创建 commit 对象，不更新 HEAD：

```powershell
$tree = mgit write-tree
$commit = mgit commit-tree $tree -m "manual commit"
```

查看差异：

```powershell
# 查看工作区相对 index 的差异
mgit diff

# 查看 index 相对 HEAD 的差异
mgit diff --staged

# 查看工作区相对指定 commit 的差异
mgit diff $commit
```

`diff` 会输出简化版文本差异，只比较当前 index 已跟踪的工作区文件；`--staged` 用于比较指定 commit（默认 `HEAD`）和 index。

重置到指定提交：

```powershell
# 默认 --mixed：移动 HEAD，并把 index 重置到目标 commit，工作区不变
mgit reset $commit

# 只移动 HEAD，不改 index 和工作区
mgit reset --soft $commit

# 移动 HEAD，并把 index 和工作区都重置到目标 commit
mgit reset --hard $commit
```

`reset` 默认目标是 `HEAD`，所以 `mgit reset --hard` 可以把 index 和工作区恢复到当前 HEAD。`--hard` 会删除当前 index 中已跟踪、但目标 commit 中不存在的文件；不会主动清理未跟踪文件。

临时保存工作状态：

```powershell
# 保存当前已跟踪文件的工作区和 index 状态，并恢复到 HEAD
mgit stash push -m "try something"

# 查看 stash 列表
mgit stash list

# 应用但保留 stash
mgit stash apply stash@{0}

# 应用并删除 stash
mgit stash pop

# 删除 stash
mgit stash drop stash@{0}
```

`stash` 会保存当前 index 中已跟踪路径的工作区快照和 index 状态；它不会主动保存未跟踪文件。

合并分支或 commit：

```powershell
# 把 dev 合并到当前分支
mgit merge dev

# 合并指定 commit，并自定义提交说明
mgit merge $commit -m "merge experiment"
```

`merge` 会先寻找共同祖先：如果可以 fast-forward，就直接移动当前分支；否则会做三方合并并创建双父提交。双方修改不同路径时会自动合并；同一路径双方都修改且内容不同，会在工作区写入冲突标记并停止提交。

查看提交历史：

```powershell
# 从 HEAD 开始显示完整日志
mgit log

# 一行一个提交
mgit log --oneline

# 限制显示数量
mgit log -n 3

# 从指定分支、标签或 commit 开始显示
mgit log dev
```

`log` 会遍历 merge commit 的所有 parent，并自动去重；完整格式会显示 commit、Merge、Author、Date 和提交说明。

查看 HEAD 移动记录：

```powershell
# 查看最近的 HEAD 变动
mgit reflog

# 限制显示数量
mgit reflog -n 5
```

`reflog` 读取 `.git/logs/HEAD`，会记录 `commit`、`reset`、`merge` 等通过 `updateHead` 移动 HEAD 的操作。

创建和查看分支：

```powershell
# 列出分支，当前分支前会显示 *
mgit branch

# 基于当前 HEAD 创建分支
mgit branch dev

# 基于指定 commit 创建分支
mgit branch experiment $commit
```

切换分支或检出 commit：

```powershell
# 切换到分支，并恢复该分支 commit 对应的文件与 index
mgit checkout dev

# 检出指定 commit，HEAD 会进入 detached 状态
mgit checkout $commit
```

`checkout` 会写回目标 commit 中记录的文件，并同步 `.git/index`；为了避免误删，它不会主动删除工作区中不属于目标 tree 的额外文件。

恢复文件：

```powershell
# 从 HEAD 恢复 README.md 到工作区，默认不改 index
mgit restore README.md

# 从指定 commit 恢复 README.md 到工作区
mgit restore --source $commit README.md

# 只恢复 index，相当于取消暂存到 HEAD 版本
mgit restore --staged README.md

# 同时恢复工作区和 index
mgit restore --worktree --staged README.md

# 恢复整个目录
mgit restore object
```

`restore` 只覆盖源 commit 中存在的目标路径；为了避免误删，它不会删除目标之外的额外工作区文件。

创建标签：

```powershell
# 轻量标签，refs/tags/v0.1 直接指向 commit
mgit tag v0.1 $commit

# 注解标签，会创建 tag 对象，refs/tags/v0.1.0 指向该 tag 对象
mgit tag -a -m "release v0.1.0" v0.1.0 $commit

# 列出标签
mgit tag
```

也可以先构建二进制：

```powershell
go build .
mgit init
mgit add .
mgit status
$commit = mgit commit -m "first commit"
mgit branch dev $commit
mgit checkout dev
mgit reset --hard $commit
mgit tag v0.1 $commit
```

## index 格式

`.git/index` 使用 JSON 保存暂存条目，方便阅读和调试：

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

当前 index 只记录普通文件，路径统一使用 `/`。执行 `add .` 时会跳过 `.gitignore` 匹配的路径，以及 `.git`、`.git`、`.gocache`、`.agents` 和 `.codex`。

## 对象目录

mgit 使用项目内的 `.git` 目录保存对象、引用和 index：

```text
.git/
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