// Package reqorigin provides functions for extracting 'X-Origin' header from
// http request.
package reqorigin

import (
	"net/http"
)

const OriginKey = "X-Origin"

// FromRequest extracts the value of the 'X-Origin' header.
func FromRequest(req *http.Request) string {
	return req.Header.Get(OriginKey)
}

// SetHeader sets X-Origin: origin to the request headers.
//
// If origin is empty string, it does nothing.
func SetHeader(req *http.Request, origin string) {
	if origin != "" {
		req.Header.Add(OriginKey, origin)
	}
}
