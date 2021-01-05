package gotiny

import (
	"reflect"
	"strconv"
)

var (
	type2name = map[reflect.Type]string{}
	name2type = map[string]reflect.Type{}
)

func GetName(obj interface{}) string {
	return GetNameByType(reflect.TypeOf(obj))
}
func GetNameByType(rt reflect.Type) string {
	return string(getName([]byte(nil), rt))
}

func getName(prefix []byte, rt reflect.Type) []byte {
	if rt == nil || rt.Kind() == reflect.Invalid {
		return append(prefix, []byte("<nil>")...)
	}
	if rt.Name() == "" { //未命名的，组合类型
		switch rt.Kind() {
		case reflect.Ptr:
			return getName(append(prefix, '*'), rt.Elem())
		case reflect.Array:
			return getName(append(prefix, "["+strconv.Itoa(rt.Len())+"]"...), rt.Elem())
		case reflect.Slice:
			return getName(append(prefix, '[', ']'), rt.Elem())
		case reflect.Struct:
			prefix = append(prefix, "struct {"...)
			nf := rt.NumField()
			if nf > 0 {
				prefix = append(prefix, ' ')
			}
			for i := 0; i < nf; i++ {
				field := rt.Field(i)
				if field.Anonymous {
					prefix = getName(prefix, field.Type)
				} else {
					prefix = getName(append(prefix, field.Name+" "...), field.Type)
				}
				if i != nf-1 {
					prefix = append(prefix, ';', ' ')
				} else {
					prefix = append(prefix, ' ')
				}
			}
			return append(prefix, '}')
		case reflect.Map:
			return getName(append(getName(append(prefix, "map["...), rt.Key()), ']'), rt.Elem())
		case reflect.Interface:
			prefix = append(prefix, "interface {"...)
			nm := rt.NumMethod()
			if nm > 0 {
				prefix = append(prefix, ' ')
			}
			for i := 0; i < nm; i++ {
				method := rt.Method(i)
				fn := getName([]byte(nil), method.Type)
				prefix = append(prefix, method.Name+string(fn[4:])...)
				if i != nm-1 {
					prefix = append(prefix, ';', ' ')
				} else {
					prefix = append(prefix, ' ')
				}
			}
			return append(prefix, '}')
		case reflect.Func:
			prefix = append(prefix, "func("...)
			for i := 0; i < rt.NumIn(); i++ {
				prefix = getName(prefix, rt.In(i))
				if i != rt.NumIn()-1 {
					prefix = append(prefix, ',', ' ')
				}
			}
			prefix = append(prefix, ')')
			no := rt.NumOut()
			if no > 0 {
				prefix = append(prefix, ' ')
			}
			if no > 1 {
				prefix = append(prefix, '(')
			}
			for i := 0; i < no; i++ {
				prefix = getName(prefix, rt.Out(i))
				if i != no-1 {
					prefix = append(prefix, ',', ' ')
				}
			}
			if no > 1 {
				prefix = append(prefix, ')')
			}
			return prefix
		}
	}

	if rt.PkgPath() == "" {
		prefix = append(prefix, rt.Name()...)
	} else {
		prefix = append(prefix, rt.PkgPath()+"."+rt.Name()...)
	}
	return prefix
}

func getNameOfType(rt reflect.Type) string {
	if name, has := type2name[rt]; has {
		return name
	} else {
		return registerType(rt)
	}
}

func Register(i interface{}) string {
	return registerType(reflect.TypeOf(i))
}

func registerType(rt reflect.Type) string {
	name := GetNameByType(rt)
	RegisterName(name, rt)
	return name
}

func RegisterName(name string, rt reflect.Type) {
	if name == "" {
		panic("attempt to register empty name")
	}

	if rt == nil || rt.Kind() == reflect.Invalid {
		panic("attempt to register nil type or invalid type")
	}

	if _, has := type2name[rt]; has {
		panic("gotiny: registering duplicate types for " + GetNameByType(rt))
	}

	if _, has := name2type[name]; has {
		panic("gotiny: registering name" + name + " is exist")
	}
	name2type[name] = rt
	type2name[rt] = name
}
