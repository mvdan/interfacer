package foo

import (
	"io"
)

func Foo(r io.ReadCloser) {
	r.Close()
}
