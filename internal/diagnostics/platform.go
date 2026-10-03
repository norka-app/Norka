package diagnostics

import "strings"

// OSVersion is a short operating-system description, or empty when it cannot be read.
func OSVersion() string {
	return strings.TrimSpace(osVersion())
}

// WebViewVersion is the WebView or WebKit version when it can be read.
func WebViewVersion() string {
	return strings.TrimSpace(webViewVersion())
}
