// Package json provides helpers for decoding LLM-produced JSON that may be truncated.
package json

import (
	stdjson "encoding/json"
	"strings"
)

// Unmarshal decodes data into v like encoding/json.Unmarshal.
// If the first decode fails, it attempts to repair truncated JSON and retries.
// repaired is true when the successful decode used a repaired payload.
func Unmarshal(data []byte, v any) (repaired bool, err error) {
	if err := stdjson.Unmarshal(data, v); err == nil {
		return false, nil
	} else {
		repairedTxt := repairTruncated(string(data))
		if repairedTxt == "" {
			return false, err
		}
		if err2 := stdjson.Unmarshal([]byte(repairedTxt), v); err2 == nil {
			return true, nil
		}
		return false, err
	}
}

// repairTruncated attempts to make truncated JSON parseable by closing open
// strings/structures and dropping incomplete trailing keys or values.
func repairTruncated(s string) string {
	s = strings.TrimRight(s, " \t\r\n")
	if s == "" {
		return s
	}

	inString, _ := scanState(s)
	if inString {
		if endsWithEscapedBackslash(s) {
			s = s[:len(s)-1]
		}
		s += `"`
	}

	for {
		s = strings.TrimRight(s, " \t\r\n")
		if s == "" {
			return s
		}
		inString, _ = scanState(s)
		if inString {
			break
		}

		if trimmed, found := strings.CutSuffix(s, ","); found {
			s = trimmed
			continue
		}

		if strings.HasSuffix(s, ":") {
			s = strings.TrimRight(s[:len(s)-1], " \t\r\n")
			s = trimTrailingString(s)
			s = strings.TrimRight(s, " \t\r\n")
			s = strings.TrimSuffix(s, ",")
			continue
		}

		before, ok := splitTrailingString(s)
		if !ok {
			break
		}
		beforeTrimmed := strings.TrimRight(before, " \t\r\n")
		if strings.HasSuffix(beforeTrimmed, ":") {
			break
		}
		if strings.HasSuffix(beforeTrimmed, ",") || strings.HasSuffix(beforeTrimmed, "{") {
			s = strings.TrimSuffix(beforeTrimmed, ",")
			continue
		}
		break
	}

	s = strings.TrimRight(s, " \t\r\n")
	_, stack := scanState(s)
	var b strings.Builder
	b.Grow(len(s) + len(stack))
	b.WriteString(s)
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			b.WriteByte('}')
		} else {
			b.WriteByte(']')
		}
	}
	return b.String()
}

func scanState(s string) (inString bool, stack []byte) {
	escape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, c)
		case '}':
			if len(stack) > 0 && stack[len(stack)-1] == '{' {
				stack = stack[:len(stack)-1]
			}
		case ']':
			if len(stack) > 0 && stack[len(stack)-1] == '[' {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return inString, stack
}

func endsWithEscapedBackslash(s string) bool {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

func splitTrailingString(s string) (before string, ok bool) {
	if !strings.HasSuffix(s, `"`) {
		return "", false
	}
	i := len(s) - 2
	for i >= 0 {
		if s[i] != '\\' {
			break
		}
		i--
	}
	bs := len(s) - 2 - i
	if bs%2 == 1 {
		return "", false
	}
	for j := len(s) - 2; j >= 0; j-- {
		if s[j] != '"' {
			continue
		}
		k := j - 1
		esc := 0
		for k >= 0 && s[k] == '\\' {
			esc++
			k--
		}
		if esc%2 == 1 {
			continue
		}
		return s[:j], true
	}
	return "", false
}

func trimTrailingString(s string) string {
	before, ok := splitTrailingString(s)
	if !ok {
		return s
	}
	return before
}
