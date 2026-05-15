package pkg

import "net/url"

// parseQueryBytes turns the raw form-encoded body Slack POSTed into a
// Values map without disturbing the bytes the HMAC was computed over.
// Equivalent to url.ParseQuery(string(b)) but kept as its own helper
// so the call site reads intent-first.
func parseQueryBytes(b []byte) (url.Values, error) {
	return url.ParseQuery(string(b))
}
