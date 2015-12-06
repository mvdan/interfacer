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
