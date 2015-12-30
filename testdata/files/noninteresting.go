package foo

import (
	"os"
)

type EmptyIface interface{}

type UninterestingMethods interface {
	Foo() error
	bar() int
}

type InterestingUnexported interface {
	Foo(f *os.File) error
	bar(f *os.File) int
}

type st struct{}

func (s st) Foo(f *os.File) {}

func Bar(s st) {
	s.Foo(nil)
}

type NonInterestingFunc func() error

func NonInterestingCall() {
	os.Exit(3)
}
