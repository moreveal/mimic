package state

import "reflect"

// Clone returns an independently owned environment, including nested catalogs.
// Time values contain immutable implementation pointers and are copied as values.
func (e Environment) Clone() Environment {
	out := cloneValue(reflect.ValueOf(e)).Interface().(Environment)
	return out
}

// Fork is an internal Page view of an already-owned Context environment.
// Nested catalogs are immutable after Context construction. Page emulation
// replaces slices, maps and pointers when it changes them; scalar fields live
// in the copied struct. Public Environment getters must still use Clone.
func (e Environment) Fork() Environment { return e }
func cloneValue(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		n := reflect.New(v.Type().Elem())
		n.Elem().Set(cloneValue(v.Elem()))
		return n
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		n := reflect.MakeMapWithSize(v.Type(), v.Len())
		it := v.MapRange()
		for it.Next() {
			n.SetMapIndex(it.Key(), cloneValue(it.Value()))
		}
		return n
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		n := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			n.Index(i).Set(cloneValue(v.Index(i)))
		}
		return n
	case reflect.Struct:
		n := reflect.New(v.Type()).Elem()
		n.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				n.Field(i).Set(cloneValue(v.Field(i)))
			}
		}
		return n
	default:
		return v
	}
}
