package handler

import "strings"

// parseUserAgent extracts a short browser and OS name from a User-Agent
// string for the web client's session listing (docs/AUTH.md, п.1).
func parseUserAgent(ua string) (browser, osName string) {
	switch {
	case strings.Contains(ua, "Windows"):
		osName = "Windows"
	case strings.Contains(ua, "Android"):
		osName = "Android"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iOS"):
		osName = "iOS"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		osName = "macOS"
	case strings.Contains(ua, "Linux"):
		osName = "Linux"
	}

	switch {
	case strings.Contains(ua, "Edg/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/"):
		browser = "Opera"
	case strings.Contains(ua, "Chrome/"):
		browser = "Chrome"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	return browser, osName
}
