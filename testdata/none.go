package foo

import (
	"io"
)

func Foo(c io.Closer) {
	c.Close()
}
