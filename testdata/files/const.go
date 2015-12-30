package foo

import (
	"io"
)

const offset = 1

func Const(s io.Seeker) {
	var whence int = 0
	s.Seek(offset, whence)
}

func ConstWrong(rs io.ReadSeeker) {
	var whence int = 0
	rs.Seek(offset, whence)
}

func LocalConst(s io.Seeker) {
	const offset2 = 2
	var whence int = 0
	s.Seek(offset2, whence)
}

func LocalConstWrong(rs io.ReadSeeker) {
	const offset2 = 2
	var whence int = 0
	rs.Seek(offset2, whence)
}

func AssignFromConst() {
	var i int
	i = offset
	println(i)
}
