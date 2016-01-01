package foo

type Reader interface {
	Read([]byte) (int, error)
}

type Closer interface {
	Close() error
}

type ReadCloser interface {
	Reader
	Closer
}

func CompareNil(rc ReadCloser) {
	if rc != nil {
		rc.Close()
	}
}

func CompareIface(rc ReadCloser) {
	if rc != ReadCloser(nil) {
		rc.Close()
	}
}

type st int

func (s st) Close() error {
	return nil
}

func CompareStruct(s st) {
	if s != st(3) {
		s.Close()
	}
}
