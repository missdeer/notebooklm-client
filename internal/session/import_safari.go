package session

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/missdeer/notebooklm-client/internal/types"
)

// macEpochOffset is the offset between Mac absolute time (seconds since
// 2001-01-01 UTC) and Unix epoch (seconds since 1970-01-01 UTC).
const macEpochOffset = 978307200

// safariRecord carries a parsed cookie plus metadata we want to preserve for
// multi-source reconciliation.
type safariRecord struct {
	cookie    types.SessionCookie
	createdAt time.Time // zero if the record did not carry a valid creation time
}

// ParseSafariSnapshots parses a Safari Cookies.binarycookies blob and returns
// each cookie paired with its creation timestamp (LastSeen).
func ParseSafariSnapshots(data []byte, label string) ([]CookieSnapshot, error) {
	records, err := parseSafariAllRecords(data)
	if err != nil {
		return nil, err
	}
	out := make([]CookieSnapshot, 0, len(records))
	for _, r := range records {
		out = append(out, CookieSnapshot{
			Cookie:   r.cookie,
			LastSeen: r.createdAt,
			Source:   label,
		})
	}
	return out, nil
}

// ParseSafariCookies parses Safari's Cookies.binarycookies format.
//
// Format (big-endian in header, little-endian inside pages):
//
//	header:
//	  magic "cook"
//	  uint32 BE num_pages
//	  uint32 BE page_sizes[num_pages]
//	pages (concatenated, each page is page_sizes[i] bytes):
//	  uint32 LE magic 0x00000100
//	  uint32 LE num_cookies
//	  uint32 LE record_offsets[num_cookies]
//	  ... records
//	record (offsets within page):
//	  uint32 LE record_size
//	  uint32 LE (unknown)
//	  uint32 LE flags  (bit 0 = Secure)
//	  uint32 LE (unknown)
//	  uint32 LE domain_offset
//	  uint32 LE name_offset
//	  uint32 LE path_offset
//	  uint32 LE value_offset
//	  uint64 LE (unknown)
//	  float64 LE expires (Mac absolute time)
//	  float64 LE creation (Mac absolute time)
//	  cstrings at the specified offsets
//
// References:
//   - https://github.com/libyal/dtformats/blob/main/documentation/Safari%20Cookies.asciidoc
//   - yt-dlp/yt_dlp/cookies.py
func ParseSafariCookies(data []byte) ([]types.SessionCookie, error) {
	records, err := parseSafariAllRecords(data)
	if err != nil {
		return nil, err
	}
	out := make([]types.SessionCookie, 0, len(records))
	for _, r := range records {
		out = append(out, r.cookie)
	}
	return out, nil
}

func parseSafariAllRecords(data []byte) ([]safariRecord, error) {
	if len(data) < 8 {
		return nil, errors.New("safari cookies: data too short")
	}
	if string(data[0:4]) != "cook" {
		return nil, fmt.Errorf("safari cookies: bad magic %q, expected 'cook'", data[0:4])
	}
	numPages := binary.BigEndian.Uint32(data[4:8])
	headerEnd := 8 + 4*int(numPages)
	if headerEnd > len(data) {
		return nil, errors.New("safari cookies: truncated page table")
	}
	pageSizes := make([]uint32, numPages)
	for i := uint32(0); i < numPages; i++ {
		pageSizes[i] = binary.BigEndian.Uint32(data[8+4*i : 8+4*i+4])
	}

	var out []safariRecord
	cursor := headerEnd
	for _, size := range pageSizes {
		end := cursor + int(size)
		if end > len(data) {
			return nil, fmt.Errorf("safari cookies: page extends past end (cursor=%d size=%d len=%d)", cursor, size, len(data))
		}
		pageRecords, err := parseSafariPage(data[cursor:end])
		if err != nil {
			return nil, err
		}
		out = append(out, pageRecords...)
		cursor = end
	}
	return out, nil
}

func parseSafariPage(page []byte) ([]safariRecord, error) {
	if len(page) < 8 {
		return nil, errors.New("safari cookies: page too short")
	}
	if binary.LittleEndian.Uint32(page[0:4]) != 0x00000100 {
		return nil, fmt.Errorf("safari cookies: bad page signature %x", page[0:4])
	}
	numCookies := binary.LittleEndian.Uint32(page[4:8])
	if 8+4*int(numCookies) > len(page) {
		return nil, errors.New("safari cookies: truncated cookie offset table")
	}

	out := make([]safariRecord, 0, numCookies)
	for i := uint32(0); i < numCookies; i++ {
		recordOffset := binary.LittleEndian.Uint32(page[8+4*i : 8+4*i+4])
		if int(recordOffset) >= len(page) {
			return nil, fmt.Errorf("safari cookies: record offset %d past page end %d", recordOffset, len(page))
		}
		rec, err := parseSafariRecord(page[recordOffset:], page, int(recordOffset))
		if err != nil {
			return nil, err
		}
		if rec != nil {
			out = append(out, *rec)
		}
	}
	return out, nil
}

// parseSafariRecord reads one cookie record. `record` is the tail of `page`
// starting at the record's offset; `page` is passed in because the offset
// fields inside the record are page-relative.
func parseSafariRecord(record []byte, page []byte, recordStart int) (*safariRecord, error) {
	const headerSize = 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 8 + 8
	if len(record) < headerSize {
		return nil, errors.New("safari cookies: record too short")
	}
	flags := binary.LittleEndian.Uint32(record[8:12])
	domainOffRel := binary.LittleEndian.Uint32(record[16:20])
	nameOffRel := binary.LittleEndian.Uint32(record[20:24])
	pathOffRel := binary.LittleEndian.Uint32(record[24:28])
	valueOffRel := binary.LittleEndian.Uint32(record[28:32])
	expiresBits := binary.LittleEndian.Uint64(record[40:48])
	creationBits := binary.LittleEndian.Uint64(record[48:56])

	isSecure := flags&0x0001 != 0

	readCStr := func(relOffset uint32) (string, error) {
		abs := recordStart + int(relOffset)
		if abs < 0 || abs >= len(page) {
			return "", fmt.Errorf("cstring offset out of range: %d (page len %d)", abs, len(page))
		}
		// Scan until NUL.
		end := abs
		for end < len(page) && page[end] != 0 {
			end++
		}
		return string(page[abs:end]), nil
	}

	domain, err := readCStr(domainOffRel)
	if err != nil {
		return nil, fmt.Errorf("domain: %w", err)
	}
	name, err := readCStr(nameOffRel)
	if err != nil {
		return nil, fmt.Errorf("name: %w", err)
	}
	path, err := readCStr(pathOffRel)
	if err != nil {
		return nil, fmt.Errorf("path: %w", err)
	}
	value, err := readCStr(valueOffRel)
	if err != nil {
		return nil, fmt.Errorf("value: %w", err)
	}

	if name == "" || domain == "" {
		return nil, nil // skip malformed
	}

	cookie := types.SessionCookie{
		Name:   name,
		Value:  value,
		Domain: domain,
		Path:   path,
		Secure: isSecure,
	}

	expiresMac := math.Float64frombits(expiresBits)
	if expiresMac > 0 {
		unixSec := int64(expiresMac) + macEpochOffset
		t := time.Unix(unixSec, 0).UTC()
		cookie.Expires = &t
	} else {
		cookie.Session = true
	}

	out := &safariRecord{cookie: cookie}
	createdMac := math.Float64frombits(creationBits)
	if createdMac > 0 {
		out.createdAt = time.Unix(int64(createdMac)+macEpochOffset, 0).UTC()
	}
	return out, nil
}
