package config

import (
	"os"
	"path/filepath"
	"strings"
)

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func DefaultStoreDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.xiaohongshu-cli"
	}
	return filepath.Join(home, ".xiaohongshu-cli")
}

func EnsureStoreDir(dir string) error {
	dir = ExpandPath(dir)
	return os.MkdirAll(dir, 0700)
}
