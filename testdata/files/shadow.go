package foo

import (
	"io"
)

type FooCloser interface {
	Foo()
	Close() error
}

func ShadowArg(fc FooCloser) {
	fc.Close()
	for {
		fc := 3
		println(fc + 1)
	}
}
