package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

func normalizeWorktreePath(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(filepath.Clean(path))
	path = strings.TrimPrefix(path, "./")
	if path == "." {
		return "."
	}
	return strings.Trim(path, "/")
}

func normalizeWorktreePaths(command string, paths []string) ([]string, error) {
	pathspecs := make([]string, 0, len(paths))
	for _, path := range paths {
		pathspec := normalizeWorktreePath(path)
		if pathspec == "" {
			return nil, fmt.Errorf("%s 路径不能为空", command)
		}
		pathspecs = append(pathspecs, pathspec)
	}
	return pathspecs, nil
}

func restorePathMatches(filePath string, pathspec string) bool {
	filePath = normalizeWorktreePath(filePath)
	pathspec = normalizeWorktreePath(pathspec)
	if pathspec == "." {
		return true
	}
	return filePath == pathspec || strings.HasPrefix(filePath, pathspec+"/")
}
