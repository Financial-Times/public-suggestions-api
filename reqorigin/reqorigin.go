// Package reqorigin provides functions for extracting 'X-Origin' header from
// http request.
package reqorigin

import "net/http"

const OriginKey = "X-Origin"

// FromRequest extracts the value of the 'X-Origin' header.
func FromRequest(req *http.Request) string {
	return req.Header.Get(OriginKey)
}
