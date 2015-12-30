package foo

import (
	"grab-import/def"
)

type st struct{}

func (s *st) Foo(rc def.ReadCloser, i int) int {
	rc.Close()
	return def.SomeVar
}

func NonInterestingCall() {
	def.SomeFunc()
}
