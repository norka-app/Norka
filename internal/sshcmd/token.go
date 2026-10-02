package sshcmd

import "strings"

// quoteStyle picks how quotes and escapes are read.
// POSIX is the default. Windows cmd is used when the line has cmd markers
// (a caret escape or a doubled quote inside double quotes) and no single-quoted span.
type quoteStyle int

const (
	stylePOSIX quoteStyle = iota
	styleWindows
)

func tokenizeLine(line string) ([]string, error) {
	return tokenize(strings.TrimSpace(line), detectStyle(line))
}

func detectStyle(line string) quoteStyle {
	if hasSingleQuotedSpan(line) {
		return stylePOSIX
	}
	if hasWindowsMarkers(line) {
		return styleWindows
	}
	return stylePOSIX
}

func hasSingleQuotedSpan(line string) bool {
	inDouble := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if !inDouble && i+1 < len(line) {
				i++
			}
		case '"':
			inDouble = !inDouble
		case '\'':
			if !inDouble {
				return true
			}
		}
	}
	return false
}

func hasWindowsMarkers(line string) bool {
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			if inDouble && i+1 < len(line) && line[i+1] == '"' {
				return true
			}
			inDouble = !inDouble
			continue
		}
		if c == '^' && !inDouble && i+1 < len(line) {
			return true
		}
	}
	return false
}

func tokenize(line string, style quoteStyle) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inToken := false
	flush := func() {
		if !inToken {
			return
		}
		tokens = append(tokens, cur.String())
		cur.Reset()
		inToken = false
	}
	write := func(b byte) {
		cur.WriteByte(b)
		inToken = true
	}

	for i := 0; i < len(line); i++ {
		c := line[i]
		if style == styleWindows {
			if c == '^' && i+1 < len(line) {
				i++
				write(line[i])
				continue
			}
			if c == '"' {
				i = consumeWindowsQuotes(line, i+1, write)
				continue
			}
			if c == ' ' || c == '\t' {
				flush()
				continue
			}
			write(c)
			continue
		}

		switch c {
		case ' ', '\t':
			flush()
		case '\'':
			i = consumePOSIXSingle(line, i+1, write)
		case '"':
			i = consumePOSIXDouble(line, i+1, write)
		case '\\':
			if i+1 < len(line) {
				i++
				write(line[i])
			}
		default:
			write(c)
		}
	}
	flush()
	return tokens, nil
}

func consumeWindowsQuotes(line string, start int, write func(byte)) int {
	for i := start; i < len(line); i++ {
		if line[i] != '"' {
			write(line[i])
			continue
		}
		if i+1 < len(line) && line[i+1] == '"' {
			write('"')
			i++
			continue
		}
		return i
	}
	return len(line) - 1
}

func consumePOSIXSingle(line string, start int, write func(byte)) int {
	for i := start; i < len(line); i++ {
		if line[i] == '\'' {
			return i
		}
		write(line[i])
	}
	return len(line) - 1
}

func consumePOSIXDouble(line string, start int, write func(byte)) int {
	for i := start; i < len(line); i++ {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			next := line[i+1]
			if next == '\\' || next == '"' || next == '$' || next == '`' || next == '\n' {
				write(next)
				i++
				continue
			}
		}
		if c == '"' {
			return i
		}
		write(c)
	}
	return len(line) - 1
}

// splitCommands breaks a paste into individual command lines.
// Newlines, unquoted && and ; separate commands. A trailing \ (POSIX) or ^ (cmd)
// continues onto the next physical line.
func splitCommands(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	joined := make([]string, 0, len(lines))
	var pending string
	for _, line := range lines {
		if pending != "" {
			line = pending + line
			pending = ""
		}
		trimmedRight := strings.TrimRight(line, " \t")
		if strings.HasSuffix(trimmedRight, "\\") && !strings.HasSuffix(trimmedRight, "\\\\") {
			pending = strings.TrimSuffix(trimmedRight, "\\")
			continue
		}
		if strings.HasSuffix(trimmedRight, "^") && !strings.HasSuffix(trimmedRight, "^^") {
			pending = strings.TrimSuffix(trimmedRight, "^")
			continue
		}
		joined = append(joined, line)
	}
	if pending != "" {
		joined = append(joined, pending)
	}

	var commands []string
	for _, line := range joined {
		commands = append(commands, splitStatements(line)...)
	}
	return commands
}

func splitStatements(line string) []string {
	var parts []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\\' && !inSingle && i+1 < len(line) {
			cur.WriteByte(c)
			i++
			cur.WriteByte(line[i])
			continue
		}
		if c == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteByte(c)
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteByte(c)
			continue
		}
		if !inSingle && !inDouble {
			if c == ';' {
				parts = append(parts, cur.String())
				cur.Reset()
				continue
			}
			if c == '&' && i+1 < len(line) && line[i+1] == '&' {
				parts = append(parts, cur.String())
				cur.Reset()
				i++
				continue
			}
		}
		cur.WriteByte(c)
	}
	parts = append(parts, cur.String())
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, part)
		}
	}
	return out
}
