package multipkg

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

func BasicWrong(rc ReadCloser) { // WARN rc can be pkg2.Closer
	rc.Close()
}
