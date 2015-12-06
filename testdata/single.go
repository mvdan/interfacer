package foo

import (
	"io"
)

func Basic(rc io.ReadCloser) {
	rc.Close()
}

func Args(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
}

func ArgsMake(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
}

func ArgsLit(rs io.ReadSeeker) {
	rs.Seek(20, 0)
}

type st struct{}

func (s *st) Basic(rc io.ReadCloser) {
	rc.Close()
}

func (s st) Args(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
}
