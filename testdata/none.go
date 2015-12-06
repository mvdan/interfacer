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
