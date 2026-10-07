package server

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// Network reads and writes happen outside the handler's room/state locks.
// WebSocket upgrades must bypass this wrapper (they require Hijacker).
func bufferedHTTP(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(8 * time.Second))
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			_ = r.Body.Close()
			if err != nil {
				writeError(w, http.StatusBadRequest, "请求过大或读取超时")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		response := &httpBuffer{header: make(http.Header)}
		handler(response, r)
		for key, values := range response.header {
			w.Header()[key] = values
		}
		code := response.status
		if code == 0 {
			code = http.StatusOK
		}
		w.WriteHeader(code)
		_, _ = w.Write(response.body.Bytes())
	}
}

type httpBuffer struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *httpBuffer) Header() http.Header { return w.header }
func (w *httpBuffer) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}
func (w *httpBuffer) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}
