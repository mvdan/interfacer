package pkg2

import (
	"io"
)

func BasicWrong(rc io.ReadCloser) {
	rc.Close()
}
