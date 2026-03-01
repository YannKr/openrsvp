package invite

import "net/http"

// maxUploadSize is the maximum allowed image upload size (2 MB).
const maxUploadSize = 2 << 20

// allowedImageTypes maps MIME types detected via http.DetectContentType
// to their canonical file extensions.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// detectImageType reads the first 512 bytes of data and returns the
// detected MIME type and file extension if the content is an allowed
// image type. Returns empty strings for disallowed types.
func detectImageType(data []byte) (mimeType, ext string) {
	ct := http.DetectContentType(data)
	if e, ok := allowedImageTypes[ct]; ok {
		return ct, e
	}
	return "", ""
}
