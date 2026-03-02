package system

import (
	"os"
	"path/filepath"
)

type LocalFilesystem struct{}

func (LocalFilesystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (LocalFilesystem) ReadLink(name string) (string, error) {
	return os.Readlink(name)
}

func (LocalFilesystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (LocalFilesystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (LocalFilesystem) CreateFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	_ = f.Close()
	return nil
}

func (LocalFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	entries := make([]os.FileInfo, len(dirEntries))
	for i, entry := range dirEntries {
		var info os.FileInfo
		info, err = entry.Info()
		if err != nil {
			return nil, err
		}
		entries[i] = info
	}

	return entries, nil
}

func (LocalFilesystem) RealPath(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (LocalFilesystem) Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}
