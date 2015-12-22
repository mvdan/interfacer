package foo

import (
	"io"
)

func If(s io.Seeker) error {
	for i := int64(0); i < 10; i++ {
		if _, err := s.Seek(i, 0); err != nil {
			return err
		}
	}
	return nil
}

func IfWrong(rc io.ReadCloser) error {
	if err := rc.Close(); err != nil {
		return err
	}
	return nil
}
