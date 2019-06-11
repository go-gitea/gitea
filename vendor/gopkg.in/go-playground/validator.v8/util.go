package validator

import (
	"reflect"
	"strconv"
	"strings"
)

const (
	blank              = ""
	namespaceSeparator = "."
	leftBracket        = "["
	rightBracket       = "]"
	restrictedTagChars = ".[],|=+()`~!@#$%^&*\\\"/?<>{}"
	restrictedAliasErr = "Alias '%s' either contains restricted characters or is the same as a restricted tag needed for normal operation"
	restrictedTagErr   = "Tag '%s' either contains restricted characters or is the same as a restricted tag needed for normal operation"
)

var (
	restrictedTags = map[string]struct{}{
		diveTag:           {},
		existsTag:         {},
		structOnlyTag:     {},
		omitempty:         {},
		skipValidationTag: {},
		utf8HexComma:      {},
		utf8Pipe:          {},
		noStructLevelTag:  {},
	}
)

// ExtractType gets the actual underlying type of field value.
// It will dive into pointers, customTypes and return you the
// underlying value and it's kind.
// it is exposed for use within you Custom Functions
func (v *Validate) ExtractType(current reflect.Value) (reflect.Value, reflect.Kind) {

	val, k, _ := v.extractTypeInternal(current, false)
	return val, k
}

// only exists to not break backward compatibility, needed to return the third param for a bug fix internally
func (v *Validate) extractTypeInternal(current reflect.Value, nullable bool) (reflect.Value, reflect.Kind, bool) {

	switch current.Kind() {
	case reflect.Ptr:

		nullable = true

		if current.IsNil() {
			return current, reflect.Ptr, nullable
		}

		return v.extractTypeInternal(current.Elem(), nullable)

	case reflect.Interface:

		nullable = true

		if current.IsNil() {
			return current, reflect.Interface, nullable
		}

		return v.extractTypeInternal(current.Elem(), nullable)

	case reflect.Invalid:
		return current, reflect.Invalid, nullable

	default:

		if v.hasCustomFuncs {

			if fn, ok := v.customTypeFuncs[current.Type()]; ok {
				return v.extractTypeInternal(reflect.ValueOf(fn(current)), nullable)
			}
		}

		return current, current.Kind(), nullable
	}
}

// GetStructFieldOK traverses a struct to retrieve a specific field denoted by the provided namespace and
// returns the field, field kind and whether is was successful in retrieving the field at all.
// NOTE: when not successful ok will be false, this can happen when a nested struct is nil and so the field
// could not be retrieved because it didn't exist.
func (v *Validate) GetStructFieldOK(current reflect.Value, namespace string) (reflect.Value, reflect.Kind, bool) {

	current, kind := v.ExtractType(current)

	if kind == reflect.Invalid {
		return current, kind, false
	}

	if namespace == blank {
		return current, kind, true
	}

	switch kind {

	case reflect.Ptr, reflect.Interface:

		return current, kind, false

	case reflect.Struct:

		typ := current.Type()
		fld := namespace
		ns := namespace

		if typ != timeType && typ != timePtrType {

			idx := strings.Index(namespace, namespaceSeparator)

			if idx != -1 {
				fld = namespace[:idx]
				ns = namespace[idx+1:]
			} else {
				ns = blank
			}

			bracketIdx := strings.Index(fld, leftBracket)
			if bracketIdx != -1 {
				fld = fld[:bracketIdx]

				ns = namespace[bracketIdx:]
			}

			current = current.FieldByName(fld)

			return v.GetStructFieldOK(current, ns)
		}

	case reflect.Array, reflect.Slice:
		idx := strings.Index(namespace, leftBracket)
		idx2 := strings.Index(namespace, rightBracket)

		arrIdx, _ := strconv.Atoi(namespace[idx+1 : idx2])

		if arrIdx >= current.Len() {
			return current, kind, false
		}

		startIdx := idx2 + 1

		if startIdx < len(namespace) {
			if namespace[startIdx:startIdx+1] == namespaceSeparator {
				startIdx++
			}
		}

		return v.GetStructFieldOK(current.Index(arrIdx), namespace[startIdx:])

	case reflect.Map:
		idx := strings.Index(namespace, leftBracket) + 1
		idx2 := strings.Index(namespace, rightBracket)

		endIdx := idx2

		if endIdx+1 < len(namespace) {
			if namespace[endIdx+1:endIdx+2] == namespaceSeparator {
				endIdx++
			}
		}

		key := namespace[idx:idx2]

		switch current.Type().Key().Kind() {
		case reflect.Int:
			i, _ := strconv.Atoi(key)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(i)), namespace[endIdx+1:])
		case reflect.Int8:
			i, _ := strconv.ParseInt(key, 10, 8)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(int8(i))), namespace[endIdx+1:])
		case reflect.Int16:
			i, _ := strconv.ParseInt(key, 10, 16)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(int16(i))), namespace[endIdx+1:])
		case reflect.Int32:
			i, _ := strconv.ParseInt(key, 10, 32)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(int32(i))), namespace[endIdx+1:])
		case reflect.Int64:
			i, _ := strconv.ParseInt(key, 10, 64)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(i)), namespace[endIdx+1:])
		case reflect.Uint:
			i, _ := strconv.ParseUint(key, 10, 0)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(uint(i))), namespace[endIdx+1:])
		case reflect.Uint8:
			i, _ := strconv.ParseUint(key, 10, 8)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(uint8(i))), namespace[endIdx+1:])
		case reflect.Uint16:
			i, _ := strconv.ParseUint(key, 10, 16)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(uint16(i))), namespace[endIdx+1:])
		case reflect.Uint32:
			i, _ := strconv.ParseUint(key, 10, 32)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(uint32(i))), namespace[endIdx+1:])
		case reflect.Uint64:
			i, _ := strconv.ParseUint(key, 10, 64)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(i)), namespace[endIdx+1:])
		case reflect.Float32:
			f, _ := strconv.ParseFloat(key, 32)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(float32(f))), namespace[endIdx+1:])
		case reflect.Float64:
			f, _ := strconv.ParseFloat(key, 64)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(f)), namespace[endIdx+1:])
		case reflect.Bool:
			b, _ := strconv.ParseBool(key)
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(b)), namespace[endIdx+1:])

		// reflect.Type = string
		default:
			return v.GetStructFieldOK(current.MapIndex(reflect.ValueOf(key)), namespace[endIdx+1:])
		}
	}

	// if got here there was more namespace, cannot go any deeper
	panic("Invalid field namespace")
}

// asInt returns the parameter as a int64
// or panics if it can't convert
func asInt(param string) int64 {

	i, err := strconv.ParseInt(param, 0, 64)
	panicIf(err)

	return i
}

// asUint returns the parameter as a uint64
// or panics if it can't convert
func asUint(param string) uint64 {

	i, err := strconv.ParseUint(param, 0, 64)
	panicIf(err)

	return i
}

// asFloat returns the parameter as a float64
// or panics if it can't convert
func asFloat(param string) float64 {

	i, err := strconv.ParseFloat(param, 64)
	panicIf(err)

	return i
}

func panicIf(err error) {
	if err != nil {
		panic(err.Error())
	}
}
