package unified

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// ParseError is a sentinel for JSON parse failures — yielded as an event, NOT thrown.
type ParseError struct {
	Line  string
	Error string
}

// IsParseError type guard (mirrors clowder-ai's isParseError).
func IsParseError(v any) bool {
	_, ok := v.(ParseError)
	return ok
}

// ParseNDJSON reads stdout line-by-line, yielding parsed objects or ParseError sentinels.
// Blank lines are silently skipped. Parse failures NEVER break the stream.
func ParseNDJSON(r io.Reader) <-chan any {
	ch := make(chan any, 64)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				ch <- ParseError{Line: line, Error: err.Error()}
				continue
			}
			ch <- obj
		}
	}()
	return ch
}
