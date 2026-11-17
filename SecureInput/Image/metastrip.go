// Package metastrip removes privacy-sensitive metadata (EXIF incl. GPS, XMP,
// IPTC, comments, text chunks) from JPEG, PNG and HEIC images without
// re-encoding the pixels, so there is no quality loss.
//
// The functions never modify the input slice; they return a new one.
package Image

import "errors"

var (
	ErrNotJPEG = errors.New("metastrip: not a JPEG file")
	ErrNotPNG  = errors.New("metastrip: not a PNG file")
	ErrNotHEIC = errors.New("metastrip: not a HEIC/HEIF file")
	ErrCorrupt = errors.New("metastrip: corrupt or truncated file")
)
