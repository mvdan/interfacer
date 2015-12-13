package foo

import (
	"os"
)

type Barer interface {
	Bar(f *os.File) int
}

type st struct {}

func (s *st) Bar(f *os.File) int {
	_ = f.Fd()
	return 2
}

func Foo(s *st) {
	_ = s.Bar(nil)
}

func Bar(f *os.File) int {
	f.Close()
	return 3
}
