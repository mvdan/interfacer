package foo

import (
	"io"
)

var Basic = func(c io.Closer) {
	c.Close()
}

var BasicWrong = func(rc io.ReadCloser) {
	rc.Close()
}

type st struct {}

func (s st) Close() error {
	return nil
}

type MyFunc func(s st, err error) bool

var MyFuncImpl = func(s st, err error) bool {
	s.Close()
	return false
}

var MyFuncWrong = func(s st, err error) {
	s.Close()
}
