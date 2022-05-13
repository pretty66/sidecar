package bodybuffer

import (
	"bytes"
	"net/http"
)

type BodyWriter struct {
	http.ResponseWriter
	Body *bytes.Buffer
	// Response mirror replication size limit
	cloneLimit int
}

func NewBodWriter(cloneLimit int) *BodyWriter {
	return &BodyWriter{
		ResponseWriter: nil,
		Body:           bytes.NewBuffer(nil),
		cloneLimit:     cloneLimit,
	}
}

func (w *BodyWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}

	l := w.Body.Len()
	if l <= w.cloneLimit {
		if len(b) > w.cloneLimit-l {
			w.Body.Write(b[0 : w.cloneLimit-l])
		} else {
			w.Body.Write(b)
		}
	}

	return w.ResponseWriter.Write(b)
}

func (w *BodyWriter) Reset() {
	w.Body.Reset()
}
