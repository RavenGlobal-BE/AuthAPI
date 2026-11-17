package Image

import (
	"bytes"
	"encoding/binary"
)

// HEIC strips EXIF and XMP from a HEIC/HEIF/AVIF file.
//
// In these files, metadata is stored as separate "items" (type "Exif", or
// "mime" with an XMP content type). Their bytes are located through the iloc
// box, usually inside mdat. Removing them would shift every offset in the file,
// so instead the metadata bytes are overwritten in place: the EXIF item becomes
// a valid but empty EXIF block, and XMP is zeroed. The file structure and
// Image data are untouched.
//
// Rotation/mirroring in HEIC is stored in the irot/imir boxes, not EXIF,
// so orientation is preserved automatically.
func HEIC(data []byte) ([]byte, error) {
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		return nil, ErrNotHEIC
	}
	out := append([]byte(nil), data...)

	meta, ok, err := findBox(out, 0, len(out), "meta")
	if err != nil || !ok {
		return nil, ErrCorrupt
	}
	ms := meta.dataStart + 4 // meta is a FullBox (version + flags)
	if ms > meta.end {
		return nil, ErrCorrupt
	}

	iinf, ok, err := findBox(out, ms, meta.end, "iinf")
	if err != nil {
		return nil, ErrCorrupt
	}
	if !ok {
		return out, nil // no item info: no metadata items
	}
	targets, err := metadataItems(out, iinf)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return out, nil
	}

	iloc, ok, err := findBox(out, ms, meta.end, "iloc")
	if err != nil || !ok {
		return nil, ErrCorrupt
	}
	idat, hasIdat, err := findBox(out, ms, meta.end, "idat")
	if err != nil {
		return nil, ErrCorrupt
	}
	extents, err := itemExtents(out, iloc, targets)
	if err != nil {
		return nil, err
	}

	firstExif := map[uint32]bool{}
	for _, e := range extents {
		var lo, hi uint64 // allowed range for this construction method
		var base uint64
		switch e.method {
		case 0: // absolute file offset
			lo, hi, base = 0, uint64(len(out)), 0
		case 1: // offset into idat
			if !hasIdat {
				return nil, ErrCorrupt
			}
			lo, hi, base = uint64(idat.dataStart), uint64(idat.end), uint64(idat.dataStart)
		default: // item-reference construction: data lives in another item
			continue
		}
		start := base + e.offset
		length := e.length
		if length == 0 { // 0 means "to the end"
			if start > hi {
				return nil, ErrCorrupt
			}
			length = hi - start
		}
		if start < lo || start > hi || length > hi-start {
			return nil, ErrCorrupt
		}
		region := out[start : start+length]
		clear(region)

		if targets[e.item] == itemExif && !firstExif[e.item] {
			firstExif[e.item] = true
			if len(region) >= len(emptyHEICExif) {
				copy(region, emptyHEICExif)
			}
		}
	}
	return out, nil
}

// Empty EXIF payload as stored in HEIC: 4-byte offset to the TIFF header (0),
// then a big-endian TIFF header with an empty IFD0.
var emptyHEICExif = []byte{
	0, 0, 0, 0,
	'M', 'M', 0, 42, 0, 0, 0, 8,
	0, 0, // 0 entries
	0, 0, 0, 0, // no next IFD
}

type itemKind int

const (
	itemExif itemKind = iota + 1
	itemXMP
)

type box struct {
	typ            string
	dataStart, end int
}

func readBox(b []byte, pos, limit int) (box, error) {
	if pos+8 > limit {
		return box{}, ErrCorrupt
	}
	size := uint64(binary.BigEndian.Uint32(b[pos:]))
	typ := string(b[pos+4 : pos+8])
	hdr := 8
	switch size {
	case 1:
		if pos+16 > limit {
			return box{}, ErrCorrupt
		}
		size = binary.BigEndian.Uint64(b[pos+8:])
		hdr = 16
	case 0:
		size = uint64(limit - pos)
	}
	if size < uint64(hdr) || size > uint64(limit-pos) {
		return box{}, ErrCorrupt
	}
	return box{typ: typ, dataStart: pos + hdr, end: pos + int(size)}, nil
}

func findBox(b []byte, pos, limit int, typ string) (box, bool, error) {
	for pos < limit {
		bx, err := readBox(b, pos, limit)
		if err != nil {
			return box{}, false, err
		}
		if bx.typ == typ {
			return bx, true, nil
		}
		pos = bx.end
	}
	return box{}, false, nil
}

// metadataItems returns the IDs of EXIF and XMP items listed in iinf.
func metadataItems(b []byte, iinf box) (map[uint32]itemKind, error) {
	r := &reader{b: b, p: iinf.dataStart, end: iinf.end}
	ver := r.u8()
	r.skip(3)
	if ver == 0 {
		r.u16()
	} else {
		r.u32()
	}
	if r.bad {
		return nil, ErrCorrupt
	}

	items := map[uint32]itemKind{}
	for p := r.p; p < iinf.end; {
		bx, err := readBox(b, p, iinf.end)
		if err != nil {
			return nil, ErrCorrupt
		}
		p = bx.end
		if bx.typ != "infe" {
			continue
		}
		e := &reader{b: b, p: bx.dataStart, end: bx.end}
		v := e.u8()
		e.skip(3)
		if v < 2 { // versions 0/1 have no item_type
			continue
		}
		var id uint32
		if v == 2 {
			id = uint32(e.u16())
		} else {
			id = e.u32()
		}
		e.u16() // item_protection_index
		typ := string(e.bytes(4))
		if e.bad {
			return nil, ErrCorrupt
		}
		switch typ {
		case "Exif":
			items[id] = itemExif
		case "mime":
			rest := b[e.p:bx.end]
			// item_name (null-terminated) comes first, then content_type.
			z := bytes.IndexByte(rest, 0)
			if z < 0 {
				continue
			}
			rest = rest[z+1:]
			if z := bytes.IndexByte(rest, 0); z >= 0 {
				rest = rest[:z]
			}
			ct := bytes.ToLower(rest)
			if bytes.Contains(ct, []byte("rdf+xml")) || bytes.Contains(ct, []byte("xmp")) {
				items[id] = itemXMP
			}
		}
	}
	return items, nil
}

type extent struct {
	item           uint32
	method         int
	offset, length uint64
}

// itemExtents parses iloc and returns the byte ranges of the wanted items.
func itemExtents(b []byte, iloc box, want map[uint32]itemKind) ([]extent, error) {
	r := &reader{b: b, p: iloc.dataStart, end: iloc.end}
	ver := r.u8()
	r.skip(3)
	if ver > 2 {
		return nil, ErrCorrupt
	}
	x := r.u8()
	offSz, lenSz := int(x>>4), int(x&0x0F)
	y := r.u8()
	baseSz, idxSz := int(y>>4), 0
	if ver == 1 || ver == 2 {
		idxSz = int(y & 0x0F)
	}
	var count uint32
	if ver < 2 {
		count = uint32(r.u16())
	} else {
		count = r.u32()
	}

	var out []extent
	for k := uint32(0); k < count && !r.bad; k++ {
		var id uint32
		if ver < 2 {
			id = uint32(r.u16())
		} else {
			id = r.u32()
		}
		method := 0
		if ver == 1 || ver == 2 {
			method = int(r.u16() & 0x0F)
		}
		r.u16() // data_reference_index
		base := r.uintN(baseSz)
		n := r.u16()
		for e := uint16(0); e < n && !r.bad; e++ {
			if idxSz > 0 {
				r.uintN(idxSz)
			}
			off := r.uintN(offSz)
			ln := r.uintN(lenSz)
			if _, ok := want[id]; ok {
				if base+off < base { // overflow
					return nil, ErrCorrupt
				}
				out = append(out, extent{item: id, method: method, offset: base + off, length: ln})
			}
		}
	}
	if r.bad {
		return nil, ErrCorrupt
	}
	return out, nil
}

// reader is a bounds-checked big-endian reader; on overrun it sets bad.
type reader struct {
	b      []byte
	p, end int
	bad    bool
}

func (r *reader) bytes(n int) []byte {
	if r.bad || n < 0 || r.p+n > r.end {
		r.bad = true
		return make([]byte, n)
	}
	s := r.b[r.p : r.p+n]
	r.p += n
	return s
}
func (r *reader) skip(n int)  { r.bytes(n) }
func (r *reader) u8() uint8   { return r.bytes(1)[0] }
func (r *reader) u16() uint16 { return binary.BigEndian.Uint16(r.bytes(2)) }
func (r *reader) u32() uint32 { return binary.BigEndian.Uint32(r.bytes(4)) }
func (r *reader) uintN(size int) uint64 {
	switch size {
	case 0:
		return 0
	case 4:
		return uint64(r.u32())
	case 8:
		return binary.BigEndian.Uint64(r.bytes(8))
	default:
		r.bad = true
		return 0
	}
}
