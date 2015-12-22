package foo

import (
	"io"
)

func Array(ints [3]int) {}

func ArrayIface(rcs [3]io.ReadCloser) {
	rcs[1].Close()
}

func Slice(ints []int) {}

func SliceIface(rcs []io.ReadCloser) {
	rcs[1].Close()
}
