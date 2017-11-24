package main

import (
	"io"
	"net/http"
	"strconv"
)

// httpWebGetter downloads file via HTTP
type httpWebGetter struct {
}

// Get returns data body and content length by URL
func (g httpWebGetter) Get(url string) (body io.ReadCloser, contentLen int, err error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, 0, err
	}

	contentLen, _ = strconv.Atoi(response.Header.Get("Content-Length"))

	return response.Body, contentLen, nil
}
