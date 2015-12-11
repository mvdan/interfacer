package foo

import (
	"io"
	"os"
)

func FooCloser(c io.Closer) {
	c.Close()
}

func FooFile(f *os.File) {
	f.Stat()
}

func Bar(f *os.File) {
	f.Close()
	FooFile(f)
}

func BarWrong(f *os.File) {
	f.Close()
	FooCloser(f)
}
