package system

import (
	"io"
	"os"

	"github.com/pkg/sftp"
)

type SFTPFilesystem struct {
	client *sftp.Client
}

func NewSFTPFilesystem(client *sftp.Client) *SFTPFilesystem {
	return &SFTPFilesystem{
		client: client,
	}
}

func (f *SFTPFilesystem) Stat(path string) (os.FileInfo, error) {
	return f.client.Stat(path)
}

func (f *SFTPFilesystem) ReadLink(path string) (string, error) {
	return f.client.ReadLink(path)
}

func (f *SFTPFilesystem) ReadFile(path string) ([]byte, error) {
	file, err := f.client.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

func (f *SFTPFilesystem) MkdirAll(path string, perm os.FileMode) error {
	return f.client.MkdirAll(path)
}

func (f *SFTPFilesystem) CreateFile(path string) error {
	file, err := f.client.Create(path)
	if err != nil {
		return err
	}

	_ = file.Close()
	return nil
}

func (f *SFTPFilesystem) ReadDir(path string) ([]os.FileInfo, error) {
	return f.client.ReadDir(path)
}

func (f *SFTPFilesystem) Glob(pattern string) ([]string, error) {
	return f.client.Glob(pattern)
}
