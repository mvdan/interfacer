package foo

import (
	"io"
)

func Results(rc io.ReadCloser) {
	b := make([]byte, 10)
	_, _ = rc.Read(b)
	err := rc.Close()
	println(err)
}

func ResultsWrong(rc io.ReadCloser) {
	err := rc.Close()
	println(err)
}
