package system

import "os"

type Filesystem interface {
	Stat(path string) (os.FileInfo, error)
	ReadLink(path string) (string, error)
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
}
