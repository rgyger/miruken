package internal

import (
	"reflect"
	"runtime"
	"strings"
)

// TypeOf returns reflect.Type of generic argument.
func TypeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// ValueAs returns the value of v As the type T.
// Provides panics if the value isn't assignable to T.
func ValueAs[T any](v reflect.Value) (r T) {
	reflect.ValueOf(&r).Elem().Set(v)
	return
}

// IsNil determine if the val is typed or untyped nil.
func IsNil(val any) bool {
	if val == nil {
		return true
	}
	v := reflect.ValueOf(val)
	switch v.Type().Kind() {
	case reflect.Chan,
		 reflect.Func,
		 reflect.Interface,
		 reflect.Map,
		 reflect.Ptr,
		 reflect.Slice:
			 return v.IsNil()
	default:
		return false
	}
}

// IsStruct returns true if val is a struct value.
func IsStruct(val any) bool {
	if val == nil {
		return false
	}
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	return v.Kind() == reflect.Struct
}

// IsAny returns true if tpe is assignable to any.
func IsAny(typ reflect.Type) bool {
	return AnyType.AssignableTo(typ)
}

// New creates a new T and optionally initializes it.
func New[T any]() *T {
	var (
		t = new(T)
		a any = t
	)
	if init, ok := a.(objInit); ok {
		if err := init.Init(); err != nil {
			panic("init: " + err.Error())
		}
	}
	return t
}

// TargetValue validates the interface contains a
// non-nil typed pointer and return reflect.Value.
func TargetValue(target any) reflect.Value {
	if IsNil(target) {
		panic("source cannot be nil")
	}
	val := reflect.ValueOf(target)
	typ := val.Type()
	if typ.Kind() != reflect.Ptr || val.IsNil() {
		panic("target must be a non-nil pointer")
	}
	return val
}

// TargetSliceValue validates the interface contains a
// non-nil typed slice pointer and return reflect.Value.
func TargetSliceValue(target any) reflect.Value {
	val := TargetValue(target)
	typ := val.Type()
	if typ.Elem().Kind() != reflect.Slice {
		panic("target must be a non-nil slice pointer")
	}
	return val
}

// CopyIndirect copies the contents of src into the target
// pointer or reflect.Value.
func CopyIndirect(src any, target any) {
	var val reflect.Value
	if v, ok := target.(reflect.Value); ok {
		if v.Kind() != reflect.Ptr || val.IsNil() {
			panic("target must be a non-nil pointer")
		}
		val = v
	} else {
		val = TargetValue(target)
	}
	srcVal := reflect.ValueOf(src)
	if val == srcVal {
		return
	}
	val = reflect.Indirect(val)
	typ := val.Type()
	if src == nil {
		val.Set(reflect.Zero(typ))
	} else {
		if st := srcVal.Type(); st.Kind() == reflect.Ptr && st.Elem() == typ {
			srcVal = reflect.Indirect(srcVal)
		} else if !st.AssignableTo(typ) && st.ConvertibleTo(typ) {
			srcVal = srcVal.Convert(typ)
		} else if typ.Kind() == reflect.Slice {
			if sv, ok := CoerceSlice(srcVal, typ.Elem()); ok {
				srcVal = sv
			}
		}
		val.Set(srcVal)
	}
}

// CopySliceIndirect copies the contents of src slice into the
// target pointer or reflect.Value.
func CopySliceIndirect(src []any, target any) {
	var val reflect.Value
	if v, ok := target.(reflect.Value); ok {
		if v.Kind() != reflect.Slice {
			panic("target must be a non-nil slice pointer")
		}
		val = v
	} else {
		val = TargetSliceValue(target)
	}
	val = reflect.Indirect(val)
	typ := val.Type()
	if src == nil {
		val.Set(reflect.MakeSlice(typ, 0, 0))
		return
	}
	elemTyp := typ.Elem()
	slice  := reflect.MakeSlice(typ, len(src), len(src))
	for i, element := range src {
		elVal := reflect.ValueOf(element)
		elTyp := elVal.Type()
		if !elTyp.AssignableTo(elemTyp) && elTyp.ConvertibleTo(elemTyp) {
			elVal = elVal.Convert(elemTyp)
		}
		slice.Index(i).Set(elVal)
	}
	val.Set(slice)
}

// CoerceSlice attempts to upcast the elements of a slice
// and return the newly promoted slice and true if successful.
// If elemType is nil, the most specific type will be inferred.
func CoerceSlice(
	slice   reflect.Value,
	elemTyp reflect.Type,
) (reflect.Value, bool) {
	st := slice.Type()
	if st.Kind() != reflect.Slice {
		panic("expected a slice value")
	}
	se := st.Elem()
	sl := slice.Len()
	if elemTyp == nil {
		for i := 0; i < sl; i++ {
			elem := slice.Index(i)
			typ := elem.Type()
			if typ.Kind() == reflect.Interface {
				typ = elem.Elem().Type()
			}
			if elemTyp == nil {
				elemTyp = typ
			} else if typ != elemTyp {
				if elemTyp.AssignableTo(typ) {
					elemTyp = typ
				} else {
					return slice, false
				}
			}
		}
	}
	if elemTyp == nil || elemTyp == se {
		return slice, false
	}
	newSlice := reflect.MakeSlice(reflect.SliceOf(elemTyp), sl, sl)
	for i := 0; i < sl; i++ {
		elem := reflect.ValueOf(slice.Index(i).Interface())
		if elt := elem.Type(); !elt.AssignableTo(elemTyp) && elt.ConvertibleTo(elemTyp) {
			elem = elem.Convert(elemTyp)
		}
		newSlice.Index(i).Set(elem)
	}
	return newSlice, true
}

func Exported(t any) bool {
	if reflect.TypeOf(t).Kind() == reflect.Func {
		path := strings.Split(runtime.FuncForPC(reflect.ValueOf(t).Pointer()).Name(), ".")
		name := path[len(path)-1]
		return strings.ToUpper(name[0:1]) == name[0:1]
	}
	switch m := t.(type) {
	case reflect.Type:
		if m.Kind() == reflect.Ptr {
			m = m.Elem()
		}
		name := m.Name()
		return strings.ToUpper(name[0:1]) == name[0:1]
	case reflect.Method:
		return strings.ToUpper(m.Name[0:1]) == m.Name[0:1] &&
			Exported(m.Type.In(0))
	}
	return true
}

func CoerceToPtr(
	givenType   reflect.Type,
	desiredType reflect.Type,
) reflect.Type {
	if givenType.AssignableTo(desiredType) {
		return givenType
	} else if givenType.Kind() != reflect.Ptr {
		givenType = reflect.PtrTo(givenType)
		if givenType.AssignableTo(desiredType) {
			return givenType
		}
	}
	return nil
}

func NewWithTag(
	typ reflect.Type,
	tag reflect.StructTag,
) (any, error) {
	if typ.Kind() == reflect.Ptr {
		obj := reflect.New(typ.Elem()).Interface()
		if err := tryInitObj(obj, tag); err != nil {
			return nil, err
		}
		return obj, nil
	} else {
		val := reflect.New(typ)
		if err := tryInitObj(val.Interface(), tag); err != nil {
			return nil, err
		}
		obj := val.Elem().Interface()
		return obj, nil
	}
}

func tryInitObj(obj any, tag reflect.StructTag) error {
	if tag != "" {
		if oi, ok := obj.(objInitWithTag); ok {
			if err := oi.InitWithTag(tag); err != nil {
				return err
			}
			return nil
		}
	}
	if oi, ok := obj.(objInit); ok {
		if err := oi.Init(); err != nil {
			return err
		}
		return nil
	}
	return nil
}


type (
	objInit interface{Init() error}
	objInitWithTag interface{InitWithTag(reflect.StructTag) error}
)
