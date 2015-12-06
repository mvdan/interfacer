package foo

import (
	"io"
)

func Foo(rc io.ReadCloser) {
	rc.Close()
}

func FooArgs(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
}

func FooArgsMake(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
}

func FooArgsLit(rs io.ReadSeeker) {
	rs.Seek(20, 0)
}

type st struct{}

func (s *st) Foo(rc io.ReadCloser) {
	rc.Close()
}

func (s st) FooArgs(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
}
