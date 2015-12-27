package foo

import (
	"io"
)

func FooGo(rc io.ReadCloser) {
	rc.Read(nil)
	go func() {
		rc.Close()
	}()
}

func FooArg(rc io.ReadCloser) {
	rc.Read(nil)
	f := func(err error) {}
	f(rc.Close())
}

func FooGoWrong(rc io.ReadCloser) {
	go func() {
		rc.Close()
	}()
}

func FooArgWrong(rc io.ReadCloser) {
	f := func(err error) {}
	f(rc.Close())
}

func FooNestedWrong(rc io.ReadCloser) {
	f := func(rc io.ReadCloser) {
		rc.Close()
	}
	f(nil)
	b := make([]byte, 10)
	rc.Read(b)
}
