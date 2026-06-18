package normalize

import (
	"net/url"
	"sort"
	"strings"
)

// Normalize standardizes a URL for consistent comparison.
// Rules:
// - Scheme to lowercase
// - Host to lowercase
// - Remove default ports (80 for http, 443 for https)
// - Remove fragment (#...)
// - Sort query parameters alphabetically by key
// - Remove trailing slash from path (unless path is "/")
// - Remove "www." prefix from host
// - Return empty string for non-http/https schemes
// - Reject empty strings
func Normalize(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" || u.Host == "" {
		return "", nil
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", nil
	}

	host := strings.ToLower(u.Host)

	// Remove default ports
	if scheme == "http" && strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	}
	if scheme == "https" && strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}

	// Remove "www." prefix
	host = strings.TrimPrefix(host, "www.")

	// Path: remove trailing slash unless it's just "/"
	path := u.Path
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	if path == "" {
		path = "/"
	}

	// Sort query parameters
	q := u.Query()
	if len(q) > 0 {
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		values := url.Values{}
		for _, k := range keys {
			vals := q[k]
			sort.Strings(vals)
			for _, v := range vals {
				values.Add(k, v)
			}
		}
		u.RawQuery = values.Encode()
	} else {
		u.RawQuery = ""
	}

	// Fragment is dropped
	u.Fragment = ""

	// Reconstruct
	u.Scheme = scheme
	u.Host = host
	u.Path = path

	return u.String(), nil
}

// SameHost returns true if u1 and u2 have the same normalized host.
func SameHost(u1, u2 *url.URL) bool {
	if u1 == nil || u2 == nil {
		return false
	}
	h1 := strings.ToLower(u1.Host)
	h2 := strings.ToLower(u2.Host)
	h1 = strings.TrimPrefix(h1, "www.")
	h2 = strings.TrimPrefix(h2, "www.")
	return h1 == h2
}
