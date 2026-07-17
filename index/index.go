package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/23jdd/mgit/ignore"
	"github.com/23jdd/mgit/object"
)

const DefaultPath = ".mygit/index"

type Entry struct {
	Mode string `json:"mode"`
	Hash string `json:"hash"`
	Path string `json:"path"`
}

type File struct {
	Entries []Entry `json:"entries"`
}

func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &File{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取 index 失败：%w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return &File{}, nil
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("解析 index 失败：%w", err)
	}
	file.Sort()
	return &file, nil
}

func Save(path string, file *File) error {
	file.Sort()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建 index 目录失败：%w", err)
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("编码 index 失败：%w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("写入 index 失败：%w", err)
	}
	return nil
}

func AddPaths(root string, paths []string) (*File, []Entry, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("解析工作区失败：%w", err)
	}
	matcher, err := ignore.Load(rootAbs)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 .gitignore 失败：%w", err)
	}

	file, err := Load(DefaultPath)
	if err != nil {
		return nil, nil, err
	}

	added := make([]Entry, 0)
	for _, path := range paths {
		entries, rel, err := collectPath(rootAbs, path, matcher)
		if err != nil {
			return nil, nil, err
		}
		if rel == "" && len(entries) == 0 {
			continue
		}
		file.removePath(rel)
		added = append(added, entries...)
	}

	file.merge(added)
	if err := Save(DefaultPath, file); err != nil {
		return nil, nil, err
	}
	return file, added, nil
}

func (f *File) ToObjectFiles() []object.FileEntry {
	files := make([]object.FileEntry, 0, len(f.Entries))
	for _, entry := range f.Entries {
		files = append(files, object.FileEntry{Mode: entry.Mode, Path: entry.Path, Hash: entry.Hash})
	}
	return files
}

func (f *File) Sort() {
	sort.Slice(f.Entries, func(i, j int) bool {
		return f.Entries[i].Path < f.Entries[j].Path
	})
}

func (f *File) removePath(rel string) {
	rel = normalizePath(rel)
	if rel == "." || rel == "" {
		f.Entries = nil
		return
	}
	prefix := rel + "/"
	kept := f.Entries[:0]
	for _, entry := range f.Entries {
		if entry.Path == rel || strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		kept = append(kept, entry)
	}
	f.Entries = kept
}

func (f *File) merge(entries []Entry) {
	byPath := make(map[string]Entry, len(f.Entries)+len(entries))
	for _, entry := range f.Entries {
		byPath[entry.Path] = entry
	}
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}
	f.Entries = f.Entries[:0]
	for _, entry := range byPath {
		f.Entries = append(f.Entries, entry)
	}
	f.Sort()
}

func collectPath(rootAbs string, path string, matcher *ignore.Matcher) ([]Entry, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("解析路径失败 %s：%w", path, err)
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return nil, "", fmt.Errorf("计算相对路径失败：%w", err)
	}
	rel = normalizePath(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, "", fmt.Errorf("路径不在工作区内：%s", path)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, "", fmt.Errorf("读取路径失败 %s：%w", path, err)
	}
	if matcher.Ignored(abs, info.IsDir()) {
		return nil, "", nil
	}
	if info.IsDir() {
		entries, err := collectDir(rootAbs, abs, matcher)
		return entries, rel, err
	}
	entry, err := collectFile(rootAbs, abs)
	if err != nil {
		return nil, "", err
	}
	return []Entry{entry}, rel, nil
}

func collectDir(rootAbs string, dir string, matcher *ignore.Matcher) ([]Entry, error) {
	entries := make([]Entry, 0)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if matcher.Ignored(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		entry, err := collectFile(rootAbs, path)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描目录失败：%w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func collectFile(rootAbs string, path string) (Entry, error) {
	rel, err := filepath.Rel(rootAbs, path)
	if err != nil {
		return Entry{}, fmt.Errorf("计算文件相对路径失败：%w", err)
	}
	rel = normalizePath(rel)
	content, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, fmt.Errorf("读取文件失败 %s：%w", path, err)
	}
	blob := object.NewBlob(content)
	hash, err := blob.Write()
	if err != nil {
		return Entry{}, err
	}
	return Entry{Mode: "100644", Hash: hash, Path: rel}, nil
}

func normalizePath(path string) string {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == "./" || path == "." {
		return "."
	}
	return strings.TrimPrefix(path, "./")
}
