package session

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// buildSafariBlob constructs a minimal Safari binarycookies file with a
// single page containing the given cookies. Sufficient to exercise the
// parser's header/page/record paths.
type safariCookieInput struct {
	Name, Value, Domain, Path string
	Secure                    bool
	ExpiresMacAbs             float64 // seconds since 2001-01-01 UTC; 0 → session cookie
	CreationMacAbs            float64 // seconds since 2001-01-01 UTC; 0 → unknown
}

func buildSafariBlob(t *testing.T, cookies []safariCookieInput) []byte {
	t.Helper()

	// Build the single page.
	numCookies := uint32(len(cookies))
	// Page layout:
	//  [0..4)   page magic 0x00000100 (LE)
	//  [4..8)   num_cookies (LE)
	//  [8..8+4*N) record offsets
	//  then records, each record is a header + 4 cstrings.

	// First, reserve offsets. We lay records out sequentially after the
	// offset table. Each record's cstring offsets are relative to the record
	// start.
	recordHeaderSize := 4 + 4 + 4 + 4 + 4 + 4 + 4 + 4 + 8 + 8 + 8 // 56 bytes (+ 4 size prefix, included in "4" at offset 0)

	// Compute record payloads so we know final sizes before stitching.
	type rec struct {
		size                                          uint32
		flags                                         uint32
		domainOff, nameOff, pathOff, valueOff         uint32
		expiresBits, creationBits                     uint64
		domain, name, path, value                     []byte
	}
	records := make([]rec, len(cookies))
	pageCursor := uint32(8 + 4*numCookies)
	recordOffsets := make([]uint32, numCookies)
	for i, c := range cookies {
		r := rec{}
		if c.Secure {
			r.flags |= 0x0001
		}
		r.domain = append([]byte(c.Domain), 0)
		r.name = append([]byte(c.Name), 0)
		r.path = append([]byte(c.Path), 0)
		r.value = append([]byte(c.Value), 0)

		// Offsets inside the record.
		r.domainOff = uint32(recordHeaderSize)
		r.nameOff = r.domainOff + uint32(len(r.domain))
		r.pathOff = r.nameOff + uint32(len(r.name))
		r.valueOff = r.pathOff + uint32(len(r.path))

		r.size = r.valueOff + uint32(len(r.value))
		r.expiresBits = math.Float64bits(c.ExpiresMacAbs)
		r.creationBits = math.Float64bits(c.CreationMacAbs)

		records[i] = r
		recordOffsets[i] = pageCursor
		pageCursor += r.size
	}

	pageSize := pageCursor
	page := make([]byte, pageSize)
	binary.LittleEndian.PutUint32(page[0:4], 0x00000100)
	binary.LittleEndian.PutUint32(page[4:8], numCookies)
	for i, off := range recordOffsets {
		binary.LittleEndian.PutUint32(page[8+4*i:8+4*i+4], off)
	}

	for i, r := range records {
		base := recordOffsets[i]
		binary.LittleEndian.PutUint32(page[base+0:base+4], r.size)
		// bytes [4..8) unknown — leave 0
		binary.LittleEndian.PutUint32(page[base+8:base+12], r.flags)
		// bytes [12..16) unknown — leave 0
		binary.LittleEndian.PutUint32(page[base+16:base+20], r.domainOff)
		binary.LittleEndian.PutUint32(page[base+20:base+24], r.nameOff)
		binary.LittleEndian.PutUint32(page[base+24:base+28], r.pathOff)
		binary.LittleEndian.PutUint32(page[base+28:base+32], r.valueOff)
		// bytes [32..40) unknown — leave 0
		binary.LittleEndian.PutUint64(page[base+40:base+48], r.expiresBits)
		binary.LittleEndian.PutUint64(page[base+48:base+56], r.creationBits)

		copy(page[base+r.domainOff:], r.domain)
		copy(page[base+r.nameOff:], r.name)
		copy(page[base+r.pathOff:], r.path)
		copy(page[base+r.valueOff:], r.value)
	}

	// Header: "cook" + BE uint32 num_pages + BE uint32 pagesize.
	var header bytes.Buffer
	header.WriteString("cook")
	binary.Write(&header, binary.BigEndian, uint32(1))
	binary.Write(&header, binary.BigEndian, pageSize)

	return append(header.Bytes(), page...)
}

func TestParseSafariCookiesRoundTrip(t *testing.T) {
	// Expire 1 hour after the Mac epoch (just a reasonable nonzero time).
	macExpires := float64(800000000) // 2026-05 ish in mac absolute time
	wantUnix := int64(macExpires) + macEpochOffset

	blob := buildSafariBlob(t, []safariCookieInput{
		{Name: "__Secure-1PSID", Value: "abc", Domain: ".google.com", Path: "/", Secure: true, ExpiresMacAbs: macExpires},
		{Name: "TMP", Value: "x", Domain: ".google.com", Path: "/", Secure: false, ExpiresMacAbs: 0},
	})

	cookies, err := ParseSafariCookies(blob)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(cookies) != 2 {
		t.Fatalf("want 2 cookies, got %d", len(cookies))
	}

	psid := cookies[0]
	if psid.Name != "__Secure-1PSID" || psid.Value != "abc" || psid.Domain != ".google.com" {
		t.Errorf("fields lost: %+v", psid)
	}
	if !psid.Secure {
		t.Errorf("secure flag lost")
	}
	if psid.Expires == nil || psid.Expires.Unix() != wantUnix {
		t.Errorf("expires wrong: got %v want unix %d", psid.Expires, wantUnix)
	}

	tmp := cookies[1]
	if !tmp.Session || tmp.Expires != nil {
		t.Errorf("mac expires=0 should be session cookie: %+v", tmp)
	}
}

func TestParseSafariCookiesRejectsBadMagic(t *testing.T) {
	_, err := ParseSafariCookies([]byte("XXXX\x00\x00\x00\x00"))
	if err == nil {
		t.Fatal("expected error on bad magic")
	}
}

func TestParseSafariCookiesRejectsShortInput(t *testing.T) {
	_, err := ParseSafariCookies([]byte("coo"))
	if err == nil {
		t.Fatal("expected error on short input")
	}
}

func TestParseSafariSnapshotsCapturesCreationDate(t *testing.T) {
	// Creation at a known Mac absolute time → expected Unix timestamp.
	creationMac := float64(780000000)
	wantCreationUnix := int64(creationMac) + macEpochOffset

	blob := buildSafariBlob(t, []safariCookieInput{
		{
			Name: "SID", Value: "v", Domain: ".google.com", Path: "/",
			Secure: true, ExpiresMacAbs: 800000000, CreationMacAbs: creationMac,
		},
	})
	snaps, err := ParseSafariSnapshots(blob, "safari(Test)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].Source != "safari(Test)" {
		t.Errorf("source label wrong: %q", snaps[0].Source)
	}
	if snaps[0].LastSeen.Unix() != wantCreationUnix {
		t.Errorf("LastSeen wrong: got %v want unix %d", snaps[0].LastSeen, wantCreationUnix)
	}
}

// Keeping this reference so that future format changes (or hypothetical
// off-by-one regressions) get an obvious test reference.
func TestSafariMacEpochOffsetMatchesNSDate(t *testing.T) {
	// NSDate epoch = 2001-01-01 00:00:00 UTC.
	want := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if int64(macEpochOffset) != want {
		t.Errorf("macEpochOffset = %d, want %d", macEpochOffset, want)
	}
}
