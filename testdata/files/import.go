package foo

import (
	"io"
	"os"
)

func Empty() {
}

func Basic(c io.Closer) {
	c.Close()
}

func BasicWrong(rc io.ReadCloser) {
	rc.Close()
}

type st struct{}

func (s *st) Basic(c io.Closer) {
	c.Close()
}

func (s *st) BasicWrong(rc io.ReadCloser) {
	rc.Close()
}

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
