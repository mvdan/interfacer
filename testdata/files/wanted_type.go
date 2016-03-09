package foo

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

type Foo struct{}

func (f Foo) Close() {}

func DoClose(f Foo) {
	f.Close()
}

func DoCloseFoo(f Foo) {
	f.Close()
}

type bar struct{}

func (f bar) Close() {}

func doCloseBar(b bar) {
	b.Close()
}

func barwrongClose(b bar) {
	b.Close()
}

func doCloseBarwrong(b bar) {
	b.Close()
}
