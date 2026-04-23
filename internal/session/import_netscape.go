package session

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// httpOnlyPrefix marks HttpOnly cookies in the Netscape format (extension
// originally introduced by curl).
const httpOnlyPrefix = "#HttpOnly_"

// ParseNetscape parses a Netscape-format cookies.txt stream (curl / wget
// compatible) into SessionCookie records.
//
// Line format: domain \t flag \t path \t secure \t expiry \t name \t value
//
// HttpOnly cookies may be prefixed with "#HttpOnly_". Lines starting with "#"
// (other than the HttpOnly marker) and blank lines are skipped.
func ParseNetscape(r io.Reader) ([]types.SessionCookie, error) {
	var out []types.SessionCookie
	scanner := bufio.NewScanner(r)
	// Some cookies.txt exports contain very long values; bump the buffer.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimRight(raw, "\r")

		httpOnly := false
		if strings.HasPrefix(line, httpOnlyPrefix) {
			httpOnly = true
			line = strings.TrimPrefix(line, httpOnlyPrefix)
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			// Be lenient: many exporters emit fewer columns when value is empty.
			if len(fields) == 6 {
				fields = append(fields, "")
			} else {
				return nil, fmt.Errorf("cookies.txt line %d: expected 7 tab-separated fields, got %d", lineNum, len(fields))
			}
		}

		domain := strings.TrimSpace(fields[0])
		// fields[1] = "TRUE"/"FALSE" include-subdomains flag (derived from
		// leading "." in domain); ignored here since Domain already carries it.
		path := strings.TrimSpace(fields[2])
		secure := strings.EqualFold(strings.TrimSpace(fields[3]), "TRUE")
		expiryStr := strings.TrimSpace(fields[4])
		name := strings.TrimSpace(fields[5])
		value := fields[6]

		if name == "" || domain == "" {
			continue
		}

		cookie := types.SessionCookie{
			Name:     name,
			Value:    value,
			Domain:   domain,
			Path:     path,
			Secure:   secure,
			HttpOnly: httpOnly,
		}

		// Netscape convention: expiry == 0 means session cookie.
		if expiryStr != "" && expiryStr != "0" {
			n, err := strconv.ParseInt(expiryStr, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cookies.txt line %d: invalid expiry %q: %w", lineNum, expiryStr, err)
			}
			t := time.Unix(n, 0).UTC()
			cookie.Expires = &t
		} else {
			cookie.Session = true
		}

		out = append(out, cookie)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
