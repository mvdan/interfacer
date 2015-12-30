package foo

import (
	"os"
)

type MyFunc func(rc *os.File, err error) bool

func MyFuncImpl(f *os.File, err error) bool {
	f.Close()
	return false
}

func MyFuncWrong(f *os.File, err error) {
	f.Close()
}
