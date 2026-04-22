package transport

import (
	"fmt"

	"github.com/missdeer/notebooklm-client/internal/types"
)

const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

func ChromeHeaders(session types.NotebookRpcSession, contentLength int) map[string]string {
	ua := session.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	return map[string]string{
		"Content-Type":       "application/x-www-form-urlencoded;charset=UTF-8",
		"Content-Length":     fmt.Sprintf("%d", contentLength),
		"User-Agent":         ua,
		"Cookie":             session.Cookies,
		"Origin":             "https://notebooklm.google.com",
		"Referer":            "https://notebooklm.google.com/",
		"Accept":             "*/*",
		"Accept-Language":    "en-US,en;q=0.9",
		"Sec-Ch-Ua":         `"Chromium";v="131", "Not_A Brand";v="24"`,
		"Sec-Ch-Ua-Mobile":  "?0",
		"Sec-Ch-Ua-Platform": `"Windows"`,
		"Sec-Fetch-Dest":    "empty",
		"Sec-Fetch-Mode":    "cors",
		"Sec-Fetch-Site":    "same-origin",
		"X-Same-Domain":     "1",
	}
}
