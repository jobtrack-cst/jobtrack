package validator_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/functions/resume-validator/internal/validator"
	"github.com/stretchr/testify/require"
)

// Rebuild offsets so failures exercise catalog lexing, not damaged xref data.
func pdfWithCatalog(dictionary string) []byte {
	content := bytes.Replace(pdf(0), []byte("<< /Type /Catalog /Pages 2 0 R >>"), []byte(dictionary), 1)
	prefix := content[:bytes.LastIndex(content, []byte("\nxref\n"))+1]
	var out bytes.Buffer
	out.Write(prefix)
	out.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for id := 1; id <= 5; id++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", bytes.Index(prefix, []byte(fmt.Sprintf("%d 0 obj\n", id))))
	}
	fmt.Fprintf(&out, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(prefix))
	return out.Bytes()
}

func TestValidate_CatalogRequiresActualTypeEntry(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"literal string spoof", `<< /Bogus (/Type /Catalog) /Pages 2 0 R >>`, false},
		{"comment spoof", "<< % /Type /Catalog\n/Pages 2 0 R >>", false},
		{"unrelated entry", `<< /Bogus /Type /Catalog /Pages >>`, false},
		{"nested dictionary", `<< /Bogus << /Type /Catalog >> /Pages 2 0 R >>`, false},
		{"valid catalog", `<< /Type /Catalog /Pages 2 0 R >>`, true},
		{"hex string spoof", `<< /Bogus <2f54797065202f436174616c6f67> /Pages 2 0 R >>`, false},
		{"malformed hex raw spoof", `<< /Bogus </Type /Catalog> /Pages 2 0 R >>`, false},
		{"nested array spoof", `<< /Bogus [<< /Type /Catalog >>] /Pages 2 0 R >>`, false},
		{"unrelated name value", `<< /Bogus /Catalog /Pages 2 0 R >>`, false},
		{"wrong actual Type", `<< /Type /Pages /Bogus (/Type /Catalog) >>`, false},
		{"Type string value", `<< /Type (/Catalog) >>`, false},
		{"Type hex value", `<< /Type <436174616c6f67> >>`, false},
		{"Type reference", `<< /Type 2 0 R >>`, false},
		{"Type nested dictionary", `<< /Type << /Type /Catalog >> >>`, false},
		{"Type name prefix", `<< /Type /CatalogExtra >>`, false},
		{"Type key prefix", `<< /TypeExtra /Catalog >>`, false},
		{"escaped name suffix", `<< /Type /Catalog#2fExtra >>`, false},
		{"escaped delimiter in key", `<< /Ty#70e#2fExtra /Catalog >>`, false},
		{"escaped correct names", `<< /Ty#70e /Cat#61log /Pages 2 0 R >>`, true},
		{"delimiter separated names", `<</Type/Catalog/Pages 2 0 R>>`, true},
		{"comments between key and value", "<< /Type % /Bogus /Pages\n /Catalog /Pages 2 0 R >>", true},
		{"literal and hex unrelated values", `<< /Bogus (escaped \( and \) and (nested) /Type /Pages) /Hex <f 0 a> /Type /Catalog /Pages 2 0 R >>`, true},
		{"nested values with true root Type", `<< /Bogus [1 0 -1 +2 .5 1. false true null /Name << /Type /Pages >>] /Type /Catalog >>`, true},
		{"endobj text inside string", `<< /Bogus (endobj /Type /Pages) /Type /Catalog /Pages 2 0 R >>`, true},
		{"endobj text inside comment", "<< % endobj /Type /Pages\n /Type /Catalog /Pages 2 0 R >>", true},
		{"escaped parentheses and octal", `<< /Bogus (\( /Type /Pages \) \057) /Type /Catalog >>`, true},
		{"escaped line continuation", "<< /Bogus (a" + string([]byte{92, 13, 10}) + "b) /Type /Catalog >>", true},
		{"duplicate Type", `<< /Type /Catalog /Type /Catalog >>`, false},
		{"duplicate decoded Type", `<< /Type /Catalog /Ty#70e /Catalog >>`, false},
		{"bad name escape", `<< /Type /Cat#ZZalog >>`, false},
		{"truncated name escape", `<< /Type /Catalog#2 >>`, false},
		{"escaped NUL in name", `<< /Type /Catalog#00 >>`, false},
		{"unclosed string after valid Type", `<< /Type /Catalog /Bogus (unclosed >>`, false},
		{"escaped closing without string terminator", `<< /Type /Catalog /Bogus (unclosed\) >>`, false},
		{"unclosed string", `<< /Bogus ( /Type /Catalog >>`, false},
		{"unclosed nested dictionary", `<< /Bogus << /Type /Catalog >>`, false},
		{"unclosed array", `<< /Bogus [ /Type /Catalog >>`, false},
		{"invalid unrelated hex", `<< /Bogus <zz> /Type /Catalog >>`, false},
		{"dangling key", `<< /Type /Catalog /Bogus >>`, false},
		{"trailing dictionary", `<< /Type /Catalog >> << /Type /Catalog >>`, false},
		{"stray keyword", `<< /Bogus stream /Type /Catalog >>`, false},
		{"excessive nesting", `<< /Bogus ` + strings.Repeat("[", 64) + strings.Repeat("]", 64) + ` /Type /Catalog >>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := validator.Validate("application/pdf", pdfWithCatalog(tc.body))
			require.Equal(t, tc.valid, reason == "", reason)
		})
	}
}
