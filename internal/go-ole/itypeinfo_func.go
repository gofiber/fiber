//go:build !windows
// +build !windows

package ole

func (v *ITypeInfo) GetTypeAttr() (*TYPEATTR, error) {
	return nil, NewError(E_NOTIMPL)
}
