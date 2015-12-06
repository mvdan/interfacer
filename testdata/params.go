package foo

import (
	"io"
)

func Args(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
	rc.Close()
}

func ArgsWrong(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
}

func ArgsLit(rs io.ReadSeeker) {
	b := make([]byte, 10)
	rs.Read(b)
	rs.Seek(20, 0)
}

func ArgsLitWrong(rs io.ReadSeeker) {
	rs.Seek(20, 0)
}

func ArgsNil(rs io.ReadSeeker) {
	rs.Read(nil)
	rs.Seek(20, 0)
}

func ArgsNilWrong(rs io.ReadSeeker) {
	rs.Read(nil)
}

type st struct{}

func (s st) Args(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
	rc.Close()
}

func (s st) ArgsWrong(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
}

type argBad struct{}

func (a argBad) Read(n int) (int, error) {
	return 0, nil
}

type argGood struct{}

func (a argGood) Read(p []byte) (int, error) {
	return 0, nil
}

func ArgsMismatch(a argBad) {
	a.Read(10)
}

func ArgsMatch(a argGood) {
	b := make([]byte, 10)
	a.Read(b)
}
