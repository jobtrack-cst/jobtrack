package domain

import "bytes"

// Photo limits from JT-103, the same as the company logo (JT-102). The
// migration 00003_create_profile_photos.sql enforces the same rules.
const (
	MaxPhotoBytes = 1_000_000
	PhotoTypeJPEG = "image/jpeg"
	PhotoTypePNG  = "image/png"
)

var (
	jpegSignature = []byte{0xff, 0xd8, 0xff}
	pngSignature  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
)

// Photo is a validated profile photo.
type Photo struct {
	ContentType string
	Content     []byte
}

// NewPhoto checks the size, then the declared type, then that the file
// signature matches the type, so a renamed file is refused.
func NewPhoto(contentType string, content []byte) (Photo, error) {
	if len(content) < 1 || len(content) > MaxPhotoBytes {
		return Photo{}, ErrInvalidPhotoSize
	}
	var signature []byte
	switch contentType {
	case PhotoTypeJPEG:
		signature = jpegSignature
	case PhotoTypePNG:
		signature = pngSignature
	default:
		return Photo{}, ErrInvalidPhotoType
	}
	if !bytes.HasPrefix(content, signature) {
		return Photo{}, ErrInvalidPhotoContent
	}
	return Photo{ContentType: contentType, Content: content}, nil
}
