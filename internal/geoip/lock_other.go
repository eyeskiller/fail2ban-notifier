//go:build !unix

package geoip

import (
	"os"
)

type fileLock struct {
	file *os.File
}

func acquireFileLock(path string) (*fileLock, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return &fileLock{file: file}, nil
}

func (l *fileLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}

func openNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
