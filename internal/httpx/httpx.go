// Package httpx provides the small HTTP helper used to fetch receipt HTML.
// Kept separate from the parsers so the transport concern is obvious and the
// User-Agent (which has caused 403s in the past) lives in one well-known place.
package httpx

import (
	"fmt"
	"io"
	"net/http"
)

// Hardcoded UA: some receipt hosts (Silpo, historically) block requests with
// the Go default UA. Rotate this if a host starts returning 403.
const userAgent = `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.27 Safari/537.36`

// GetHTML issues a GET, requires a 200, and returns the response body as a string.
func GetHTML(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %v", err)
	}
	return string(body), nil
}
