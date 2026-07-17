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
- `branch`：列出本地分支，或基于当前 HEAD/指定 commit 创建分支。
- `checkout`：切换到分支，或检出 commit 到 detached HEAD。
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