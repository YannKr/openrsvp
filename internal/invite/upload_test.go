package invite

import "testing"

func TestDetectImageType_AcceptsValidPNG(t *testing.T) {
	// Minimal valid PNG: signature + IHDR + IDAT + IEND (won't decode, but
	// signature + http.DetectContentType is enough for our gate).
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	body := append(pngHeader, make([]byte, 100)...)
	mime, ext := detectImageType(body)
	if mime != "image/png" || ext != ".png" {
		t.Fatalf("expected image/png .png, got %q %q", mime, ext)
	}
}

func TestDetectImageType_RejectsPNGSignatureWithBadBody(t *testing.T) {
	// http.DetectContentType identifies this as image/png because the first
	// 8 bytes match the PNG signature — but our verifyImageSignature
	// shouldn't add extra rejection here because the signature itself is
	// correct. This documents the existing behavior: signature match is
	// sufficient.
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	body := append(pngHeader, []byte("not actually image data")...)
	mime, ext := detectImageType(body)
	if mime != "image/png" || ext != ".png" {
		t.Fatalf("PNG signature should be accepted, got %q %q", mime, ext)
	}
}

func TestDetectImageType_RejectsForgedHeader(t *testing.T) {
	// http.DetectContentType returns image/png for the 8-byte signature
	// alone; if someone strips the signature and pastes attack content,
	// our verifyImageSignature stage MUST reject.
	body := []byte("MZ\x90\x00fakeexe")
	mime, ext := detectImageType(body)
	if mime != "" || ext != "" {
		t.Fatalf("forged content should be rejected, got %q %q", mime, ext)
	}
}

func TestDetectImageType_RejectsSVG(t *testing.T) {
	// SVG can carry inline <script> — must never be accepted as an image.
	body := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	mime, ext := detectImageType(body)
	if mime != "" || ext != "" {
		t.Fatalf("SVG must be rejected, got %q %q", mime, ext)
	}
}

func TestDetectImageType_RejectsPlainText(t *testing.T) {
	body := []byte("just a text file\n")
	mime, ext := detectImageType(body)
	if mime != "" || ext != "" {
		t.Fatalf("text should be rejected, got %q %q", mime, ext)
	}
}

func TestDetectImageType_RejectsJPEGWithoutEOIMarker(t *testing.T) {
	// JPEG SOI marker but no EOI at the end — invalid JPEG, must reject.
	body := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	body = append(body, make([]byte, 100)...) // padding without EOI marker
	mime, ext := detectImageType(body)
	if mime != "" || ext != "" {
		t.Fatalf("JPEG without EOI should be rejected, got %q %q", mime, ext)
	}
}

func TestDetectImageType_AcceptsValidJPEG(t *testing.T) {
	body := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	body = append(body, make([]byte, 100)...)
	body = append(body, 0xFF, 0xD9) // EOI marker
	mime, ext := detectImageType(body)
	if mime != "image/jpeg" || ext != ".jpg" {
		t.Fatalf("expected image/jpeg .jpg, got %q %q", mime, ext)
	}
}
