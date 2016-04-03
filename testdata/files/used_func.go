package foo

type Closer interface {
	Close()
}

type ReadCloser interface {
	Read(p []byte) (n int, err error)
	Closer
}

func Wrong(rc ReadCloser) { // WARN rc can be Closer
	rc.Close()
}

func receiver(f func(ReadCloser)) {
	var rc ReadCloser
	f(rc)
}

func Correct(rc ReadCloser) {
	rc.Close()
}

func CorrectUse() {
	receiver(Correct)
}

func WrongLit() {
	f := func(rc ReadCloser) { // WARN rc can be Closer
		rc.Close()
	}
	f(nil)
}

func CorrectLit() {
	f := func(rc ReadCloser) {
		rc.Close()
	}
	receiver(f)
}
