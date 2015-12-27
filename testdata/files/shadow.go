package foo

import (
	"io"
)

func ShadowArg(rc io.ReadCloser) {
	rc.Close()
	for {
		rc := 3
		println(rc + 1)
	}
}
