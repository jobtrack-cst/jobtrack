package validator_test

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"testing"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/testpdf"
	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	"github.com/stretchr/testify/require"
)

func TestValidate_CrossReferenceLayouts(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"classic", pdf(0)},
		{"hybrid", testpdf.Hybrid()},
		{"raw stream", testpdf.XrefStream(false, false)},
		{"Flate stream", testpdf.XrefStream(true, false)},
		{"PNG predictor stream", testpdf.XrefStream(true, true)},
		{"compressed catalog", testpdf.CompressedCatalog()},
		{"classic incremental", testpdf.Incremental(pdf(0), false)},
		{"stream incremental", testpdf.Incremental(testpdf.XrefStream(true, true), true)},
		{"classic after stream", testpdf.Incremental(testpdf.CompressedCatalog(), false)},
		{"stream after classic", testpdf.Incremental(pdf(0), true)},
		{"three revisions", testpdf.Incremental(testpdf.Incremental(pdf(0), true), false)},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Empty(t, validator.Validate("application/pdf", tc.data)) })
	}
}

// Match UAT-AP-07's format and byte size without using its personal document.
func TestValidate_AcceptsUATAP07SyntheticShape(t *testing.T) {
	overhead := len(testpdf.Incremental(pdf(490_000), true)) - 490_000
	data := testpdf.Incremental(pdf(490_694-overhead), true)
	data = bytes.Replace(data, []byte("%PDF-1.4"), []byte("%PDF-1.7"), 1)
	require.Len(t, data, 490_694)
	require.Equal(t, 2, bytes.Count(data, []byte("%%EOF")))
	require.Contains(t, string(data), "/Type /XRef")
	require.Empty(t, validator.Validate("application/pdf", data))
}

func TestValidate_RejectsMalformedCrossReferences(t *testing.T) {
	stream := testpdf.XrefStream(false, false)
	update := testpdf.Incremental(pdf(0), false)
	replace := func(data []byte, from, to string) []byte {
		require.Contains(t, string(data), from)
		return bytes.Replace(data, []byte(from), []byte(to), 1)
	}
	lastReplace := func(data []byte, from, to string) []byte {
		at := bytes.LastIndex(data, []byte(from))
		require.GreaterOrEqual(t, at, 0)
		out := append(bytes.Clone(data[:at]), []byte(to)...)
		return append(out, data[at+len(from):]...)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"stream truncated", stream[:len(stream)-20]},
		{"stream bytes removed", lastReplace(stream, "\x00\nendstream", "\nendstream")},
		{"bad metadata object generation", lastReplace(stream, "6 0 obj", "6 1 obj")},
		{"overflow metadata object number", lastReplace(stream, "6 0 obj", "999999999999999999999999 0 obj")},
		{"wrong stream type", replace(stream, "/Type /XRef", "/Type /Fake")},
		{"spoofed stream type", replace(stream, "/Type /XRef", "/Bogus (/Type /XRef)")},
		{"missing stream terminator", lastReplace(stream, "endstream", "badstream")},
		{"missing object terminator", lastReplace(stream, "endstream\nendobj", "endstream\nbadobj")},
		{"short declared length", replace(stream, "/Length 49", "/Length 48")},
		{"long declared length", replace(stream, "/Length 49", "/Length 50")},
		{"overflow length", replace(stream, "/Length 49", "/Length 999999999999999999999999")},
		{"overlapping index", replace(stream, "/W [1 4 2]", "/W [1 4 2] /Index [0 4 3 3]")},
		{"odd index", replace(stream, "/W [1 4 2]", "/W [1 4 2] /Index [0 7 1]")},
		{"extra data after xref object", lastReplace(stream, "endobj\nstartxref", "endobj\nGARBAGE\nstartxref")},
		{"extra data after trailer", lastReplace(update, ">>\nstartxref", ">>\nGARBAGE\nstartxref")},
		{"missing widths", replace(stream, "/W [1 4 2]", "/Bogus [1 4 2]")},
		{"zero widths", replace(stream, "/W [1 4 2]", "/W [0 0 0]")},
		{"overflow widths", replace(stream, "/W [1 4 2]", "/W [1 9999999999999999999999 2]")},
		{"oversized width", replace(stream, "/W [1 4 2]", "/W [1 9 2]")},
		{"excessive size", replace(stream, "/Size 7", "/Size 999999999")},
		{"short size", replace(stream, "/Size 7", "/Size 6")},
		{"wrong root generation", replace(stream, "/Root 1 0 R", "/Root 1 1 R")},
		{"root not catalog", replace(stream, "/Root 1 0 R", "/Root 2 0 R")},
		{"duplicate root", replace(stream, "/Root 1 0 R", "/Root 1 0 R /Root 2 0 R")},
		{"unknown filter", replace(stream, "/W [1 4 2]", "/W [1 4 2] /Filter /Unknown")},
		{"external stream", replace(stream, "/W [1 4 2]", "/W [1 4 2] /F (elsewhere)")},
		{"truncated incremental", update[:len(update)-10]},
		{"invalid Prev", replace(update, "/Prev ", "/Prev 999999")},
		{"Prev not integer", replace(update, "/Prev ", "/Prev (bogus) /Bogus ")},
		{"broken older xref", replace(update, "xref\n0 6", "xref\n0 9")},
		{"missing object stream", replace(testpdf.CompressedCatalog(), "/Type /ObjStm", "/Type /Unknown")},
		{"bad object stream count", replace(testpdf.CompressedCatalog(), "/N 1", "/N 999999")},
		{"bad object stream First", replace(testpdf.CompressedCatalog(), "/First 4", "/First 999999")},
		{"bad compressed data", replace(testpdf.CompressedCatalog(), "/Filter /FlateDecode", "/Filter /FlateDecode /DecodeParms << /Predictor 12 /Columns 99999999 >>")},
	} {
		t.Run(tc.name, func(t *testing.T) { require.NotEmpty(t, validator.Validate("application/pdf", tc.data)) })
	}
	// Tamper binary entries without changing file offsets or stream lengths.
	start := bytes.LastIndex(stream, []byte("\nstream\n")) + len("\nstream\n")
	for _, tc := range []struct {
		name  string
		at    int
		value byte
	}{
		{"unknown entry type", start + 7, 3},
		{"forged object offset", start + 7 + 4, 10},
		{"missing compressed container", start + 7, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := bytes.Clone(stream)
			data[tc.at] = tc.value
			require.NotEmpty(t, validator.Validate("application/pdf", data))
		})
	}
	// A /Prev self-loop must terminate and reject, never recurse indefinitely.
	at := bytes.LastIndex(update, []byte("\nxref\n")) + 1
	prev := bytes.Index(update, []byte("/Prev "))
	data := append(bytes.Clone(update[:prev]), []byte(fmt.Sprintf("/Prev %d >>\nstartxref\n%d\n%%%%EOF\n", at, at))...)
	require.NotEmpty(t, validator.Validate("application/pdf", data))
}

func TestValidate_RejectsUnboundedMetadata(t *testing.T) {
	var compressed bytes.Buffer
	w := zlib.NewWriter(&compressed)
	_, err := w.Write(bytes.Repeat([]byte(" "), 16_000_001))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n")
	fmt.Fprintf(&out, "1 0 obj\n<< /Type /XRef /Size 1 /W [1 4 2] /Length %d /Filter /FlateDecode >>\nstream\n", compressed.Len())
	out.Write(compressed.Bytes())
	out.WriteString("\nendstream\nendobj\nstartxref\n9\n%%EOF\n")
	require.NotEmpty(t, validator.Validate("application/pdf", out.Bytes()))
	data := pdf(0)
	for i := 0; i < 129; i++ {
		data = testpdf.Incremental(data, false)
	}
	require.NotEmpty(t, validator.Validate("application/pdf", data))
}

func TestValidate_RejectsFreedCatalogWithoutFallingBack(t *testing.T) {
	data := testpdf.Incremental(pdf(0), false)
	at := bytes.LastIndex(data, []byte("\nxref\n")) + len("\nxref\n1 1\n")
	entry := bytes.Clone(data[at : at+20])
	entry[17] = 'f'
	copy(data[at:at+20], entry)
	require.NotEmpty(t, validator.Validate("application/pdf", data))
}

func FuzzValidate_PDFMetadata(f *testing.F) {
	for _, data := range [][]byte{pdf(0), testpdf.BareXref(), testpdf.XrefStream(false, false), testpdf.XrefStream(true, true), testpdf.CompressedCatalog(), testpdf.Hybrid(), testpdf.Incremental(pdf(0), true)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { validator.Validate("application/pdf", data) })
}
