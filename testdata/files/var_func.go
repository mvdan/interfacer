package foo

import (
	"io"
	"os"
)

var Basic = func(c io.Closer) {
	c.Close()
}

var BasicWrong = func(rc io.ReadCloser) {
	rc.Close()
}

type MyFunc func(rc *os.File, err error) bool

var MyFuncImpl = func(f *os.File, err error) bool {
	f.Close()
	return false
}

var MyFuncWrong = func(f *os.File, err error) {
	f.Close()
}
