package validator

import "bytes"

// Only inspect a direct dictionary's own Type entry. Values are skipped lexically;
// no references, streams, page trees or compressed objects are resolved here.
func hasDirectCatalogType(body []byte) bool {
	p := catalogLexer{data: body}
	catalog, ok := p.dictionary(true, 0)
	if !ok || !catalog {
		return false
	}
	p.space()
	return string(p.word()) == "endobj"
}

type catalogLexer struct {
	data []byte
	pos  int
}

const maxCatalogDepth = 32

func (p *catalogLexer) space() {
	for p.pos < len(p.data) {
		if pdfSpace(p.data[p.pos]) {
			p.pos++
			continue
		}
		if p.data[p.pos] != '%' {
			return
		}
		for p.pos < len(p.data) && p.data[p.pos] != '\r' && p.data[p.pos] != '\n' {
			p.pos++
		}
	}
}
func (p *catalogLexer) take(s string) bool {
	if !bytes.HasPrefix(p.data[p.pos:], []byte(s)) {
		return false
	}
	p.pos += len(s)
	return true
}
func (p *catalogLexer) word() []byte {
	start := p.pos
	for p.pos < len(p.data) && !pdfBoundary(p.data[p.pos]) {
		p.pos++
	}
	return p.data[start:p.pos]
}
func (p *catalogLexer) name() (string, bool) {
	p.space()
	if !p.take("/") {
		return "", false
	}
	raw := p.word()
	decoded := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if ch == '#' {
			if i+2 >= len(raw) {
				return "", false
			}
			high, ok1 := pdfHex(raw[i+1])
			low, ok2 := pdfHex(raw[i+2])
			if !ok1 || !ok2 {
				return "", false
			}
			ch = high<<4 | low
			i += 2
		}
		if ch == 0 {
			return "", false
		}
		decoded = append(decoded, ch)
	}
	return string(decoded), true
}
func (p *catalogLexer) dictionary(root bool, depth int) (bool, bool) {
	p.space()
	if depth > maxCatalogDepth || !p.take("<<") {
		return false, false
	}
	seen := make(map[string]bool)
	catalog := false
	for {
		p.space()
		if p.take(">>") {
			return catalog, true
		}
		key, ok := p.name()
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		if root && key == "Type" {
			value, ok := p.name()
			if !ok || value != "Catalog" {
				return false, false
			}
			catalog = true
		} else if !p.value(depth + 1) {
			return false, false
		}
	}
}
func (p *catalogLexer) value(depth int) bool {
	p.space()
	if depth > maxCatalogDepth || p.pos >= len(p.data) {
		return false
	}
	switch p.data[p.pos] {
	case '/':
		_, ok := p.name()
		return ok
	case '(':
		return p.literal()
	case '<':
		if bytes.HasPrefix(p.data[p.pos:], []byte("<<")) {
			_, ok := p.dictionary(false, depth)
			return ok
		}
		return p.hexString()
	case '[':
		p.pos++
		for {
			p.space()
			if p.take("]") {
				return true
			}
			if !p.value(depth + 1) {
				return false
			}
		}
	}
	token := p.word()
	if string(token) == "true" || string(token) == "false" || string(token) == "null" {
		return true
	}
	if !pdfNumber(token) {
		return false
	}
	// Consume an indirect reference as one value, but never resolve it. Roll back
	// lookahead when adjacent numbers are separate array elements instead.
	afterNumber := p.pos
	if pdfUnsigned(token) {
		p.space()
		second := p.word()
		if pdfUnsigned(second) {
			p.space()
			if string(p.word()) == "R" {
				return true
			}
		}
	}
	p.pos = afterNumber
	return true
}
func (p *catalogLexer) literal() bool {
	p.pos++
	nesting := 1
	for p.pos < len(p.data) {
		ch := p.data[p.pos]
		p.pos++
		switch ch {
		case '\\':
			if p.pos == len(p.data) {
				return false
			}
			escaped := p.data[p.pos]
			p.pos++
			if escaped == '\r' && p.pos < len(p.data) && p.data[p.pos] == '\n' {
				p.pos++
			}
		case '(':
			nesting++
		case ')':
			nesting--
			if nesting == 0 {
				return true
			}
		}
	}
	return false
}
func (p *catalogLexer) hexString() bool {
	p.pos++
	for p.pos < len(p.data) {
		ch := p.data[p.pos]
		p.pos++
		if ch == '>' {
			return true
		}
		if _, ok := pdfHex(ch); !ok && !pdfSpace(ch) {
			return false
		}
	}
	return false
}
func pdfSpace(ch byte) bool {
	return ch == 0 || ch == '\t' || ch == '\n' || ch == '\f' || ch == '\r' || ch == ' '
}
func pdfBoundary(ch byte) bool { return pdfSpace(ch) || bytes.IndexByte([]byte("()<>[]{}/%"), ch) >= 0 }
func pdfHex(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	}
	return 0, false
}
func pdfUnsigned(token []byte) bool {
	if len(token) == 0 {
		return false
	}
	for _, ch := range token {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
func pdfNumber(token []byte) bool {
	if len(token) > 0 && (token[0] == '+' || token[0] == '-') {
		token = token[1:]
	}
	digits, points := 0, 0
	for _, ch := range token {
		if ch == '.' {
			points++
		} else if ch >= '0' && ch <= '9' {
			digits++
		} else {
			return false
		}
	}
	return digits > 0 && points <= 1
}
