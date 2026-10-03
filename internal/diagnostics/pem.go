package diagnostics

import "regexp"

// privateKeyPattern removes PEM private-key blocks from text we keep.
type privateKeyPattern struct{}

var privateKeyRE = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)

func (privateKeyPattern) ReplaceAllString(text, repl string) string {
	return privateKeyRE.ReplaceAllString(text, repl)
}
