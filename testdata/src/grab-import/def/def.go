package def

import (
	"io"
)

type FooFunc func(io.ReadCloser, int) int

var SomeVar int = 3
