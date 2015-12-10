package pkg1

import (
	"io"
)

func BasicWrong(rc io.ReadCloser) {
	rc.Close()
}
