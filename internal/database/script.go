package database

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"strings"
	"unicode"
)

type DatabaseScript struct {
	Path  string
	Bytes []byte
}

func (DatabaseScript) Root() string {
	return "sql"
}

func (DatabaseScript) Table() string {
	return "hyperfeed_scripts"
}

func (script *DatabaseScript) Checksum() string {
	sum := sha256.Sum256(script.Bytes)
	return hex.EncodeToString(sum[:])
}

func (DatabaseScript) List(root string, match func(path string, d fs.DirEntry) bool) (scripts []DatabaseScript, err error) {
	err = fs.WalkDir(sqlFS, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if match != nil && !match(path, d) {
			return nil
		}
		if b, err := sqlFS.ReadFile(path); err != nil {
			return fmt.Errorf("read database script %q: %w", path, err)
		} else if len(bytes.TrimSpace(b)) == 0 {
			return nil
		} else {
			scripts = append(scripts, DatabaseScript{Path: path, Bytes: b})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scripts, nil
}

func (script *DatabaseScript) IsTransaction() bool {
	var token []rune
	var inLineComment bool
	var inBlockComment bool
	var inSingleQuote bool
	var inDoubleQuote bool
	var inBracketQuote bool

	flushToken := func() bool {
		if len(token) == 0 {
			return false
		}
		word := strings.ToLower(string(token))
		token = token[:0]
		switch word {
		case "begin", "commit", "rollback":
			return true
		default:
			return false
		}
	}

	payload := script.Bytes
	for i := 0; i < len(payload); i++ {
		ch := payload[i]
		next := byte(0)
		if i+1 < len(payload) {
			next = payload[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingleQuote {
			if ch == '\'' {
				inSingleQuote = false
			}
			continue
		}
		if inDoubleQuote {
			if ch == '"' {
				inDoubleQuote = false
			}
			continue
		}
		if inBracketQuote {
			if ch == ']' {
				inBracketQuote = false
			}
			continue
		}

		if ch == '-' && next == '-' {
			if flushToken() {
				return true
			}
			inLineComment = true
			i++
			continue
		}
		if ch == '/' && next == '*' {
			if flushToken() {
				return true
			}
			inBlockComment = true
			i++
			continue
		}
		if ch == '\'' {
			if flushToken() {
				return true
			}
			inSingleQuote = true
			continue
		}
		if ch == '"' {
			if flushToken() {
				return true
			}
			inDoubleQuote = true
			continue
		}
		if ch == '[' {
			if flushToken() {
				return true
			}
			inBracketQuote = true
			continue
		}

		if unicode.IsLetter(rune(ch)) {
			token = append(token, rune(ch))
			continue
		}
		if flushToken() {
			return true
		}
	}

	return flushToken()
}

func (script DatabaseScript) nooped(err error) bool {
	var prefix string = "HYPERFEED_NOOP:"
	if err == nil {
		return false
	}
	if strings.Contains(strings.TrimSpace(err.Error()), prefix) {
		fmt.Printf("maintenance %q no-op\n", script.Path)
		return true
	}
	return false
}
