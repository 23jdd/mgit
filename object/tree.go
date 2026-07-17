package object

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TreeEntry struct {
	Mode       string
	Name       string
	Hash       string
	ObjectType string
}

type FileEntry struct {
	Mode string
	Path string
	Hash string
}

type Tree struct {
	Entries []TreeEntry
}

type treeNode struct {
	Files []TreeEntry
	Dirs  map[string]*treeNode
}

func NewTree(entries []TreeEntry) *Tree {
	copied := make([]TreeEntry, len(entries))
	copy(copied, entries)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i].Name < copied[j].Name
	})
	return &Tree{Entries: copied}
}

func (t *Tree) Type() string {
	return "tree"
}

func (t *Tree) Payload() []byte {
	var payload bytes.Buffer
	for _, entry := range t.Entries {
		payload.WriteString(entry.Mode)
		payload.WriteByte(' ')
		payload.WriteString(entry.Name)
		payload.WriteByte(0)

		hashBytes, err := hex.DecodeString(entry.Hash)
		if err != nil || len(hashBytes) != 20 {
			continue
		}
		payload.Write(hashBytes)
	}
	return payload.Bytes()
}

func (t *Tree) Size() int {
	return len(t.Payload())
}

func (t *Tree) Raw() []byte {
	return RawObject(t)
}

func (t *Tree) HashString() string {
	return HashObject(t)
}

func (t *Tree) Write() (string, error) {
	return WriteObject(t)
}

func TreeFromStored(stored *StoredObject) (*Tree, error) {
	if stored.ObjectType != "tree" {
		return nil, ErrUnexpectedType("tree", stored.ObjectType)
	}
	return ParseTreePayload(stored.Payload)
}

func ReadTree(hash string) (*Tree, error) {
	stored, err := ReadObject(hash)
	if err != nil {
		return nil, err
	}
	return TreeFromStored(stored)
}

func ParseTreePayload(payload []byte) (*Tree, error) {
	entries := make([]TreeEntry, 0)
	for offset := 0; offset < len(payload); {
		space := bytes.IndexByte(payload[offset:], ' ')
		if space == -1 {
			return nil, fmt.Errorf("无效 tree 对象：缺少 mode 分隔符")
		}
		mode := string(payload[offset : offset+space])
		offset += space + 1

		nul := bytes.IndexByte(payload[offset:], 0)
		if nul == -1 {
			return nil, fmt.Errorf("无效 tree 对象：缺少文件名分隔符")
		}
		name := string(payload[offset : offset+nul])
		offset += nul + 1

		if offset+20 > len(payload) {
			return nil, fmt.Errorf("无效 tree 对象：哈希长度不足")
		}
		hash := hex.EncodeToString(payload[offset : offset+20])
		offset += 20

		objectType := "blob"
		if mode == "40000" {
			objectType = "tree"
		}
		entries = append(entries, TreeEntry{Mode: mode, Name: name, Hash: hash, ObjectType: objectType})
	}
	return NewTree(entries), nil
}

func WriteTreeFromFiles(files []FileEntry) (string, *Tree, error) {
	if len(files) == 0 {
		return "", nil, fmt.Errorf("index 为空，请先运行 mgit add .")
	}
	root := &treeNode{Dirs: map[string]*treeNode{}}
	for _, file := range files {
		if err := ValidateHash(file.Hash); err != nil {
			return "", nil, fmt.Errorf("index 中的哈希无效 %s：%w", file.Path, err)
		}
		mode := file.Mode
		if mode == "" {
			mode = "100644"
		}
		parts := splitPath(file.Path)
		if len(parts) == 0 {
			continue
		}
		node := root
		for _, part := range parts[:len(parts)-1] {
			if node.Dirs == nil {
				node.Dirs = map[string]*treeNode{}
			}
			child := node.Dirs[part]
			if child == nil {
				child = &treeNode{Dirs: map[string]*treeNode{}}
				node.Dirs[part] = child
			}
			node = child
		}
		node.Files = append(node.Files, TreeEntry{Mode: mode, Name: parts[len(parts)-1], Hash: file.Hash, ObjectType: "blob"})
	}
	return writeTreeNode(root)
}

func WriteTreeFromDir(root string) (string, *Tree, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", nil, fmt.Errorf("解析目录失败：%w", err)
	}
	return writeTree(absRoot, true)
}

func writeTreeNode(node *treeNode) (string, *Tree, error) {
	entries := make([]TreeEntry, 0, len(node.Files)+len(node.Dirs))
	entries = append(entries, node.Files...)

	dirNames := make([]string, 0, len(node.Dirs))
	for name := range node.Dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	for _, name := range dirNames {
		hash, childTree, err := writeTreeNode(node.Dirs[name])
		if err != nil {
			return "", nil, err
		}
		if len(childTree.Entries) == 0 {
			continue
		}
		entries = append(entries, TreeEntry{Mode: "40000", Name: name, Hash: hash, ObjectType: "tree"})
	}

	tree := NewTree(entries)
	hash, err := tree.Write()
	if err != nil {
		return "", nil, err
	}
	return hash, tree, nil
}

func writeTree(dir string, isRoot bool) (string, *Tree, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, fmt.Errorf("读取目录失败：%w", err)
	}

	entries := make([]TreeEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if shouldSkip(name) {
			continue
		}

		fullPath := filepath.Join(dir, name)
		if item.IsDir() {
			hash, childTree, err := writeTree(fullPath, false)
			if err != nil {
				return "", nil, err
			}
			if len(childTree.Entries) == 0 {
				continue
			}
			entries = append(entries, TreeEntry{Mode: "40000", Name: name, Hash: hash, ObjectType: "tree"})
			continue
		}

		info, err := item.Info()
		if err != nil {
			return "", nil, fmt.Errorf("读取文件信息失败：%s：%w", fullPath, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return "", nil, fmt.Errorf("读取文件失败：%s：%w", fullPath, err)
		}
		blob := NewBlob(content)
		hash, err := blob.Write()
		if err != nil {
			return "", nil, err
		}
		entries = append(entries, TreeEntry{Mode: "100644", Name: name, Hash: hash, ObjectType: "blob"})
	}

	tree := NewTree(entries)
	if !isRoot && len(tree.Entries) == 0 {
		return "", tree, nil
	}
	hash, err := tree.Write()
	if err != nil {
		return "", nil, err
	}
	return hash, tree, nil
}

func splitPath(path string) []string {
	path = strings.Trim(filepath.ToSlash(filepath.Clean(path)), "/")
	if path == "" || path == "." {
		return nil
	}
	return strings.Split(path, "/")
}

func shouldSkip(name string) bool {
	switch name {
	case ".git", ".mygit", ".gocache", ".agents", ".codex":
		return true
	default:
		return false
	}
}
