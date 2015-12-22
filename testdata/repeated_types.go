package foo

import (
	"os"
)

type Namer interface {
	Name() string
}

type MyWalkFunc func(path string, info os.FileInfo, err error) error

func Impl(path string, info os.FileInfo, err error) error {
	info.Name()
	return nil
}
