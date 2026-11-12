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
var sizeField = regexp.MustCompile(`/Size\s+([0-9]+)\b`)
var rootField = regexp.MustCompile(`/Root\s+([0-9]+)\s+([0-9]+)\s+R\b`)
var previousTable = regexp.MustCompile(`/(Prev|XRefStm)\b`)

const UnsupportedLayout = "unsupported PDF layout: requires a standalone classic xref table and direct catalog"

// Validate is a bounded structural/signature gate for classic-xref PDFs, not a
// full PDF parser, page renderer or malware scanner. Unsupported layouts fail closed.
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
	return validateClassic(content, offset, match[0])
}

type reference struct {
	offset, generation int
	used               bool
}

func validateClassic(content []byte, offset, end int) string {
	// Normalize only the xref/trailer region, never file bytes used by offsets.
	tail := bytes.ReplaceAll(content[offset:end], []byte("\r\n"), []byte("\n"))
	tail = bytes.ReplaceAll(tail, []byte("\r"), []byte("\n"))
	lines := bytes.Split(tail, []byte("\n"))
	if string(trimPDF(lines[0])) != "xref" {
		return UnsupportedLayout
	}
	refs := make(map[int]reference)
	cursor := 1
	var trailer []byte
	for cursor < len(lines) {
		line := trimPDF(lines[cursor])
		cursor++
		if len(line) == 0 || line[0] == '%' {
			continue
		}
		if bytes.HasPrefix(line, []byte("trailer")) {
			trailer = append(append([]byte(nil), line[len("trailer"):]...), '\n')
			trailer = append(trailer, bytes.Join(lines[cursor:], []byte("\n"))...)
			break
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			return "invalid PDF cross-reference subsection"
		}
		first, err1 := strconv.Atoi(string(fields[0]))
		count, err2 := strconv.Atoi(string(fields[1]))
		// No loop/large allocation is driven by unchecked file-supplied counts.
		if err1 != nil || err2 != nil || first < 0 || count <= 0 || count > len(lines)-cursor || first > int(^uint(0)>>1)-count {
			return "invalid PDF cross-reference count"
		}
		for index := 0; index < count; index++ {
			entry := xrefEntry.FindSubmatch(trimPDF(lines[cursor]))
			cursor++
			if entry == nil {
				return "invalid PDF cross-reference entry"
			}
			at, err1 := strconv.Atoi(string(entry[1]))
			generation, err2 := strconv.Atoi(string(entry[2]))
			id := first + index
			if _, exists := refs[id]; exists {
				return "duplicate PDF cross-reference entry"
			}
			if err1 != nil || err2 != nil || generation > 65535 {
				return "invalid PDF cross-reference entry"
			}
			used := string(entry[3]) == "n"
			if used {
				if at < 0 || at >= offset || objectBody(content[:offset], id, generation, at) == nil {
					return "invalid PDF object reference"
				}
			}
			refs[id] = reference{offset: at, generation: generation, used: used}
		}
	}
	trailer = trimPDF(trailer)
	if len(refs) == 0 || !bytes.HasPrefix(trailer, []byte("<<")) || !bytes.HasSuffix(trailer, []byte(">>")) {
		return "invalid PDF trailer signature"
	}
	// Resolving earlier tables/hybrid streams needs a PDF object resolver. Do not
	// pretend a signature search implements that resolver.
	if previousTable.Match(trailer) {
		return UnsupportedLayout
	}
	sizes := sizeField.FindAllSubmatch(trailer, -1)
	roots := rootField.FindAllSubmatch(trailer, -1)
	if len(sizes) != 1 || len(roots) != 1 {
		return "invalid PDF trailer references"
	}
	size, err := strconv.Atoi(string(sizes[0][1]))
	if err != nil || size <= 0 {
		return "invalid PDF trailer size"
	}
	for id := range refs {
		if id >= size {
			return "invalid PDF trailer size"
		}
	}
	rootID, err1 := strconv.Atoi(string(roots[0][1]))
	generation, err2 := strconv.Atoi(string(roots[0][2]))
	root, exists := refs[rootID]
	if err1 != nil || err2 != nil || !exists || !root.used || root.generation != generation {
		return "invalid PDF catalog reference"
	}
	body := indirectBody(content[:offset], rootID, generation, root.offset)
	if !hasDirectCatalogType(body) {
		return "invalid PDF catalog signature"
	}
	return ""
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
