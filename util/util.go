package util

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	starlibtime "github.com/qri-io/starlib/time"
)

// asString unquotes a starlark string value
func asString(x starlark.Value) (string, error) {
	return strconv.Unquote(x.String())
}

// IsEmptyString checks is a starlark string is empty ("" for a go string)
// starlark.String.String performs repr-style quotation, which is necessary
// for the starlark.Value contract but a frequent source of errors in API
// clients. This helper method makes sure it'll work properly
func IsEmptyString(s starlark.String) bool {
	return s.String() == `""`
}

// Unmarshal decodes a starlark.Value into it's golang counterpart
func Unmarshal(x starlark.Value) (val interface{}, err error) {
	switch v := x.(type) {
	case starlark.NoneType:
		val = nil
	case starlark.Bool:
		val = v.Truth() == starlark.True
	case starlark.Int:
		if i, ok := v.Int64(); !ok {
			err = fmt.Errorf("couldn't parse int")
		} else {
			if math.MinInt32 <= i && i <= math.MaxInt32 {
				val = int(i)
			} else {
				val = i
			}
		}
	case starlark.Float:
		if f, ok := starlark.AsFloat(x); !ok {
			err = fmt.Errorf("couldn't parse float")
		} else {
			val = f
		}
	case starlark.String:
		val = v.GoString()
	case starlibtime.Time:
		val = time.Time(v)
	case *starlark.Dict:
		var (
			dictVal starlark.Value
			pval    interface{}
			kval    interface{}
			keys    []interface{}
			vals    []interface{}
			// key as interface if found one key is not a string
			ki bool
		)

		for _, k := range v.Keys() {
			dictVal, _, err = v.Get(k)
			if err != nil {
				return
			}

			pval, err = Unmarshal(dictVal)
			if err != nil {
				err = fmt.Errorf("unmarshaling starlark value: %w", err)
				return
			}

			kval, err = Unmarshal(k)
			if err != nil {
				err = fmt.Errorf("unmarshaling starlark key: %w", err)
				return
			}

			if _, ok := kval.(string); !ok {
				// found key as not a string
				ki = true
			}

			keys = append(keys, kval)
			vals = append(vals, pval)
		}

		// prepare result

		rs := map[string]interface{}{}
		ri := map[interface{}]interface{}{}

		for i, key := range keys {
			// key as interface
			if ki {
				ri[key] = vals[i]
			} else {
				rs[key.(string)] = vals[i]
			}
		}

		if ki {
			val = ri // map[interface{}]interface{}
		} else {
			val = rs // map[string]interface{}
		}
	case *starlark.List:
		var (
			i       int
			listVal starlark.Value
			iter    = v.Iterate()
			value   = make([]interface{}, v.Len())
		)

		defer iter.Done()
		for iter.Next(&listVal) {
			value[i], err = Unmarshal(listVal)
			if err != nil {
				return
			}
			i++
		}
		val = value
	case starlark.Tuple:
		var (
			i        int
			tupleVal starlark.Value
			iter     = v.Iterate()
			value    = make([]interface{}, v.Len())
		)

		defer iter.Done()
		for iter.Next(&tupleVal) {
			value[i], err = Unmarshal(tupleVal)
			if err != nil {
				return
			}
			i++
		}
		val = value
	case *starlark.Set:
		fmt.Println("errnotdone: SET")
		err = fmt.Errorf("sets aren't yet supported")
	case *starlarkstruct.Struct:
		if _var, ok := v.Constructor().(Unmarshaler); ok {
			err = _var.UnmarshalStarlark(x)
			if err != nil {
				err = errors.Wrapf(err, "failed marshal %q to Starlark object", v.Constructor().Type())
				return
			}
			val = _var
		} else {
			dict := new(starlark.Dict)
			for _, name := range v.AttrNames() {
				val, _ := v.Attr(name)
				dict.SetKey(starlark.String(name), val)
			}
			val, err = Unmarshal(dict)
		}
	case starlark.HasAttrs:
		dict := new(starlark.Dict)
		for _, name := range v.AttrNames() {
			val, _ := v.Attr(name)
			if val.Type() != "builtin_function_or_method" {
				dict.SetKey(starlark.String(name), val)
			}
		}
		val, err = Unmarshal(dict)
	default:
		fmt.Println("errbadtype:", x.Type())
		err = fmt.Errorf("unrecognized starlark type: %s", x.Type())
	}
	return
}

// Marshal turns go values into starlark types
func Marshal(data interface{}) (v starlark.Value, err error) {
	switch x := data.(type) {
	case nil:
		v = starlark.None
	case bool:
		v = starlark.Bool(x)
	case string:
		v = starlark.String(x)
	case int:
		v = starlark.MakeInt(x)
	case int8:
		v = starlark.MakeInt(int(x))
	case int16:
		v = starlark.MakeInt(int(x))
	case int32:
		v = starlark.MakeInt(int(x))
	case int64:
		v = starlark.MakeInt64(x)
	case uint:
		v = starlark.MakeUint(x)
	case uint8:
		v = starlark.MakeUint(uint(x))
	case uint16:
		v = starlark.MakeUint(uint(x))
	case uint32:
		v = starlark.MakeUint(uint(x))
	case uint64:
		v = starlark.MakeUint64(x)
	case float32:
		v = starlark.Float(float64(x))
	case float64:
		v = starlark.Float(x)
	case time.Time:
		v = starlibtime.Time(x)
	case []bool:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []string:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []int:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []int8:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []int16:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []int32:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []int64:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []uint:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []uint8:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []uint16:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []uint32:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []uint64:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []float32:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []float64:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []time.Time:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []map[interface{}]interface{}:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []map[string]interface{}:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case []interface{}:
		var elems = make([]starlark.Value, len(x))
		for i, val := range x {
			elems[i], err = Marshal(val)
			if err != nil {
				return
			}
		}
		v = starlark.NewList(elems)
	case map[interface{}]interface{}:
		dict := &starlark.Dict{}
		var elem starlark.Value
		for ki, val := range x {
			var key starlark.Value
			key, err = Marshal(ki)
			if err != nil {
				return
			}

			elem, err = Marshal(val)
			if err != nil {
				return
			}
			if err = dict.SetKey(key, elem); err != nil {
				return
			}
		}
		v = dict
	case map[string]interface{}:
		dict := &starlark.Dict{}
		var elem starlark.Value
		for key, val := range x {
			elem, err = Marshal(val)
			if err != nil {
				return
			}
			if err = dict.SetKey(starlark.String(key), elem); err != nil {
				return
			}
		}
		v = dict
	case Marshaler:
		v, err = x.MarshalStarlark()
	default:
		return starlark.None, fmt.Errorf("unrecognized type: %#v", x)
	}
	return
}

// Unmarshaler is the interface use to unmarshal starlark custom types.
type Unmarshaler interface {
	// UnmarshalStarlark unmarshal a starlark object to custom type.
	UnmarshalStarlark(starlark.Value) error
}

// Marshaler is the interface use to marshal starlark custom types.
type Marshaler interface {
	// MarshalStarlark marshal a custom type to starlark object.
	MarshalStarlark() (starlark.Value, error)
}
