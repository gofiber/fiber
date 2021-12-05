//go:build !race
// +build !race

package encoder

func CompileToGetCodeSet(typeptr uintptr) (*OpcodeSet, error) {
	if typeptr > typeAddr.MaxTypeAddr {
		return compileToGetCodeSetSlowPath(typeptr)
	}
	index := (typeptr - typeAddr.BaseTypeAddr) >> typeAddr.AddrShift
	if codeSet := cachedOpcodeSets[index]; codeSet != nil {
		return codeSet, nil
	}
	codeSet, err := newCompiler().compile(typeptr)
	if err != nil {
		return nil, err
	}
	cachedOpcodeSets[index] = codeSet
	return codeSet, nil
}
