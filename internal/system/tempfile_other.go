//go:build !linux

package system

import (
	"fmt"
	"os"
)

type TempFile struct {
	path string
}

func NewTempFile(pattern string, content []byte) (*TempFile, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create tempfile: %v", err)
	}

	defer func() {
		if err != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
	}()

	if _, err = file.Write(content); err != nil {
		return nil, fmt.Errorf("failed to write to tempfile: %v", err)
	}

	if err = file.Chmod(0o400); err != nil {
		return nil, fmt.Errorf("failed to change permissions of tempfile: %v", err)
	}

	path := file.Name()

	if err = file.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tempfile: %v", err)
	}

	t := TempFile{
		path: path,
	}

	return &t, nil
}

func (t *TempFile) Path() string {
	return t.path
}

func (t *TempFile) Remove() error {
	return os.Remove(t.path)
}
