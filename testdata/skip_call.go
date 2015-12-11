package foo

import (
	"os"
)

func Foo(se os.SyscallError) {
	println(se.Err.Error())
}
