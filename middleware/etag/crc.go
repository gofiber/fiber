package etag

import (
	"encoding/binary"
	"hash/crc32"
)

// crcTable holds the tables of slicing-by-8 for one CRC-32 polynomial.
// hash/crc32 only runs the IEEE and Castagnoli polynomials fast and
// checksums any other a byte at a time.
type crcTable [8][256]uint32

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

// checksum returns crc32.Checksum(p, crc32.MakeTable(poly)).
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
