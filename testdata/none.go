package foo

import (
	"io"
)

func Empty() {
}

func Basic(c io.Closer) {
	c.Close()
}

func Args(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
	rc.Close()
}

func ArgsMake(rc io.ReadCloser) {
	b := make([]byte, 10)
	rc.Read(b)
	rc.Close()
}

func ArgsLit(rs io.ReadSeeker) {
	b := make([]byte, 10)
	rs.Read(b)
	rs.Seek(20, 0)
}

func ArgsNil(rs io.ReadSeeker) {
	rs.Read(nil)
	rs.Seek(20, 0)
}

type st struct{}

func (s *st) Basic(c io.Closer) {
	c.Close()
}

func (s st) Args(rc io.ReadCloser) {
	var b []byte
	rc.Read(b)
	rc.Close()
}
