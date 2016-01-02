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

func CompareIfaceDiff(rc ReadCloser) {
	if rc != Reader(nil) {
		rc.Close()
	}
}

type mint int

func (m mint) Close() error {
	return nil
}

func CompareStruct(m mint) {
	if m != mint(3) {
		m.Close()
	}
}

func CompareStructVar(m mint) {
	m2 := mint(2)
	if m == m2 {
		m.Close()
	}
}

func CompareLit(m mint) {
	if m != 3 {
		m.Close()
	}
}
