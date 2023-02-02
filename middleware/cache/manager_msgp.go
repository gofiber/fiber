package cache

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *item) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zbai uint32
	zbai, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zbai > 0 {
		zbai--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "body":
			z.body, err = dc.ReadBytes(z.body)
			if err != nil {
				return
			}
		case "ctype":
			z.ctype, err = dc.ReadBytes(z.ctype)
			if err != nil {
				return
			}
		case "cencoding":
			z.cencoding, err = dc.ReadBytes(z.cencoding)
			if err != nil {
				return
			}
		case "status":
			z.status, err = dc.ReadInt()
			if err != nil {
				return
			}
		case "exp":
			z.exp, err = dc.ReadUint64()
			if err != nil {
				return
			}
		case "headers":
			var zcmr uint32
			zcmr, err = dc.ReadMapHeader()
			if err != nil {
				return
			}
			if z.headers == nil && zcmr > 0 {
				z.headers = make(map[string][]byte, zcmr)
			} else if len(z.headers) > 0 {
				for key := range z.headers {
					delete(z.headers, key)
				}
			}
			for zcmr > 0 {
				zcmr--
				var zxvk string
				var zbzg []byte
				zxvk, err = dc.ReadString()
				if err != nil {
					return
				}
				zbzg, err = dc.ReadBytes(zbzg)
				if err != nil {
					return
				}
				z.headers[zxvk] = zbzg
			}
		case "heapidx":
			z.heapidx, err = dc.ReadInt()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *item) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 7
	// write "body"
	err = en.Append(0x87, 0xa4, 0x62, 0x6f, 0x64, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.body)
	if err != nil {
		return
	}
	// write "ctype"
	err = en.Append(0xa5, 0x63, 0x74, 0x79, 0x70, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.ctype)
	if err != nil {
		return
	}
	// write "cencoding"
	err = en.Append(0xa9, 0x63, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67)
	if err != nil {
		return err
	}
	err = en.WriteBytes(z.cencoding)
	if err != nil {
		return
	}
	// write "status"
	err = en.Append(0xa6, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.status)
	if err != nil {
		return
	}
	// write "exp"
	err = en.Append(0xa3, 0x65, 0x78, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteUint64(z.exp)
	if err != nil {
		return
	}
	// write "headers"
	err = en.Append(0xa7, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73)
	if err != nil {
		return err
	}
	err = en.WriteMapHeader(uint32(len(z.headers)))
	if err != nil {
		return
	}
	for zxvk, zbzg := range z.headers {
		err = en.WriteString(zxvk)
		if err != nil {
			return
		}
		err = en.WriteBytes(zbzg)
		if err != nil {
			return
		}
	}
	// write "heapidx"
	err = en.Append(0xa7, 0x68, 0x65, 0x61, 0x70, 0x69, 0x64, 0x78)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.heapidx)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *item) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 7
	// string "body"
	o = append(o, 0x87, 0xa4, 0x62, 0x6f, 0x64, 0x79)
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
	o = append(o, 0xa7, 0x68, 0x65, 0x61, 0x64, 0x65, 0x72, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.headers)))
	for zxvk, zbzg := range z.headers {
		o = msgp.AppendString(o, zxvk)
		o = msgp.AppendBytes(o, zbzg)
	}
	// string "heapidx"
	o = append(o, 0xa7, 0x68, 0x65, 0x61, 0x70, 0x69, 0x64, 0x78)
	o = msgp.AppendInt(o, z.heapidx)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *item) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zajw uint32
	zajw, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "body":
			z.body, bts, err = msgp.ReadBytesBytes(bts, z.body)
			if err != nil {
				return
			}
		case "ctype":
			z.ctype, bts, err = msgp.ReadBytesBytes(bts, z.ctype)
			if err != nil {
				return
			}
		case "cencoding":
			z.cencoding, bts, err = msgp.ReadBytesBytes(bts, z.cencoding)
			if err != nil {
				return
			}
		case "status":
			z.status, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		case "exp":
			z.exp, bts, err = msgp.ReadUint64Bytes(bts)
			if err != nil {
				return
			}
		case "headers":
			var zwht uint32
			zwht, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				return
			}
			if z.headers == nil && zwht > 0 {
				z.headers = make(map[string][]byte, zwht)
			} else if len(z.headers) > 0 {
				for key := range z.headers {
					delete(z.headers, key)
				}
			}
			for zwht > 0 {
				var zxvk string
				var zbzg []byte
				zwht--
				zxvk, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					return
				}
				zbzg, bts, err = msgp.ReadBytesBytes(bts, zbzg)
				if err != nil {
					return
				}
				z.headers[zxvk] = zbzg
			}
		case "heapidx":
			z.heapidx, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *item) Msgsize() (s int) {
	s = 1 + 5 + msgp.BytesPrefixSize + len(z.body) + 6 + msgp.BytesPrefixSize + len(z.ctype) + 10 + msgp.BytesPrefixSize + len(z.cencoding) + 7 + msgp.IntSize + 4 + msgp.Uint64Size + 8 + msgp.MapHeaderSize
	if z.headers != nil {
		for zxvk, zbzg := range z.headers {
			_ = zbzg
			s += msgp.StringPrefixSize + len(zxvk) + msgp.BytesPrefixSize + len(zbzg)
		}
	}
	s += 8 + msgp.IntSize
	return
}
