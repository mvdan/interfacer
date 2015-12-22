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

type argBad struct{}

func (a argBad) Read(p []byte) (string, error) {
	return "", nil
}

func (a argBad) Write(p []byte) error {
	return nil
}

func (a argBad) Close() int {
	return 0
}

func ResultsMismatchNumber(a argBad) {
	var b []byte
	_ = a.Write(b)
}

func ResultsMismatchType(a argBad) {
	b := make([]byte, 10)
	s, _ := a.Read(b)
	println(s)
}

func ResultsMismatchTypes(a, b argBad) {
	r1, r2 := a.Close(), b.Close()
	println(r1, r2)
}

func ResultsMismatchDiscarded(a, b argBad) {
	a.Close()
	_ = b.Close()
}
