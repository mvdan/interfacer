package foo

import (
	"io"
)

func Foo(c io.Closer) {
	c.Close()
}

func FooArgs(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
	rc.Close()
}

func FooArgsMake(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
	rc.Close()
}

func FooArgsLit(rs io.ReadSeeker) {
	b := make([]byte, 10)
	rs.Read(b)
	rs.Seek(20, 0)
}

type st struct{}

func (s *st) Foo(c io.Closer) {
	c.Close()
}

func (s st) FooArgs(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
	rc.Close()
}
