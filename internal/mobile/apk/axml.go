// SPDX-License-Identifier: MIT

// Package apk parses Android .apk and split bundles (.xapk, .apks, .apkm)
// completely offline using Go's archive/zip plus pure-Go AXML & ARSC readers.
package apk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

const (
	resStringPoolType   = 0x0001
	resTableType        = 0x0002
	resXMLType          = 0x0003
	resXMLStartNS       = 0x0100
	resXMLEndNS         = 0x0101
	resXMLStartElement  = 0x0102
	resXMLEndElement    = 0x0103
	resXMLResourceMap   = 0x0180
	resTablePackageType = 0x0200
	resTableTypeType    = 0x0201

	typeNull      = 0x00
	typeReference = 0x01
	typeString    = 0x03
	typeIntDec    = 0x10
	typeIntHex    = 0x11
	typeIntBool   = 0x12

	androidNS = "http://schemas.android.com/apk/res/android"
)

// XMLAttr represents one attribute on an Android binary XML element.
type XMLAttr struct {
	Namespace string
	Name      string
	Raw       string
	DataType  uint8
	Data      uint32
	RefID     uint32
	StrValue  string
	IntValue  int32
	HasInt    bool
	BoolValue bool
	HasBool   bool
}

// XMLNode is a decoded element in an Android binary XML document.
type XMLNode struct {
	Name     string
	Attrs    []XMLAttr
	Children []*XMLNode
}

func (n *XMLNode) ChildNodes(name string) []*XMLNode {
	var out []*XMLNode
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// Attr finds an attribute by name. When ns is "", any namespace matches;
// otherwise the attribute namespace must match ns.
func (n *XMLNode) Attr(name, ns string) *XMLAttr {
	for i := range n.Attrs {
		a := &n.Attrs[i]
		if a.Name != name {
			continue
		}
		if ns == "" || a.Namespace == ns {
			return a
		}
	}
	return nil
}

func (n *XMLNode) AndroidAttr(name string) *XMLAttr {
	if a := n.Attr(name, androidNS); a != nil {
		return a
	}
	return n.Attr(name, "")
}

func readStringPool(buf []byte, start int) ([]string, int, error) {
	if start+28 > len(buf) {
		return nil, 0, errors.New("truncated string pool header")
	}
	typ := binary.LittleEndian.Uint16(buf[start:])
	if typ != resStringPoolType {
		return nil, 0, fmt.Errorf("unexpected string pool chunk type 0x%04x", typ)
	}
	headerSize := int(binary.LittleEndian.Uint16(buf[start+2:]))
	chunkSize := int(binary.LittleEndian.Uint32(buf[start+4:]))
	count := int(binary.LittleEndian.Uint32(buf[start+8:]))
	flags := binary.LittleEndian.Uint32(buf[start+16:])
	stringsStart := int(binary.LittleEndian.Uint32(buf[start+20:]))
	if chunkSize <= 0 || start+chunkSize > len(buf) {
		return nil, 0, errors.New("string pool exceeds buffer")
	}
	if count < 0 || start+headerSize+count*4 > start+chunkSize {
		return nil, 0, errors.New("string pool offsets out of range")
	}

	isUTF8 := (flags & 0x100) != 0
	base := start + stringsStart
	limit := start + chunkSize
	out := make([]string, count)

	for i := 0; i < count; i++ {
		off := int(binary.LittleEndian.Uint32(buf[start+headerSize+i*4:]))
		p := base + off
		if p < base || p >= limit {
			continue
		}
		if isUTF8 {
			// Skip UTF-16 char length (1-2 bytes)
			if p >= limit {
				continue
			}
			cLen := int(buf[p])
			p++
			if cLen&0x80 != 0 {
				if p >= limit {
					continue
				}
				p++
			}
			// Read UTF-8 byte length (1-2 bytes)
			if p >= limit {
				continue
			}
			bLen := int(buf[p])
			p++
			if bLen&0x80 != 0 {
				if p >= limit {
					continue
				}
				bLen = ((bLen & 0x7f) << 8) | int(buf[p])
				p++
			}
			if p+bLen > limit {
				bLen = limit - p
			}
			if bLen > 0 {
				out[i] = string(buf[p : p+bLen])
			}
		} else {
			if p+2 > limit {
				continue
			}
			uLen := int(binary.LittleEndian.Uint16(buf[p:]))
			p += 2
			if uLen&0x8000 != 0 {
				if p+2 > limit {
					continue
				}
				uLen = ((uLen & 0x7fff) << 16) | int(binary.LittleEndian.Uint16(buf[p:]))
				p += 2
			}
			byteLen := uLen * 2
			if p+byteLen > limit {
				byteLen = (limit - p) &^ 1
			}
			u16 := make([]uint16, byteLen/2)
			for j := range u16 {
				u16[j] = binary.LittleEndian.Uint16(buf[p+j*2:])
			}
			out[i] = string(utf16.Decode(u16))
		}
	}
	return out, chunkSize, nil
}

// ParseAXML decodes an Android binary XML buffer (e.g. AndroidManifest.xml or
// compiled drawable XML) into a DOM tree.
func ParseAXML(buf []byte) (*XMLNode, error) {
	if len(buf) < 8 {
		return nil, errors.New("binary XML too short")
	}
	if binary.LittleEndian.Uint16(buf[0:]) != resXMLType {
		return nil, errors.New("not an Android binary XML document")
	}
	pos := int(binary.LittleEndian.Uint16(buf[2:]))
	total := int(binary.LittleEndian.Uint32(buf[4:]))
	if total > len(buf) {
		total = len(buf)
	}

	var pool []string
	poolStr := func(idx uint32) string {
		if idx == 0xffffffff || int(idx) >= len(pool) {
			return ""
		}
		return pool[idx]
	}

	doc := &XMLNode{Name: "#document"}
	stack := []*XMLNode{doc}

	for pos+8 <= total {
		typ := binary.LittleEndian.Uint16(buf[pos:])
		chunkSize := int(binary.LittleEndian.Uint32(buf[pos+4:]))
		if chunkSize < 8 || pos+chunkSize > total {
			break
		}

		switch typ {
		case resStringPoolType:
			s, _, err := readStringPool(buf, pos)
			if err != nil {
				return nil, err
			}
			pool = s
		case resXMLStartElement:
			if pos+36 > pos+chunkSize {
				break
			}
			nameIdx := binary.LittleEndian.Uint32(buf[pos+20:])
			attrStart := int(binary.LittleEndian.Uint16(buf[pos+24:]))
			attrSize := int(binary.LittleEndian.Uint16(buf[pos+26:]))
			attrCount := int(binary.LittleEndian.Uint16(buf[pos+28:]))

			node := &XMLNode{Name: poolStr(nameIdx)}
			attrBase := pos + 16 + attrStart
			if attrSize >= 20 {
				for i := 0; i < attrCount; i++ {
					a := attrBase + i*attrSize
					if a+20 > pos+chunkSize {
						break
					}
					nsIdx := binary.LittleEndian.Uint32(buf[a:])
					aNameIdx := binary.LittleEndian.Uint32(buf[a+4:])
					rawIdx := binary.LittleEndian.Uint32(buf[a+8:])
					dataType := buf[a+15]
					data := binary.LittleEndian.Uint32(buf[a+16:])

					attr := XMLAttr{
						Namespace: poolStr(nsIdx),
						Name:      poolStr(aNameIdx),
						Raw:       poolStr(rawIdx),
						DataType:  dataType,
						Data:      data,
					}
					switch dataType {
					case typeReference:
						attr.RefID = data
					case typeString:
						attr.StrValue = poolStr(data)
					case typeIntDec, typeIntHex:
						attr.IntValue = int32(data)
						attr.HasInt = true
					case typeIntBool:
						attr.BoolValue = data != 0
						attr.HasBool = true
					}
					if attr.StrValue == "" && attr.Raw != "" {
						attr.StrValue = attr.Raw
					}
					node.Attrs = append(node.Attrs, attr)
				}
			}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, node)
			stack = append(stack, node)
		case resXMLEndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case resXMLStartNS, resXMLEndNS, resXMLResourceMap:
			// Recognized chunk types not needed for metadata extraction.
		}
		pos += chunkSize
	}
	if len(doc.Children) == 0 {
		return nil, errors.New("binary XML contains no root element")
	}
	return doc.Children[0], nil
}
