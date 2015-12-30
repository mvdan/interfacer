package foo

type Closer interface {
	Close()
}

type ReadCloser interface {
	Closer
	Read()
}

func BasicWrong(rc ReadCloser) {
	rc.Close()
}

func Array(ints [3]int) {}

func ArrayIface(rcs [3]ReadCloser) {
	rcs[1].Close()
}

func Slice(ints []int) {}

func SliceIface(rcs []ReadCloser) {
	rcs[1].Close()
}
