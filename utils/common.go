// ⚡️ Fiber is an Express inspired web framework written in Go with ☕️
// 🤖 Github Repository: https://github.com/gofiber/fiber
// 📌 API Documentation: https://docs.gofiber.io

package utils

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"os"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"

	googleuuid "github.com/gofiber/fiber/v2/internal/uuid"
)

const toLowerTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@abcdefghijklmnopqrstuvwxyz[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"
const toUpperTable = "\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`ABCDEFGHIJKLMNOPQRSTUVWXYZ{|}~\u007f\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f\x90\x91\x92\x93\x94\x95\x96\x97\x98\x99\x9a\x9b\x9c\x9d\x9e\x9f\xa0\xa1\xa2\xa3\xa4\xa5\xa6\xa7\xa8\xa9\xaa\xab\xac\xad\xae\xaf\xb0\xb1\xb2\xb3\xb4\xb5\xb6\xb7\xb8\xb9\xba\xbb\xbc\xbd\xbe\xbf\xc0\xc1\xc2\xc3\xc4\xc5\xc6\xc7\xc8\xc9\xca\xcb\xcc\xcd\xce\xcf\xd0\xd1\xd2\xd3\xd4\xd5\xd6\xd7\xd8\xd9\xda\xdb\xdc\xdd\xde\xdf\xe0\xe1\xe2\xe3\xe4\xe5\xe6\xe7\xe8\xe9\xea\xeb\xec\xed\xee\xef\xf0\xf1\xf2\xf3\xf4\xf5\xf6\xf7\xf8\xf9\xfa\xfb\xfc\xfd\xfe\xff"

// Copyright © 2014, Roger Peppe
// github.com/rogpeppe/fastuuid
// All rights reserved.

var uuidSeed [24]byte
var uuidCounter uint64
var uuidSetup sync.Once

// UUID generates an universally unique identifier (UUID)
func UUID() string {
	// Setup seed & counter once
	uuidSetup.Do(func() {
		if _, err := rand.Read(uuidSeed[:]); err != nil {
			return
		}
		uuidCounter = binary.LittleEndian.Uint64(uuidSeed[:8])
	})
	if atomic.LoadUint64(&uuidCounter) <= 0 {
		return "00000000-0000-0000-0000-000000000000"
	}
	// first 8 bytes differ, taking a slice of the first 16 bytes
	x := atomic.AddUint64(&uuidCounter, 1)
	uuid := uuidSeed
	binary.LittleEndian.PutUint64(uuid[:8], x)
	uuid[6], uuid[9] = uuid[9], uuid[6]

	// RFC4122 v4
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = uuid[8]&0x3f | 0x80

	// create UUID representation of the first 128 bits
	b := make([]byte, 36)
	hex.Encode(b[0:8], uuid[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], uuid[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], uuid[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], uuid[8:10])
	b[23] = '-'
	hex.Encode(b[24:], uuid[10:16])

	return UnsafeString(b)
}

// UUIDv4 returns a Random (Version 4) UUID.
// The strength of the UUIDs is based on the strength of the crypto/rand package.
func UUIDv4() string {
	token, err := googleuuid.NewRandom()
	if err != nil {
		return UUID()
	}
	return token.String()
}

// FunctionName returns function name
func FunctionName(fn interface{}) string {
	t := reflect.ValueOf(fn).Type()
	if t.Kind() == reflect.Func {
		return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	}
	return t.String()
}

// GetArgument check if key is in arguments
func GetArgument(arg string) bool {
	for i := range os.Args[1:] {
		if os.Args[1:][i] == arg {
			return true
		}
	}
	return false
}
