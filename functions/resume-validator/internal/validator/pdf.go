package validator

import (
	"bytes"
	"regexp"
	"strconv"
)

var header = regexp.MustCompile(`^%PDF-(1\.[0-7]|2\.0)[\r\n]`)
var ending = regexp.MustCompile(`startxref[\r\n ]+([0-9]+)[\r\n ]+%%EOF[\x00\x09\x0a\x0c\x0d\x20]*$`)
var indirectHeader = regexp.MustCompile(`^([0-9]+)\s+([0-9]+)\s+obj(?:[\x00\t\n\f\r ]|<)`)
var xrefEntry = regexp.MustCompile(`^([0-9]{10})[ \t]([0-9]{5})[ \t]([nf])[ \t]*$`)

const UnsupportedLayout = "unsupported PDF stream encoding"

// Validate is a bounded structural/signature gate, not a full PDF parser,
// page renderer or malware scanner. Unsupported encodings fail closed.
func Validate(contentType string, content []byte) string {
	if contentType != "application/pdf" {
		return "content_type must be application/pdf"
	}
	if len(content) == 0 || len(content) > MaxBytes {
		return "PDF size must be between 1 and 2000000 bytes"
	}
	if !header.Match(content) {
		return "invalid PDF header signature"
	}
	match := ending.FindSubmatchIndex(content)
	if match == nil {
		return "invalid PDF end signature"
	}
	offset, err := strconv.Atoi(string(content[match[2]:match[3]]))
	if err != nil || offset < 0 || offset >= match[0] {
		return "invalid PDF cross-reference offset"
	}
	return validateReferences(content, offset, match[0])
}

type reference struct {
	offset, generation int
	used               bool
	stream, index      int
}

// readClassic retains the entry/offset checks for every revision, but returns
// its dictionary and entries so /Prev can supply unchanged objects.
func readClassic(content []byte, offset, end int, latest bool) (map[int]reference, pdfDict, string) {
	tail := bytes.ReplaceAll(content[offset:end], []byte("\r\n"), []byte("\n"))
	tail = bytes.ReplaceAll(tail, []byte("\r"), []byte("\n"))
	lines := bytes.Split(tail, []byte("\n"))
	if string(trimPDF(lines[0])) != "xref" {
		return nil, nil, "invalid PDF cross-reference signature"
	}
	refs := make(map[int]reference)
	cursor := 1
	for cursor < len(lines) {
		line := trimPDF(lines[cursor])
		cursor++
		if len(line) == 0 || line[0] == '%' {
			continue
		}
		if bytes.HasPrefix(line, []byte("trailer")) {
			raw := append(append([]byte(nil), line[len("trailer"):]...), '\n')
			raw = append(raw, bytes.Join(lines[cursor:], []byte("\n"))...)
			dict, n, ok := readDictionary(raw)
			if !ok || len(refs) == 0 || !validSectionEnd(raw[n:], offset, latest) {
				return nil, nil, "invalid PDF trailer signature"
			}
			return refs, dict, ""
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			return nil, nil, "invalid PDF cross-reference subsection"
		}
		first, e1 := strconv.Atoi(string(fields[0]))
		count, e2 := strconv.Atoi(string(fields[1]))
		if e1 != nil || e2 != nil || first < 0 || count <= 0 || count > len(lines)-cursor || first > maxPDFEntries-count {
			return nil, nil, "invalid PDF cross-reference count"
		}
		for index := 0; index < count; index++ {
			entry := xrefEntry.FindSubmatch(trimPDF(lines[cursor]))
			cursor++
			if entry == nil {
				return nil, nil, "invalid PDF cross-reference entry"
			}
			at, e1 := strconv.Atoi(string(entry[1]))
			generation, e2 := strconv.Atoi(string(entry[2]))
			id := first + index
			if _, exists := refs[id]; exists {
				return nil, nil, "duplicate PDF cross-reference entry"
			}
			if e1 != nil || e2 != nil || generation > 65535 {
				return nil, nil, "invalid PDF cross-reference entry"
			}
			used := string(entry[3]) == "n"
			if used && (at >= offset || objectBody(content[:offset], id, generation, at) == nil) {
				return nil, nil, "invalid PDF object reference"
			}
			refs[id] = reference{offset: at, generation: generation, used: used}
		}
	}
	return nil, nil, "invalid PDF trailer signature"
}

func indirectBody(content []byte, id, generation, offset int) []byte {
	if offset < 0 || offset >= len(content) {
		return nil
	}
	object := content[offset:]
	// Anchoring matters: a valid object elsewhere cannot repair a forged offset.
	start := indirectHeader.FindSubmatchIndex(object)
	if start == nil {
		return nil
	}
	actualID, err1 := strconv.Atoi(string(object[start[2]:start[3]]))
	actualGen, err2 := strconv.Atoi(string(object[start[4]:start[5]]))
	if err1 != nil || err2 != nil || actualID != id || actualGen != generation {
		return nil
	}
	// Preserve a '<' delimiter consumed by the header expression.
	from := start[1]
	if object[from-1] == '<' {
		from--
	}
	return object[from:]
}

func objectBody(content []byte, id, generation, offset int) []byte {
	body := indirectBody(content, id, generation, offset)
	stop := bytes.Index(body, []byte("endobj"))
	if stop < 0 {
		return nil
	}
	return trimPDF(body[:stop])
}
func trimPDF(data []byte) []byte { return bytes.Trim(data, "\x00\t\n\f\r ") }
