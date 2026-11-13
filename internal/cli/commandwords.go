package cli

import (
	"fmt"
	"strings"
	"unicode"
)

// commandWords splits arguments, not shell programs. Quotes and escapes group
// literal arguments; unquoted shell operators and substitutions are rejected
// rather than giving the impression that a shell will evaluate them. partial
// permits unfinished quotes/escapes for completion.
func commandWords(line string, partial bool) ([]string, error) {
	var words []string
	var word strings.Builder
	var quote rune
	started, escaped := false, false
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	runes := []rune(line)
	for i, r := range runes {
		if escaped {
			// Inside double quotes, backslash only escapes quotation,
			// backslash and shell-special characters. Keep JSON's \n etc.
			if quote == '"' && !strings.ContainsRune("\\\"$`", r) {
				word.WriteRune('\\')
			}
			word.WriteRune(r)
			escaped = false
			continue
		}
		if quote == '\'' {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\\' {
			escaped, started = true, true
			continue
		}
		if quote == '"' {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		switch {
		case r == '\'' || r == '"':
			quote, started = r, true
		case unicode.IsSpace(r):
			flush()
		case strings.ContainsRune("|&;<>`", r) || r == '$' && i+1 < len(runes) && runes[i+1] == '(':
			return nil, fmt.Errorf("command mode runs hc commands, not shell operators; quote literal arguments")
		default:
			started = true
			word.WriteRune(r)
		}
	}
	if !partial && (quote != 0 || escaped) {
		return nil, fmt.Errorf("finish the quoted argument or escape")
	}
	flush()
	return words, nil
}

// commandQuote makes a word round-trip through commandWords, including empty
// strings, whitespace, JSON and quotes.
func commandQuote(s string) string {
	if s != "" && strings.IndexFunc(s, unicode.IsSpace) < 0 && !strings.ContainsAny(s, "'\"\\|&;<>`$") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func commandJoin(words []string) string {
	quoted := make([]string, len(words))
	for i, word := range words {
		quoted[i] = commandQuote(word)
	}
	return strings.Join(quoted, " ")
}
