// SPDX-License-Identifier: MIT

package apk

import (
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

type resConfig struct {
	density    uint16
	language   string
	country    string
	sdkVersion uint16
	uiMode     uint8
}

type resValue struct {
	dataType uint8
	data     uint32
}

type resTypeChunk struct {
	id      uint8
	config  resConfig
	entries map[uint16]resValue
}

type resPackage struct {
	id    uint32
	types map[uint8][]*resTypeChunk
}

// ResTable is a minimal offline reader for Android resources.arsc tables.
type ResTable struct {
	globalStrings []string
	packages      []resPackage
}

func parseResConfig(buf []byte, off int) resConfig {
	var cfg resConfig
	if off+4 > len(buf) {
		return cfg
	}
	size := int(binary.LittleEndian.Uint32(buf[off:]))
	if off+size > len(buf) {
		size = len(buf) - off
	}
	if size >= 12 {
		cfg.language = strings.Trim(string(buf[off+8:off+10]), "\x00 ")
		cfg.country = strings.Trim(string(buf[off+10:off+12]), "\x00 ")
	}
	if size >= 16 {
		cfg.density = binary.LittleEndian.Uint16(buf[off+14:])
	}
	if size >= 26 {
		cfg.sdkVersion = binary.LittleEndian.Uint16(buf[off+24:])
	}
	if size >= 30 {
		cfg.uiMode = buf[off+29]
	}
	return cfg
}

func scoreConfig(cfg resConfig, prefer string) float64 {
	var s float64
	if prefer == "file" {
		if cfg.density == 0xfffe {
			// anydpi often points to XML adaptive icons; prefer concrete bitmaps if available.
			s -= 500
		} else {
			s += float64(cfg.density)
		}
		if cfg.uiMode != 0 {
			s -= 20
		}
		return s
	}
	if cfg.density != 0 {
		s -= 10
	}
	if cfg.language != "" {
		if cfg.language == "en" {
			s -= 5
		} else {
			s -= 40
		}
	}
	if cfg.sdkVersion != 0 {
		s -= float64(cfg.sdkVersion) / 1000.0
	}
	if cfg.uiMode != 0 {
		s -= 20
	}
	return s
}

// ParseARSC decodes a resources.arsc buffer sufficient to resolve @0x7f string
// labels and drawable file paths.
func ParseARSC(buf []byte) (*ResTable, error) {
	if len(buf) < 12 || binary.LittleEndian.Uint16(buf[0:]) != resTableType {
		return nil, errors.New("not a valid resources.arsc table")
	}
	headerSize := int(binary.LittleEndian.Uint16(buf[2:]))
	total := int(binary.LittleEndian.Uint32(buf[4:]))
	if total > len(buf) {
		total = len(buf)
	}

	rt := &ResTable{}
	pos := headerSize
	for pos+8 <= total {
		typ := binary.LittleEndian.Uint16(buf[pos:])
		chunkSize := int(binary.LittleEndian.Uint32(buf[pos+4:]))
		if chunkSize < 8 || pos+chunkSize > total {
			break
		}
		switch typ {
		case resStringPoolType:
			if len(rt.globalStrings) == 0 {
				s, _, err := readStringPool(buf, pos)
				if err == nil {
					rt.globalStrings = s
				}
			}
		case resTablePackageType:
			if pkg, ok := parseResPackage(buf, pos, chunkSize); ok {
				rt.packages = append(rt.packages, pkg)
			}
		}
		pos += chunkSize
	}
	return rt, nil
}

func parseResPackage(buf []byte, start, size int) (resPackage, bool) {
	if start+284 > start+size {
		return resPackage{}, false
	}
	headerSize := int(binary.LittleEndian.Uint16(buf[start+2:]))
	pkg := resPackage{
		id:    binary.LittleEndian.Uint32(buf[start+8:]),
		types: map[uint8][]*resTypeChunk{},
	}
	pos := start + headerSize
	end := start + size
	for pos+8 <= end {
		typ := binary.LittleEndian.Uint16(buf[pos:])
		chunkSize := int(binary.LittleEndian.Uint32(buf[pos+4:]))
		if chunkSize < 8 || pos+chunkSize > end {
			break
		}
		if typ == resTableTypeType {
			if tc := parseResType(buf, pos, chunkSize); tc != nil {
				pkg.types[tc.id] = append(pkg.types[tc.id], tc)
			}
		}
		pos += chunkSize
	}
	return pkg, true
}

func parseResType(buf []byte, start, size int) *resTypeChunk {
	if start+24 > start+size {
		return nil
	}
	headerSize := int(binary.LittleEndian.Uint16(buf[start+2:]))
	id := buf[start+8]
	flags := buf[start+9]
	entryCount := int(binary.LittleEndian.Uint32(buf[start+12:]))
	entriesStart := int(binary.LittleEndian.Uint32(buf[start+16:]))
	cfg := parseResConfig(buf, start+20)

	offsetsBase := start + headerSize
	entriesBase := start + entriesStart
	end := start + size
	if offsetsBase > end || entriesBase > end {
		return nil
	}

	entries := make(map[uint16]resValue)
	readVal := func(at int) (resValue, bool) {
		if at < entriesBase || at+8 > end {
			return resValue{}, false
		}
		entrySize := int(binary.LittleEndian.Uint16(buf[at:]))
		entryFlags := binary.LittleEndian.Uint16(buf[at+2:])
		if entryFlags&0x0001 != 0 { // FLAG_COMPLEX (bag/map)
			return resValue{}, false
		}
		valAt := at + entrySize
		if valAt+8 > end {
			return resValue{}, false
		}
		return resValue{
			dataType: buf[valAt+3],
			data:     binary.LittleEndian.Uint32(buf[valAt+4:]),
		}, true
	}

	switch {
	case flags&0x01 != 0: // FLAG_SPARSE
		for i := 0; i < entryCount && offsetsBase+i*4+4 <= end; i++ {
			idx := binary.LittleEndian.Uint16(buf[offsetsBase+i*4:])
			off := int(binary.LittleEndian.Uint16(buf[offsetsBase+i*4+2:])) * 4
			if v, ok := readVal(entriesBase + off); ok {
				entries[idx] = v
			}
		}
	case flags&0x02 != 0: // FLAG_OFFSET16
		for i := 0; i < entryCount && offsetsBase+i*2+2 <= end; i++ {
			off := binary.LittleEndian.Uint16(buf[offsetsBase+i*2:])
			if off == 0xffff {
				continue
			}
			if v, ok := readVal(entriesBase + int(off)*4); ok {
				entries[uint16(i)] = v
			}
		}
	default:
		for i := 0; i < entryCount && offsetsBase+i*4+4 <= end; i++ {
			off := binary.LittleEndian.Uint32(buf[offsetsBase+i*4:])
			if off == 0xffffffff {
				continue
			}
			if v, ok := readVal(entriesBase + int(off)); ok {
				entries[uint16(i)] = v
			}
		}
	}
	return &resTypeChunk{id: id, config: cfg, entries: entries}
}

// ResolveString resolves a @0xPPTTEEEE resource ID into a string value.
// prefer is "string" (for labels) or "file" (for drawables/icons).
func (rt *ResTable) ResolveString(ref uint32, prefer string) string {
	return rt.resolve(ref, prefer, 0)
}

func (rt *ResTable) resolve(ref uint32, prefer string, depth int) string {
	if rt == nil || ref == 0 || depth > 6 {
		return ""
	}
	pkgID := (ref >> 24) & 0xff
	typeID := uint8((ref >> 16) & 0xff)
	entryIdx := uint16(ref & 0xffff)

	var pkg *resPackage
	for i := range rt.packages {
		if rt.packages[i].id == pkgID {
			pkg = &rt.packages[i]
			break
		}
	}
	if pkg == nil && len(rt.packages) > 0 {
		pkg = &rt.packages[0]
	}
	if pkg == nil {
		return ""
	}
	chunks := pkg.types[typeID]
	if len(chunks) == 0 {
		return ""
	}

	var best resValue
	found := false
	bestScore := math.Inf(-1)
	for _, c := range chunks {
		v, ok := c.entries[entryIdx]
		if !ok {
			continue
		}
		sc := scoreConfig(c.config, prefer)
		if !found || sc > bestScore {
			bestScore = sc
			best = v
			found = true
		}
	}
	if !found {
		return ""
	}
	switch best.dataType {
	case typeString:
		if int(best.data) < len(rt.globalStrings) {
			return rt.globalStrings[best.data]
		}
	case typeReference:
		return rt.resolve(best.data, prefer, depth+1)
	}
	return ""
}
