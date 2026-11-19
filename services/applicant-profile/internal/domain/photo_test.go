package domain_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
)

// photoFile returns n bytes that start with the given signature (cut if n is smaller).
func photoFile(signature []byte, n int) []byte {
	b := make([]byte, n)
	copy(b, signature)
	return b
}

var (
	jpegSig = []byte{0xff, 0xd8, 0xff}
	pngSig  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	pdfSig  = []byte("%PDF-")
)

func TestNewPhoto(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		content     []byte
		wantErr     error
	}{
		// size boundaries: 0, 1, 1,000,000, 1,000,001
		{name: "0 bytes", contentType: "image/png", content: []byte{}, wantErr: domain.ErrInvalidPhotoSize},
		{name: "nil content", contentType: "image/png", content: nil, wantErr: domain.ErrInvalidPhotoSize},
		// 1 byte passes the size check but cannot hold a full signature.
		{name: "1 byte", contentType: "image/jpeg", content: []byte{0xff}, wantErr: domain.ErrInvalidPhotoContent},
		{name: "smallest JPEG signature (3 bytes)", contentType: "image/jpeg", content: photoFile(jpegSig, 3)},
		{name: "smallest PNG signature (8 bytes)", contentType: "image/png", content: photoFile(pngSig, 8)},
		{name: "PNG cut to 7 bytes", contentType: "image/png", content: photoFile(pngSig, 7), wantErr: domain.ErrInvalidPhotoContent},
		{name: "1,000,000 byte JPEG", contentType: "image/jpeg", content: photoFile(jpegSig, 1_000_000)},
		{name: "1,000,000 byte PNG", contentType: "image/png", content: photoFile(pngSig, 1_000_000)},
		{name: "1,000,001 byte JPEG", contentType: "image/jpeg", content: photoFile(jpegSig, 1_000_001), wantErr: domain.ErrInvalidPhotoSize},
		{name: "size is checked before type", contentType: "image/gif", content: photoFile(jpegSig, 1_000_001), wantErr: domain.ErrInvalidPhotoSize},
		// declared type
		{name: "GIF type", contentType: "image/gif", content: photoFile(jpegSig, 10), wantErr: domain.ErrInvalidPhotoType},
		{name: "image/jpg is not image/jpeg", contentType: "image/jpg", content: photoFile(jpegSig, 10), wantErr: domain.ErrInvalidPhotoType},
		{name: "uppercase type", contentType: "IMAGE/PNG", content: photoFile(pngSig, 10), wantErr: domain.ErrInvalidPhotoType},
		{name: "type with spaces", contentType: " image/png ", content: photoFile(pngSig, 10), wantErr: domain.ErrInvalidPhotoType},
		{name: "empty type", contentType: "", content: photoFile(pngSig, 10), wantErr: domain.ErrInvalidPhotoType},
		// file signature
		{name: "PNG declared, JPEG bytes", contentType: "image/png", content: photoFile(jpegSig, 10), wantErr: domain.ErrInvalidPhotoContent},
		{name: "JPEG declared, PNG bytes", contentType: "image/jpeg", content: photoFile(pngSig, 10), wantErr: domain.ErrInvalidPhotoContent},
		{name: "PDF renamed to PNG", contentType: "image/png", content: photoFile(pdfSig, 10), wantErr: domain.ErrInvalidPhotoContent},
		{name: "signature not at the start", contentType: "image/jpeg", content: append([]byte{0}, jpegSig...), wantErr: domain.ErrInvalidPhotoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewPhoto(tt.contentType, tt.content)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err=%v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ContentType != tt.contentType || !bytes.Equal(got.Content, tt.content) {
				t.Fatalf("got type %q and %d bytes, want %q and %d bytes", got.ContentType, len(got.Content), tt.contentType, len(tt.content))
			}
		})
	}
}
