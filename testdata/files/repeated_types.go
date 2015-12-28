package foo

import (
	"os"
)

type Namer interface {
	Name() string
}

type MyWalkFunc func(path string, info os.FileInfo, err error) error
type MyWalkFunc2 func(path string, info os.FileInfo, err error) error

func Impl(path string, info os.FileInfo, err error) error {
	info.Name()
	return nil
}

type MyIface interface {
	FooBar()
}
type MyIface2 interface {
	MyIface
}

type st struct{}

func (s st) FooBar() {}

func FooWrong(s st) {
	s.FooBar()
}
