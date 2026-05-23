package invite

import (
	"bytes"
	"net/http"
)

// maxUploadSize is the maximum allowed image upload size (2 MB).
const maxUploadSize = 2 << 20

// allowedImageTypes maps MIME types detected via http.DetectContentType
// to their canonical file extensions.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// detectImageType validates the uploaded payload is one of the allowed
// image formats. It performs two checks:
//
//  1. http.DetectContentType reads the first 512 bytes and identifies the
//     MIME type. We reject anything outside the allowlist.
//  2. The format-specific signature is verified against the actual bytes
//     so that polyglot files (e.g. a valid PNG header glued onto a ZIP
//     containing a shell script) cannot slip through with just a forged
//     header. We trust the format only when both checks agree.
//
// Returns ("", "") when validation fails.
func detectImageType(data []byte) (mimeType, ext string) {
	ct := http.DetectContentType(data)
	e, ok := allowedImageTypes[ct]
	if !ok {
		return "", ""
	}
	if !verifyImageSignature(ct, data) {
		return "", ""
	}
	return ct, e
}

// verifyImageSignature checks the magic bytes at the start (and, where
// applicable, end) of the payload against the canonical signature for the
// claimed image format.
//
//	PNG  — 8-byte signature 89 50 4E 47 0D 0A 1A 0A
//	JPEG — starts FF D8 FF and ends FF D9 (end-of-image marker)
//	WebP — 4-byte 'RIFF', then 4-byte length, then 4-byte 'WEBP'
func verifyImageSignature(mime string, data []byte) bool {
	switch mime {
	case "image/png":
		pngSig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
		return len(data) >= len(pngSig) && bytes.Equal(data[:len(pngSig)], pngSig)
	case "image/jpeg":
		if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF {
			return false
		}
		// JPEG must terminate with the End-of-Image marker (FF D9).
		n := len(data)
		return data[n-2] == 0xFF && data[n-1] == 0xD9
	case "image/webp":
		return len(data) >= 12 &&
			bytes.Equal(data[0:4], []byte("RIFF")) &&
			bytes.Equal(data[8:12], []byte("WEBP"))
	}
	return false
}
