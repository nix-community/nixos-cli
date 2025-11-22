package system

import (
	"os"
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
