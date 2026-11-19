package validator

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
)

// Bound metadata work independently of compressed input size. Limits apply to
// the whole file, including old revisions and decoded object streams.
const maxPDFEntries = 100_000
const maxPDFSections = 128
const maxDecodedBytes = 16_000_000

type pdfDict map[string][]byte

type compressedObject struct {
	id   int
	body []byte
}

// Reuse the catalog lexer so strings, comments and nested values cannot forge
// structural keys. Keep raw values for the small subset we actually resolve.
func readDictionary(data []byte) (pdfDict, int, bool) {
	p := catalogLexer{data: data}
	p.space()
	if !p.take("<<") {
		return nil, 0, false
	}
	d := pdfDict{}
	for {
		p.space()
		if p.take(">>") {
			return d, p.pos, true
		}
		key, ok := p.name()
		if !ok {
			return nil, 0, false
		}
		if _, exists := d[key]; exists {
			return nil, 0, false
		}
		p.space()
		start := p.pos
		if !p.value(1) {
			return nil, 0, false
		}
		d[key] = data[start:p.pos]
	}
}
func integer(raw []byte) (int, bool) {
	raw = trimPDF(raw)
	if !pdfUnsigned(raw) {
		return 0, false
	}
	n, err := strconv.Atoi(string(raw))
	return n, err == nil
}
func (d pdfDict) number(key string) (int, bool) { return integer(d[key]) }
func (d pdfDict) nameIs(key, value string) bool {
	p := catalogLexer{data: d[key]}
	s, ok := p.name()
	p.space()
	return ok && s == value && p.pos == len(p.data)
}
func integers(raw []byte) ([]int, bool) {
	p := catalogLexer{data: raw}
	p.space()
	if !p.take("[") {
		return nil, false
	}
	var result []int
	for {
		p.space()
		if p.take("]") {
			p.space()
			return result, p.pos == len(raw)
		}
		n, ok := integer(p.word())
		if !ok || len(result) >= maxPDFEntries {
			return nil, false
		}
		result = append(result, n)
	}
}
func rootReference(raw []byte) (int, int, bool) {
	p := catalogLexer{data: raw}
	p.space()
	id, ok1 := integer(p.word())
	p.space()
	gen, ok2 := integer(p.word())
	p.space()
	ok3 := string(p.word()) == "R"
	p.space()
	return id, gen, ok1 && ok2 && ok3 && gen <= 65535 && p.pos == len(raw)
}

func validateReferences(content []byte, offset, end int) string {
	refs := map[int]reference{}
	seen := map[int]bool{}
	var root []byte
	latestSize := 0
	budget := maxDecodedBytes
	for section := 0; ; section++ {
		if section >= maxPDFSections || seen[offset] || offset < 0 || offset >= end {
			return "invalid PDF cross-reference chain"
		}
		seen[offset] = true
		var entries map[int]reference
		var d pdfDict
		var reason string
		if bytes.HasPrefix(content[offset:], []byte("xref")) {
			entries, d, reason = readClassic(content, offset, end, section == 0)
		} else {
			entries, d, reason = readXrefStream(content, offset, end, &budget, section == 0, true)
		}
		if reason != "" {
			return reason
		}
		size, ok := d.number("Size")
		if !ok || size < 1 || size > maxPDFEntries {
			return "invalid PDF trailer size"
		}
		if section == 0 {
			latestSize = size
			root = d["Root"]
		}
		if size > latestSize {
			return "invalid PDF trailer size"
		}
		// Hybrid-reference files add compressed entries to a classic table. Stream
		// entries take precedence within this revision, per the PDF hybrid rules.
		if raw, exists := d["XRefStm"]; exists {
			hybrid, ok := integer(raw)
			if !ok || hybrid >= offset || seen[hybrid] {
				return "invalid PDF cross-reference chain"
			}
			seen[hybrid] = true
			extra, _, why := readXrefStream(content, hybrid, offset, &budget, false, false)
			if why != "" {
				return why
			}
			for id, ref := range extra {
				entries[id] = ref
			}
		}
		for id, ref := range entries {
			if id >= size {
				return "invalid PDF trailer size"
			}
			if _, exists := refs[id]; !exists {
				refs[id] = ref
			}
		}
		raw, exists := d["Prev"]
		if !exists {
			break
		}
		prev, ok := integer(raw)
		if !ok || prev >= offset {
			return "invalid PDF cross-reference chain"
		}
		end = offset
		offset = prev
	}
	rootID, gen, ok := rootReference(root)
	ref, exists := refs[rootID]
	if !ok || !exists || !ref.used || ref.generation != gen {
		return "invalid PDF catalog reference"
	}
	// Check active references as well as the offsets in each original section.
	// A freed/overridden object in a newer section must never fall back to old data.
	objects := map[int][]compressedObject{}
	for id, ref := range refs {
		if !ref.used {
			continue
		}
		if ref.stream == 0 {
			if objectBody(content, id, ref.generation, ref.offset) == nil {
				return "invalid PDF object reference"
			}
			continue
		}
		if _, exists := objects[ref.stream]; exists {
			continue
		}
		container, exists := refs[ref.stream]
		if !exists || !container.used || container.stream != 0 || container.generation != 0 {
			return "invalid PDF object stream reference"
		}
		bodies, why := readObjectStream(content, container.offset, ref.stream, &budget)
		if why != "" {
			return why
		}
		objects[ref.stream] = bodies
	}
	for id, ref := range refs {
		if !ref.used || ref.stream == 0 {
			continue
		}
		bodies := objects[ref.stream]
		if ref.index >= len(bodies) || len(bodies[ref.index].body) == 0 {
			return "invalid PDF compressed object reference"
		}
		if bodies[ref.index].id != id {
			return "invalid PDF compressed object reference"
		}
	}
	var body []byte
	if ref.stream == 0 {
		body = indirectBody(content, rootID, gen, ref.offset)
	} else {
		stored := objects[ref.stream][ref.index].body
		body = append(append([]byte(nil), stored...), []byte("\nendobj")...)
	}
	if !hasDirectCatalogType(body) {
		return "invalid PDF catalog signature"
	}
	return ""
}

func readXrefStream(content []byte, offset, end int, budget *int, latest, checkEnd bool) (map[int]reference, pdfDict, string) {
	d, data, after, reason := readStream(content[:end], offset, budget)
	if reason != "" {
		return nil, nil, reason
	}
	if checkEnd && !validSectionEnd(content[after:end], offset, latest) {
		return nil, nil, "invalid PDF cross-reference ending"
	}
	if !d.nameIs("Type", "XRef") {
		return nil, nil, "invalid PDF cross-reference stream type"
	}
	size, ok := d.number("Size")
	if !ok || size < 1 || size > maxPDFEntries {
		return nil, nil, "invalid PDF trailer size"
	}
	widths, ok := integers(d["W"])
	if !ok || len(widths) != 3 {
		return nil, nil, "invalid PDF cross-reference widths"
	}
	width := 0
	for _, w := range widths {
		if w > 8 {
			return nil, nil, "invalid PDF cross-reference widths"
		}
		width += w
	}
	if width == 0 {
		return nil, nil, "invalid PDF cross-reference widths"
	}
	indices := []int{0, size}
	if raw, exists := d["Index"]; exists {
		indices, ok = integers(raw)
		if !ok || len(indices) == 0 || len(indices)%2 != 0 {
			return nil, nil, "invalid PDF cross-reference index"
		}
	}
	refs := map[int]reference{}
	cursor := 0
	last := 0
	for i := 0; i < len(indices); i += 2 {
		first, count := indices[i], indices[i+1]
		if first < last || first > size || count > size-first || count > (len(data)-cursor)/width {
			return nil, nil, "invalid PDF cross-reference count"
		}
		last = first + count
		for id := first; id < last; id++ {
			fields := [3]int{}
			for j, w := range widths {
				var n uint64
				for k := 0; k < w; k++ {
					n = n<<8 | uint64(data[cursor])
					cursor++
				}
				if n > uint64(maxPDFEntries) && (j != 1 || n > uint64(len(content))) {
					return nil, nil, "invalid PDF cross-reference entry"
				}
				fields[j] = int(n)
			}
			if widths[0] == 0 {
				fields[0] = 1
			}
			ref := reference{offset: fields[1], generation: fields[2], used: fields[0] != 0}
			switch fields[0] {
			case 0:
				if fields[2] > 65535 {
					return nil, nil, "invalid PDF cross-reference entry"
				}
			case 1:
				if fields[2] > 65535 || fields[1] > offset || objectBody(content[:end], id, fields[2], fields[1]) == nil {
					return nil, nil, "invalid PDF object reference"
				}
			case 2:
				if fields[1] == 0 {
					return nil, nil, "invalid PDF object stream reference"
				}
				ref = reference{used: true, stream: fields[1], index: fields[2]}
			default:
				return nil, nil, "invalid PDF cross-reference entry"
			}
			refs[id] = ref
		}
	}
	if cursor != len(data) || len(refs) == 0 {
		return nil, nil, "invalid PDF cross-reference stream length"
	}
	return refs, d, ""
}

func readObjectStream(content []byte, offset, id int, budget *int) ([]compressedObject, string) {
	if indirectBody(content, id, 0, offset) == nil {
		return nil, "invalid PDF object stream reference"
	}
	d, data, _, reason := readStream(content, offset, budget)
	if reason != "" {
		return nil, reason
	}
	count, ok1 := d.number("N")
	first, ok2 := d.number("First")
	if !d.nameIs("Type", "ObjStm") || !ok1 || !ok2 || count < 1 || count > maxPDFEntries || first > len(data) {
		return nil, "invalid PDF object stream"
	}
	p := catalogLexer{data: data[:first]}
	ids := make([]int, count)
	offsets := make([]int, count)
	seen := map[int]bool{}
	for i := 0; i < count; i++ {
		p.space()
		id, ok1 := integer(p.word())
		p.space()
		at, ok2 := integer(p.word())
		if !ok1 || !ok2 || id < 1 || id >= maxPDFEntries || seen[id] || at >= len(data)-first || (i > 0 && at <= offsets[i-1]) {
			return nil, "invalid PDF object stream index"
		}
		seen[id] = true
		ids[i] = id
		offsets[i] = at
	}
	p.space()
	if p.pos != first {
		return nil, "invalid PDF object stream index"
	}
	bodies := make([]compressedObject, count)
	for i, at := range offsets {
		end := len(data)
		if i+1 < count {
			end = first + offsets[i+1]
		}
		body := trimPDF(data[first+at : end])
		lexer := catalogLexer{data: body}
		if !lexer.value(0) {
			return nil, "invalid PDF compressed object"
		}
		lexer.space()
		if lexer.pos != len(body) {
			return nil, "invalid PDF compressed object"
		}
		bodies[i] = compressedObject{id: ids[i], body: body}
	}
	return bodies, ""
}

func readStream(content []byte, offset int, budget *int) (pdfDict, []byte, int, string) {
	if offset < 0 || offset >= len(content) {
		return nil, nil, 0, "invalid PDF stream offset"
	}
	start := indirectHeader.FindSubmatchIndex(content[offset:])
	if start == nil {
		return nil, nil, 0, "invalid PDF stream object"
	}
	objectID, okID := integer(content[offset+start[2] : offset+start[3]])
	generation, okGen := integer(content[offset+start[4] : offset+start[5]])
	if !okID || !okGen || objectID < 1 || objectID >= maxPDFEntries || generation != 0 {
		return nil, nil, 0, "invalid PDF stream object"
	}
	from := offset + start[1]
	if content[from-1] == '<' {
		from--
	}
	d, n, ok := readDictionary(content[from:])
	if !ok {
		return nil, nil, 0, "invalid PDF stream dictionary"
	}
	p := catalogLexer{data: content, pos: from + n}
	p.space()
	if !p.take("stream") {
		return nil, nil, 0, "invalid PDF stream signature"
	}
	if p.take("\r") {
		p.take("\n")
	} else if !p.take("\n") {
		return nil, nil, 0, "invalid PDF stream signature"
	}
	length, ok := d.number("Length")
	if !ok || length > len(content)-p.pos {
		return nil, nil, 0, "invalid PDF stream length"
	}
	data := content[p.pos : p.pos+length]
	p.pos += length
	p.space()
	if string(p.word()) != "endstream" {
		return nil, nil, 0, "invalid PDF stream length"
	}
	p.space()
	if string(p.word()) != "endobj" {
		return nil, nil, 0, "invalid PDF stream signature"
	}
	if _, exists := d["F"]; exists {
		return nil, nil, 0, UnsupportedLayout
	}
	if filter, exists := d["Filter"]; exists {
		// Accept the standard Flate filter, including its one-element array form.
		if !d.nameIs("Filter", "FlateDecode") {
			q := catalogLexer{data: filter}
			q.space()
			if !q.take("[") {
				return nil, nil, 0, UnsupportedLayout
			}
			name, ok := q.name()
			q.space()
			if !ok || name != "FlateDecode" || !q.take("]") {
				return nil, nil, 0, UnsupportedLayout
			}
			q.space()
			if q.pos != len(filter) {
				return nil, nil, 0, UnsupportedLayout
			}
		}
		reader, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, nil, 0, "invalid PDF compressed stream"
		}
		decoded, err := io.ReadAll(io.LimitReader(reader, int64(*budget)+1))
		closeErr := reader.Close()
		if err != nil || closeErr != nil || len(decoded) > *budget {
			return nil, nil, 0, "invalid PDF compressed stream"
		}
		data = decoded
	}
	if len(data) > *budget {
		return nil, nil, 0, "PDF decoded metadata limit exceeded"
	}
	*budget -= len(data)
	if raw, exists := d["DecodeParms"]; exists {
		raw = trimPDF(raw)
		if len(raw) > 0 && raw[0] == '[' {
			raw = trimPDF(raw[1:])
			if len(raw) == 0 || raw[len(raw)-1] != ']' {
				return nil, nil, 0, UnsupportedLayout
			}
			raw = trimPDF(raw[:len(raw)-1])
		}
		if string(raw) != "null" {
			params, n, ok := readDictionary(raw)
			if !ok || len(trimPDF(raw[n:])) != 0 {
				return nil, nil, 0, UnsupportedLayout
			}
			var reason string
			data, reason = undoPredictor(data, params)
			if reason != "" {
				return nil, nil, 0, reason
			}
		}
	}
	return d, data, p.pos, ""
}

func undoPredictor(data []byte, d pdfDict) ([]byte, string) {
	predictor := 1
	if _, exists := d["Predictor"]; exists {
		var ok bool
		predictor, ok = d.number("Predictor")
		if !ok {
			return nil, "invalid PDF predictor"
		}
	}
	if predictor == 1 {
		return data, ""
	}
	columns := 1
	if _, exists := d["Columns"]; exists {
		var ok bool
		columns, ok = d.number("Columns")
		if !ok {
			return nil, "invalid PDF predictor"
		}
	}
	for key := range d {
		if key == "Colors" || key == "BitsPerComponent" {
			n, ok := d.number(key)
			expected := 1
			if key == "BitsPerComponent" {
				expected = 8
			}
			if !ok || n != expected {
				return nil, UnsupportedLayout
			}
		}
	}
	if predictor < 10 || predictor > 15 {
		return nil, UnsupportedLayout
	}
	if columns < 1 || columns >= maxDecodedBytes || len(data)%(columns+1) != 0 {
		return nil, "invalid PDF predictor rows"
	}
	out := make([]byte, 0, len(data))
	previous := make([]byte, columns)
	for at := 0; at < len(data); at += columns + 1 {
		filter := data[at]
		if filter > 4 || (predictor != 15 && int(filter) != predictor-10) {
			return nil, "invalid PDF predictor filter"
		}
		row := append([]byte(nil), data[at+1:at+1+columns]...)
		for i := range row {
			left, up, upperLeft := byte(0), previous[i], byte(0)
			if i > 0 {
				left = row[i-1]
				upperLeft = previous[i-1]
			}
			switch filter {
			case 1:
				row[i] += left
			case 2:
				row[i] += up
			case 3:
				row[i] += byte((int(left) + int(up)) / 2)
			case 4:
				row[i] += paeth(left, up, upperLeft)
			}
		}
		out = append(out, row...)
		previous = row
	}
	return out, ""
}
func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	da, db, dc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	if da <= db && da <= dc {
		return a
	}
	if db <= dc {
		return b
	}
	return c
}

var revisionEnding = regexp.MustCompile(`^[\x00\t\n\f\r ]*startxref[\r\n ]+([0-9]+)[\r\n ]+%%EOF(?:[\r\n]|$)`)

func validSectionEnd(data []byte, offset int, latest bool) bool {
	if latest {
		p := catalogLexer{data: data}
		p.space()
		return p.pos == len(data)
	}
	match := revisionEnding.FindSubmatch(data)
	if match == nil {
		return false
	}
	at, ok := integer(match[1])
	return ok && at == offset
}
