package clients

import (
	"io"
	"net/http"
)

type HTTPClient interface {
	// Método para realizar una requisição HTTP Post
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}
