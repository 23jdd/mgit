package main

import (
	"flag"
	"fmt"
	idx "github.com/23jdd/mgit/index"
	"github.com/23jdd/mgit/object"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type diffEntry struct {
	Mode    string
	Hash    string
	Content []byte
	Ok      bool
}

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	staged := fs.Bool("staged", false, "比较 HEAD/指定 commit 与 index")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "用法：mgit diff [--staged] [commit|分支|标签]")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return fmt.Errorf("diff 最多接收一个 commit、分支或标签")
	}

	if *staged {
		base := "HEAD"
		if fs.NArg() == 1 {
			base = fs.Arg(0)
		}
		baseHash, err := resolveCommitish(base)
		if err != nil {
			return err
		}
		baseMap, err := commitDiffMap(baseHash)
		if err != nil {
			return err
		}
		indexMap, err := indexDiffMap()
		if err != nil {
			return err
		}
		return printDiffMaps(baseMap, indexMap, "a", "b")
	}

	indexMap, err := indexDiffMap()
	if err != nil {
		return err
	}
	worktreeMap, err := worktreeDiffMap(indexMap)
	if err != nil {
		return err
	}
	if fs.NArg() == 1 {
		baseHash, err := resolveCommitish(fs.Arg(0))
		if err != nil {
			return err
		}
		baseMap, err := commitDiffMap(baseHash)
		if err != nil {
			return err
		}
		return printDiffMaps(baseMap, worktreeMap, "a", "b")
	}
	return printDiffMaps(indexMap, worktreeMap, "a", "b")
}

func commitDiffMap(commitHash string) (map[string]diffEntry, error) {
	files, err := commitFiles(commitHash)
	if err != nil {
		return nil, err
	}
	result := make(map[string]diffEntry, len(files))
	for _, file := range files {
		blob, err := object.ReadBlob(file.Hash)
		if err != nil {
			return nil, err
		}
		result[file.Path] = diffEntry{Mode: file.Mode, Hash: file.Hash, Content: blob.Content, Ok: true}
	}
	return result, nil
}

func indexDiffMap() (map[string]diffEntry, error) {
	file, err := idx.Load(idx.DefaultPath)
	if err != nil {
		return nil, err
	}
	result := make(map[string]diffEntry, len(file.Entries))
	for _, entry := range file.Entries {
		blob, err := object.ReadBlob(entry.Hash)
		if err != nil {
			return nil, err
		}
		result[entry.Path] = diffEntry{Mode: entry.Mode, Hash: entry.Hash, Content: blob.Content, Ok: true}
	}
	return result, nil
}

func worktreeDiffMap(indexMap map[string]diffEntry) (map[string]diffEntry, error) {
	result := make(map[string]diffEntry, len(indexMap))
	for path, entry := range indexMap {
		content, err := os.ReadFile(filepath.FromSlash(path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("读取工作区文件失败 %s：%w", path, err)
		}
		blob := object.NewBlob(content)
		result[path] = diffEntry{Mode: entry.Mode, Hash: blob.HashString(), Content: content, Ok: true}
	}
	return result, nil
}

func printDiffMaps(oldMap map[string]diffEntry, newMap map[string]diffEntry, oldPrefix string, newPrefix string) error {
	paths := make([]string, 0, len(oldMap)+len(newMap))
	seen := map[string]bool{}
	for path := range oldMap {
		seen[path] = true
	}
	for path := range newMap {
		seen[path] = true
	}
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		oldEntry := oldMap[path]
		newEntry := newMap[path]
		if oldEntry.Ok && newEntry.Ok && oldEntry.Hash == newEntry.Hash {
			continue
		}
		printFileDiff(path, oldEntry, newEntry, oldPrefix, newPrefix)
	}
	return nil
}

func printFileDiff(path string, oldEntry diffEntry, newEntry diffEntry, oldPrefix string, newPrefix string) {
	fmt.Printf("diff --mgit %s/%s %s/%s\n", oldPrefix, path, newPrefix, path)
	if !oldEntry.Ok {
		fmt.Printf("new file mode %s\n", firstNonEmpty(newEntry.Mode, "100644"))
	} else if !newEntry.Ok {
		fmt.Printf("deleted file mode %s\n", firstNonEmpty(oldEntry.Mode, "100644"))
	}
	if oldEntry.Ok {
		fmt.Printf("--- %s/%s\n", oldPrefix, path)
	} else {
		fmt.Println("--- /dev/null")
	}
	if newEntry.Ok {
		fmt.Printf("+++ %s/%s\n", newPrefix, path)
	} else {
		fmt.Println("+++ /dev/null")
	}
	oldLines := splitDiffLines(oldEntry.Content)
	newLines := splitDiffLines(newEntry.Content)
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	for i := 0; i < max; i++ {
		var oldLine, newLine string
		oldOK := i < len(oldLines)
		newOK := i < len(newLines)
		if oldOK {
			oldLine = oldLines[i]
		}
		if newOK {
			newLine = newLines[i]
		}
		switch {
		case oldOK && newOK && oldLine == newLine:
			fmt.Printf(" %s\n", oldLine)
		case oldOK && newOK:
			fmt.Printf("-%s\n", oldLine)
			fmt.Printf("+%s\n", newLine)
		case oldOK:
			fmt.Printf("-%s\n", oldLine)
		case newOK:
			fmt.Printf("+%s\n", newLine)
		}
	}
}

func splitDiffLines(content []byte) []string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
