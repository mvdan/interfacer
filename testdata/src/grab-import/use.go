package foo

import (
	"grab-import/def"
	"grab-import/def/nested"
)

type st struct{}

func (s *st) Foo(rc def.ReadCloser, i int) int {
	rc.Close()
	return def.SomeVar
}

func NonInterestingCall() {
	def.SomeFunc()
}

func Foo(f nested.Fooer) {
	f.Foo()
}

type st2 struct{}

func (s st2) Foo()

func FooWrong(s st2) {
	s.Foo()
}
