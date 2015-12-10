package foo

import (
	"os"
)

func Foo(f *os.File) {
	f.Stat()
}

func Bar(f *os.File) {
	f.Close()
	Foo(f)
}
