package cache

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/gofiber/fiber/v2/internal/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *item) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "body":
			z.body, err = dc.ReadBytes(z.body)
			if err != nil {
				err = msgp.WrapError(err, "body")
				return
			}
		case "ctype":
			z.ctype, err = dc.ReadBytes(z.ctype)
			if err != nil {
				err = msgp.WrapError(err, "ctype")
				return
			}
		case "cencoding":
			z.cencoding, err = dc.ReadBytes(z.cencoding)
			if err != nil {
				err = msgp.WrapError(err, "cencoding")
				return
			}
		case "status":
			z.status, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "status")
				return
			}
		case "exp":
			z.exp, err = dc.ReadUint64()
			if err != nil {
				err = msgp.WrapError(err, "exp")
				return
			}
		case "headers":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "headers")
				return
			}
			if z.headers == nil {
				z.headers = make(map[string][]byte, zb0002)
			} else if len(z.headers) > 0 {
				for key := range z.headers {
					delete(z.headers, key)
				}
			}
			for zb0002 > 0 {
				zb0002--
				var za0001 string
				var za0002 []byte
				za0001, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "headers")
					return
				}
				za0002, err = dc.ReadBytes(za0002)
				if err != nil {
					err = msgp.WrapError(err, "headers", za0001)
					return
				}
				z.headers[za0001] = za0002
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *item) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 6
	// write "body"
	err = en.Append(0x86, 0xa4, 0x62, 0x6f, 0x64, 0x79)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.body)
	if err != nil {
		err = msgp.WrapError(err, "body")
		return
	}
	// write "ctype"
	err = en.Append(0xa5, 0x63, 0x74, 0x79, 0x70, 0x65)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.ctype)
	if err != nil {
		err = msgp.WrapError(err, "ctype")
		return
	}
	// write "cencoding"
	err = en.Append(0xa9, 0x63, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.cencoding)
	if err != nil {
		err = msgp.WrapError(err, "cencoding")
		return
	}
	// write "status"
	err = en.Append(0xa6, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73)
	if err != nil {
		return
	}
	err = en.WriteInt(z.status)
	if err != nil {
		err = msgp.WrapError(err, "status")
		return
	}
	// write "exp"
	err = en.Append(0xa3, 0x65, 0x78, 0x70)
	if err != nil {
		return
	}
	err = en.WriteUint64(z.exp)
	if err != nil {
		err = msgp.WrapError(err, "exp")
		return
	}
	// write "headers"
	err = en.Append(0xaa, 0x65, 0x32, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73)
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.headers)))
	if err != nil {
		err = msgp.WrapError(err, "headers")
		return
	}
	for za0001, za0002 := range z.headers {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "headers")
			return
		}
		err = en.WriteBytes(za0002)
		if err != nil {
			err = msgp.WrapError(err, "headers", za0001)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *item) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 6
	// string "body"
	o = append(o, 0x86, 0xa4, 0x62, 0x6f, 0x64, 0x79)
	o = msgp.AppendBytes(o, z.body)
	// string "ctype"
	o = append(o, 0xa5, 0x63, 0x74, 0x79, 0x70, 0x65)
	o = msgp.AppendBytes(o, z.ctype)
	// string "cencoding"
	o = append(o, 0xa9, 0x63, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67)
	o = msgp.AppendBytes(o, z.cencoding)
	// string "status"
	o = append(o, 0xa6, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73)
	o = msgp.AppendInt(o, z.status)
	// string "exp"
	o = append(o, 0xa3, 0x65, 0x78, 0x70)
	o = msgp.AppendUint64(o, z.exp)
	// string "headers"
	o = append(o, 0xaa, 0x65, 0x32, 0x65, 0x48, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.headers)))
	for za0001, za0002 := range z.headers {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendBytes(o, za0002)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *item) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "body":
			z.body, bts, err = msgp.ReadBytesBytes(bts, z.body)
			if err != nil {
				err = msgp.WrapError(err, "body")
				return
			}
		case "ctype":
			z.ctype, bts, err = msgp.ReadBytesBytes(bts, z.ctype)
			if err != nil {
				err = msgp.WrapError(err, "ctype")
				return
			}
		case "cencoding":
			z.cencoding, bts, err = msgp.ReadBytesBytes(bts, z.cencoding)
			if err != nil {
				err = msgp.WrapError(err, "cencoding")
				return
			}
		case "status":
			z.status, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "status")
				return
			}
		case "exp":
			z.exp, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "exp")
				return
			}
		case "headers":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "headers")
				return
			}
			if z.headers == nil {
				z.headers = make(map[string][]byte, zb0002)
			} else if len(z.headers) > 0 {
				for key := range z.headers {
					delete(z.headers, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 []byte
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "headers")
					return
				}
				za0002, bts, err = msgp.ReadBytesBytes(bts, za0002)
				if err != nil {
					err = msgp.WrapError(err, "headers", za0001)
					return
				}
				z.headers[za0001] = za0002
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *item) Msgsize() (s int) {
	s = 1 + 5 + msgp.BytesPrefixSize + len(z.body) + 6 + msgp.BytesPrefixSize + len(z.ctype) + 10 + msgp.BytesPrefixSize + len(z.cencoding) + 7 + msgp.IntSize + 4 + msgp.Uint64Size + 11 + msgp.MapHeaderSize
	if z.headers != nil {
		for za0001, za0002 := range z.headers {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.BytesPrefixSize + len(za0002)
		}
	}
	return
}
