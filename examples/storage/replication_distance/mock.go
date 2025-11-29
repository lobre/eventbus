package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/lobre/eventbus"
)

type httpMock struct {
	mu   sync.Mutex
	data []byte
}

func newHTTPMock(seed *eventbus.Bus) *httpMock {
	var buf bytes.Buffer
	if err := seed.Dump(&buf); err != nil {
		log.Fatalf("seed dump: %v", err)
	}
	return &httpMock{data: buf.Bytes()}
}

func (m *httpMock) RoundTrip(req *http.Request) (*http.Response, error) {
	switch req.Method {
	case http.MethodGet:
		return m.handleGet()
	case http.MethodPatch:
		return m.handlePatch(req)
	default:
		return &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}
}

func (m *httpMock) handleGet() (*http.Response, error) {
	m.mu.Lock()
	data := append([]byte(nil), m.data...)
	m.mu.Unlock()

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (m *httpMock) handlePatch(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(bytes.NewReader([]byte("read body"))),
		}, nil
	}

	m.mu.Lock()
	m.data = append([]byte(nil), body...)
	m.mu.Unlock()

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}, nil
}

func (m *httpMock) snapshot() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.data...)
}
