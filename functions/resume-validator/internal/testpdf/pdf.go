// Package testpdf constructs a synthetic one-page PDF for deployment tests.
package testpdf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
)

// Document returns a PDF with a correct classic xref table. If size exceeds the
// base document length, legal whitespace before the xref table pads it to size.
func Document(size int) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, body := range pageObjects() {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	prefix := append([]byte(nil), b.Bytes()...)
	suffix := func(offset int) []byte {
		var out bytes.Buffer
		fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
		for _, at := range offsets[1:] {
			fmt.Fprintf(&out, "%010d 00000 n \n", at)
		}
		fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), offset)
		return out.Bytes()
	}
	padding := 0
	for i := 0; i < 8; i++ {
		need := size - len(prefix) - len(suffix(len(prefix)+padding))
		if need < 0 {
			need = 0
		}
		if need == padding {
			break
		}
		padding = need
	}
	result := append(prefix, bytes.Repeat([]byte(" "), padding)...)
	return append(result, suffix(len(result))...)
}

// BareXref is the final-review counterexample. It has signature tokens but no
// xref entries, trailer dictionary, or valid catalog object.
func BareXref() []byte {
	return []byte("%PDF-1.4\n1 0 obj\n/Type /Catalog\nendobj\nxref\nstartxref\n39\n%%EOF\n")
}

func pageObjects() []string {
	stream := "BT /F1 12 Tf 72 720 Td (JT-62 synthetic resume) Tj ET\n"
	return []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}
}

// CompressedCatalog is a PDF 1.5 specimen with catalog 1 inside FlateDecode
// object stream 6 and a type-2 entry in cross-reference stream 7.
func CompressedCatalog() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	offsets := make([]int, 8)
	objects := pageObjects()
	for id := 2; id <= 5; id++ {
		offsets[id] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", id, objects[id-1])
	}
	var compressed bytes.Buffer
	encoder := zlib.NewWriter(&compressed)
	_, _ = encoder.Write(append([]byte("1 0 "+objects[0]), bytes.Repeat([]byte(" "), 1024)...))
	_ = encoder.Close()
	offsets[6] = b.Len()
	fmt.Fprintf(&b, "6 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length %d /Filter /FlateDecode >>\nstream\n", compressed.Len())
	b.Write(compressed.Bytes())
	b.WriteString("\nendstream\nendobj\n")
	offsets[7] = b.Len()
	entries := make([]byte, 8*7)
	entries[0] = 0
	binary.BigEndian.PutUint16(entries[5:7], 65535)
	for id := 1; id < 8; id++ {
		entry := entries[id*7 : (id+1)*7]
		if id == 1 {
			entry[0] = 2
			binary.BigEndian.PutUint32(entry[1:5], 6)
		} else {
			entry[0] = 1
			binary.BigEndian.PutUint32(entry[1:5], uint32(offsets[id]))
		}
	}
	fmt.Fprintf(&b, "7 0 obj\n<< /Type /XRef /Size 8 /Root 1 0 R /W [1 4 2] /Length %d >>\nstream\n", len(entries))
	b.Write(entries)
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", offsets[7])
	return b.Bytes()
}

// XrefStream builds a PDF 1.7 with a direct catalog and a cross-reference
// stream. Flate and PNG Up prediction exercise common producer encodings.
func XrefStream(flate, predictor bool) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := []int{0}
	for i, body := range pageObjects() {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	offsets = append(offsets, b.Len())
	entries := make([]byte, len(offsets)*7)
	binary.BigEndian.PutUint16(entries[5:7], 65535)
	for id := 1; id < len(offsets); id++ {
		entry := entries[id*7 : (id+1)*7]
		entry[0] = 1
		binary.BigEndian.PutUint32(entry[1:5], uint32(offsets[id]))
	}
	options := ""
	if predictor {
		var encoded []byte
		previous := make([]byte, 7)
		for at := 0; at < len(entries); at += 7 {
			encoded = append(encoded, 2)
			for i := 0; i < 7; i++ {
				encoded = append(encoded, entries[at+i]-previous[i])
			}
			copy(previous, entries[at:at+7])
		}
		entries = encoded
		options += " /DecodeParms << /Predictor 12 /Columns 7 >>"
	}
	if flate {
		var compressed bytes.Buffer
		w := zlib.NewWriter(&compressed)
		_, _ = w.Write(entries)
		_ = w.Close()
		entries = compressed.Bytes()
		options += " /Filter /FlateDecode"
	}
	fmt.Fprintf(&b, "6 0 obj\n<< /Type /XRef /Size 7 /Root 1 0 R /W [1 4 2] /Length %d%s >>\nstream\n", len(entries), options)
	b.Write(entries)
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", offsets[6])
	return b.Bytes()
}

// Incremental appends a replacement catalog, leaving the original EOF intact.
// The newest section intentionally omits unchanged objects, requiring /Prev.
func Incremental(base []byte, stream bool) []byte {
	previous := bytes.LastIndex(base, []byte("startxref\n")) + len("startxref\n")
	var prev int
	_, _ = fmt.Sscanf(string(base[previous:]), "%d", &prev)
	var size int
	sizeAt := bytes.LastIndex(base[:previous], []byte("/Size "))
	_, _ = fmt.Sscanf(string(base[sizeAt:]), "/Size %d", &size)
	var b bytes.Buffer
	b.Write(base)
	rootOffset := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R /Lang (en) >>\nendobj\n")
	at := b.Len()
	if stream {
		entries := make([]byte, 14)
		entries[0] = 1
		entries[7] = 1
		binary.BigEndian.PutUint32(entries[1:5], uint32(rootOffset))
		binary.BigEndian.PutUint32(entries[8:12], uint32(at))
		fmt.Fprintf(&b, "8 0 obj\n<< /Type /XRef /Size 9 /Root 1 0 R /Prev %d /Index [1 1 8 1] /W [1 4 2] /Length 14 >>\nstream\n", prev)
		b.Write(entries)
		b.WriteString("\nendstream\nendobj\n")
	} else {
		fmt.Fprintf(&b, "xref\n1 1\n%010d 00000 n \ntrailer\n<< /Size %d /Root 1 0 R /Prev %d >>\n", rootOffset, size, prev)
	}
	fmt.Fprintf(&b, "startxref\n%d\n%%%%EOF\n", at)
	return b.Bytes()
}

// Hybrid supplies compressed catalog entries through /XRefStm in the trailer.
func Hybrid() []byte {
	base := CompressedCatalog()
	at := bytes.LastIndex(base, []byte("startxref\n"))
	var streamOffset int
	_, _ = fmt.Sscanf(string(base[at+len("startxref\n"):]), "%d", &streamOffset)
	var b bytes.Buffer
	b.Write(base[:at])
	xref := b.Len()
	b.WriteString("xref\n0 1\n0000000000 65535 f \n2 6\n")
	for id := 2; id <= 7; id++ {
		offset := bytes.Index(base, []byte(fmt.Sprintf("%d 0 obj\n", id)))
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size 8 /Root 1 0 R /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n", streamOffset, xref)
	return b.Bytes()
}
