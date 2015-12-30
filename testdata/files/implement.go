package foo

import (
	"os"
)

type MyWalkFunc func(string, os.FileInfo, error) error

type Namer interface {
	Name() string
}

func WalkFuncImpl(path string, info os.FileInfo, err error) error {
	info.Name()
	return nil
}

func WalkFuncImplWrong(path string, info os.FileInfo, err error) {
	info.Name()
}

type MyFunc func(rc *os.File, err error) bool

func MyFuncImpl(f *os.File, err error) bool {
	f.Close()
	return false
}

func MyFuncWrong(f *os.File, err error) {
	f.Close()
}
