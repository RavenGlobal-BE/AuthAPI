package Image

import (
	"bytes"
	"encoding/binary"
)

func JPEG(data []byte) ([]byte, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil, ErrNotJPEG
	}

	var app0, body bytes.Buffer
	var orientation uint16
	first := true
	i := 2

	for {
		if i >= len(data) || data[i] != 0xFF {
			return nil, ErrCorrupt
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return nil, ErrCorrupt
		}
		m := data[i]
		i++

		// Markers without a length field.
		if m == 0xD9 { // EOI: stop here, dropping any trailing data
			body.Write([]byte{0xFF, 0xD9})
			return assembleJPEG(app0.Bytes(), orientation, body.Bytes()), nil
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD7) {
			body.Write([]byte{0xFF, m})
			continue
		}

		if i+2 > len(data) {
			return nil, ErrCorrupt
		}
		n := int(binary.BigEndian.Uint16(data[i:]))
		if n < 2 || i+n > len(data) {
			return nil, ErrCorrupt
		}
		seg := data[i : i+n] // length field + payload
		payload := seg[2:]
		i += n

		keep := true
		switch {
		case m == 0xE0: // APP0 JFIF/JFXX
			if first {
				app0.Write([]byte{0xFF, m})
				app0.Write(seg)
				keep = false
			}
		case m == 0xE1: // APP1: EXIF or XMP
			if orientation == 0 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
				orientation = readOrientation(payload[6:])
			}
			keep = false
		case m == 0xE2: // APP2: keep ICC profile only (drop MPF, FlashPix, ...)
			keep = bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00"))
		case m == 0xEE: // APP14 Adobe: needed for correct colour transform
			keep = bytes.HasPrefix(payload, []byte("Adobe"))
		case m >= 0xE3 && m <= 0xEF, m == 0xFE: // other APPn, COM
			keep = false
		}
		first = false

		if keep {
			body.Write([]byte{0xFF, m})
			body.Write(seg)
		}

		// Start of Scan: copy the entropy-coded data up to the next real marker.
		// Progressive JPEGs have several scans, so we then continue the loop.
		if m == 0xDA {
			start := i
			for {
				if i+1 >= len(data) {
					// Missing EOI: be lenient, keep the data and close the Image.
					body.Write(data[start:])
					body.Write([]byte{0xFF, 0xD9})
					return assembleJPEG(app0.Bytes(), orientation, body.Bytes()), nil
				}
				if data[i] == 0xFF {
					nx := data[i+1]
					if nx != 0x00 && nx != 0xFF && !(nx >= 0xD0 && nx <= 0xD7) {
						break
					}
				}
				i++
			}
			body.Write(data[start:i])
		}
	}
}

func assembleJPEG(app0 []byte, orientation uint16, body []byte) []byte {
	out := make([]byte, 0, 2+len(app0)+40+len(body))
	out = append(out, 0xFF, 0xD8)
	out = append(out, app0...)
	if orientation > 1 && orientation <= 8 {
		out = append(out, orientationAPP1(orientation)...)
	}
	return append(out, body...)
}

// readOrientation returns the Orientation tag (0x0112) from IFD0 of a TIFF
// structure, or 0 if absent or malformed.
func readOrientation(t []byte) uint16 {
	if len(t) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	off := uint64(bo.Uint32(t[4:]))
	if off < 8 || off+2 > uint64(len(t)) {
		return 0
	}
	count := int(bo.Uint16(t[off:]))
	for k := 0; k < count; k++ {
		e := int(off) + 2 + 12*k
		if e+12 > len(t) {
			return 0
		}
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 { // SHORT
			if v := bo.Uint16(t[e+8:]); v >= 1 && v <= 8 {
				return v
			}
			return 0
		}
	}
	return 0
}

// orientationAPP1 builds a minimal APP1 EXIF segment holding only Orientation.
func orientationAPP1(v uint16) []byte {
	tiff := []byte{
		'M', 'M', 0, 42, // big-endian TIFF header
		0, 0, 0, 8, // IFD0 at offset 8
		0, 1, // 1 entry
		0x01, 0x12, 0, 3, 0, 0, 0, 1, byte(v >> 8), byte(v), 0, 0, // Orientation, SHORT, count 1
		0, 0, 0, 0, // no next IFD
	}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(seg, payload...)
}
