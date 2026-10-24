package etag

import (
	"encoding/binary"
	"hash/crc32"
)

// crcTable holds the eight lookup tables of slicing-by-8 for one CRC-32
// polynomial: t[0] is the table of hash/crc32, and t[k][b] is the CRC state of
// the byte b followed by k zero bytes. hash/crc32 only runs the IEEE and
// Castagnoli polynomials fast and checksums any other a byte at a time.
type crcTable [8][256]uint32

// newCRCTable builds the tables for poly, given in the reflected form that
// crc32.MakeTable takes. The tables take 8 KiB, so build them once.
func newCRCTable(poly uint32) *crcTable {
	t := new(crcTable)
	t[0] = *crc32.MakeTable(poly)
	for i := range 256 {
		crc := t[0][i]
		for j := 1; j < len(t); j++ {
			crc = t[0][crc&0xff] ^ (crc >> 8)
			t[j][i] = crc
		}
	}
	return t
}

// checksum returns the CRC-32 of p for the polynomial the tables were built
// for, the same value as crc32.Checksum with the table crc32.MakeTable returns
// for that polynomial.
//
// It takes eight bytes per step: the first four are folded into the CRC state,
// and then one lookup per byte, in tables 7 down to 0, stands in for eight
// byte-at-a-time steps. The last 0 to 7 bytes take the byte-at-a-time step.
// As in hash/crc32, the state starts as all ones and the result is its complement.
//
// It does not allocate and only reads t, so it is safe for concurrent use.
func (t *crcTable) checksum(p []byte) uint32 {
	crc := ^uint32(0)
	for len(p) >= 8 {
		crc ^= binary.LittleEndian.Uint32(p)
		crc = t[7][crc&0xff] ^ t[6][crc>>8&0xff] ^ t[5][crc>>16&0xff] ^ t[4][crc>>24] ^
			t[3][p[4]] ^ t[2][p[5]] ^ t[1][p[6]] ^ t[0][p[7]]
		p = p[8:]
	}
	for _, b := range p {
		crc = t[0][(crc^uint32(b))&0xff] ^ (crc >> 8)
	}
	return ^crc
}
