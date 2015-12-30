package foo

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

func ForIf(rc ReadCloser) {
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			rc.Close()
		}
	}
	rc.Read()
}

func IfWrong(rc ReadCloser) {
	if 3 > 2 {
		rc.Close()
	}
}
