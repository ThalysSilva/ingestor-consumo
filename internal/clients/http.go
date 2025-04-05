package clients

import (
	"io"
	"net/http"
)

type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}
