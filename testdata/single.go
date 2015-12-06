package foo

import (
	"io"
)

func Foo(r io.ReadCloser) {
	r.Close()
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
