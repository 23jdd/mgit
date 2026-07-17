package repo

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	PrimaryDir  = ".git"
	FallbackDir = ".mygit"
	MarkerFile  = "MGIT"
	envDir      = "MGIT_DIR"
)

// Dir returns the mgit metadata directory for the current workspace.
// In a normal empty/project directory mgit uses .git. When the directory is
// already a real Git repository, mgit falls back to .mygit to avoid corrupting
// Git's own index, objects and refs.
func Dir() string {
	if value := strings.TrimSpace(os.Getenv(envDir)); value != "" {
		return filepath.Clean(value)
	}
	if exists(FallbackDir) {
		return FallbackDir
	}
	if isMgitDir(PrimaryDir) {
		return PrimaryDir
	}
	if isRealGitDir(PrimaryDir) {
		return FallbackDir
	}
	return PrimaryDir
}

func Path(parts ...string) string {
	items := append([]string{Dir()}, parts...)
	return filepath.Join(items...)
}

func Mark() error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(MarkerFile), []byte("mgit repository\n"), 0o644)
}

func IsInternalName(name string) bool {
	switch name {
	case PrimaryDir, FallbackDir, ".gocache", ".agents", ".codex":
		return true
	default:
		return false
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isMgitDir(path string) bool {
	return exists(filepath.Join(path, MarkerFile))
}

func isRealGitDir(path string) bool {
	if isMgitDir(path) {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	if exists(filepath.Join(path, "config")) {
		return true
	}
	if exists(filepath.Join(path, "commondir")) || exists(filepath.Join(path, "packed-refs")) {
		return true
	}
	return false
}
