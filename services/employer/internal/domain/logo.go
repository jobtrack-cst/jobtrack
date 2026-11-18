package domain

import "bytes"

// Logo limits from JT-102, used for boundary value tests. The migration
// 00004_create_company_logos.sql enforces the same rules.
const (
	MaxLogoBytes = 1_000_000
	LogoTypeJPEG = "image/jpeg"
	LogoTypePNG  = "image/png"
)

var (
	jpegSignature = []byte{0xff, 0xd8, 0xff}
	pngSignature  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

// Logo is a validated company logo.
type Logo struct {
	ContentType string
	Content     []byte
}

// NewLogo checks the size, then the declared type, then that the file
// signature matches the type, so a renamed file is refused.
func NewLogo(contentType string, content []byte) (Logo, error) {
	if len(content) < 1 || len(content) > MaxLogoBytes {
		return Logo{}, ErrInvalidLogoSize
	}
	var signature []byte
	switch contentType {
	case LogoTypeJPEG:
		signature = jpegSignature
	case LogoTypePNG:
		signature = pngSignature
	default:
		return Logo{}, ErrInvalidLogoType
	}
	if !bytes.HasPrefix(content, signature) {
		return Logo{}, ErrInvalidLogoContent
	}
	return Logo{ContentType: contentType, Content: content}, nil
}
