//go:build race
// +build race

package encoder

import (
	"sync"
)

var setsMu sync.RWMutex

func CompileToGetCodeSet(typeptr uintptr) (*OpcodeSet, error) {
	if typeptr > typeAddr.MaxTypeAddr {
		return compileToGetCodeSetSlowPath(typeptr)
	}
	index := (typeptr - typeAddr.BaseTypeAddr) >> typeAddr.AddrShift
	setsMu.RLock()
	if codeSet := cachedOpcodeSets[index]; codeSet != nil {
		setsMu.RUnlock()
		return codeSet, nil
	}
	setsMu.RUnlock()

	codeSet, err := newCompiler().compile(typeptr)
	if err != nil {
		return nil, err
	}
	setsMu.Lock()
	cachedOpcodeSets[index] = codeSet
	setsMu.Unlock()
	return codeSet, nil
}
