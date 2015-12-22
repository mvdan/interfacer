package foo

import (
	"io"

	"grab-import/def"
)

type st struct{}

func (s *st) Foo(rc io.ReadCloser, i int) int {
	rc.Close()
	return def.SomeVar
}
