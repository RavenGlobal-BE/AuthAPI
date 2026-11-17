package Image

import (
	"bytes"
	"encoding/binary"
)

var pngSig = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}

// Ancillary chunks that affect how the Image renders. All other ancillary
// chunks (eXIf, tEXt, zTXt, iTXt, tIME, C2PA, vendor-private chunks...) are dropped.
var pngKeep = map[string]bool{
	"tRNS": true, "gAMA": true, "cHRM": true, "sRGB": true, "iCCP": true,
	"sBIT": true, "bKGD": true, "pHYs": true, "cICP": true, "mDCv": true,
	"cLLi": true,
	"acTL": true, "fcTL": true, "fdAT": true, // APNG animation
}

// PNG strips metadata from a PNG. Critical chunks (IHDR, PLTE, IDAT, IEND) and
// rendering-related ancillary chunks are kept; data after IEND is dropped.
func PNG(data []byte) ([]byte, error) {
	if !bytes.HasPrefix(data, pngSig) {
		return nil, ErrNotPNG
	}
	out := bytes.NewBuffer(make([]byte, 0, len(data)))
	out.Write(pngSig)
	i := len(pngSig)

	for i+12 <= len(data) {
		n := uint64(binary.BigEndian.Uint32(data[i:]))
		typ := data[i+4 : i+8]
		end := uint64(i) + 12 + n // length + type + data + CRC
		if end > uint64(len(data)) {
			return nil, ErrCorrupt
		}
		critical := typ[0] >= 'A' && typ[0] <= 'Z'
		if critical || pngKeep[string(typ)] {
			out.Write(data[i:end])
		}
		i = int(end)
		if string(typ) == "IEND" {
			return out.Bytes(), nil
		}
	}
	return nil, ErrCorrupt
}
