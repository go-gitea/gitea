package code

import (
	"fmt"
	"go/types"
)

// CompatibleTypes isnt a strict comparison, it allows for pointer differences
func CompatibleTypes(expected types.Type, actual types.Type) error {
	//fmt.Println("Comparing ", expected.String(), actual.String())

	// Special case to deal with pointer mismatches
	{
		expectedPtr, expectedIsPtr := expected.(*types.Pointer)
		actualPtr, actualIsPtr := actual.(*types.Pointer)

		if expectedIsPtr && actualIsPtr {
			return CompatibleTypes(expectedPtr.Elem(), actualPtr.Elem())
		}
		if expectedIsPtr && !actualIsPtr {
			return CompatibleTypes(expectedPtr.Elem(), actual)
		}
		if !expectedIsPtr && actualIsPtr {
			return CompatibleTypes(expected, actualPtr.Elem())
		}
	}

	switch expected := expected.(type) {
	case *types.Slice:
		if actual, ok := actual.(*types.Slice); ok {
			return CompatibleTypes(expected.Elem(), actual.Elem())
		}

	case *types.Array:
		if actual, ok := actual.(*types.Array); ok {
			if expected.Len() != actual.Len() {
				return fmt.Errorf("array length differs")
			}

			return CompatibleTypes(expected.Elem(), actual.Elem())
		}

	case *types.Basic:
		if actual, ok := actual.(*types.Basic); ok {
			if actual.Kind() != expected.Kind() {
				return fmt.Errorf("basic kind differs, %s != %s", expected.Name(), actual.Name())
			}

			return nil
		}

	case *types.Struct:
		if actual, ok := actual.(*types.Struct); ok {
			if expected.NumFields() != actual.NumFields() {
				return fmt.Errorf("number of struct fields differ")
			}

			for i := 0; i < expected.NumFields(); i++ {
				if expected.Field(i).Name() != actual.Field(i).Name() {
					return fmt.Errorf("struct field %d name differs, %s != %s", i, expected.Field(i).Name(), actual.Field(i).Name())
				}
				if err := CompatibleTypes(expected.Field(i).Type(), actual.Field(i).Type()); err != nil {
					return err
				}
			}
			return nil
		}

	case *types.Tuple:
		if actual, ok := actual.(*types.Tuple); ok {
			if expected.Len() != actual.Len() {
				return fmt.Errorf("tuple length differs, %d != %d", expected.Len(), actual.Len())
			}

			for i := 0; i < expected.Len(); i++ {
				if err := CompatibleTypes(expected.At(i).Type(), actual.At(i).Type()); err != nil {
					return err
				}
			}

			return nil
		}

	case *types.Signature:
		if actual, ok := actual.(*types.Signature); ok {
			if err := CompatibleTypes(expected.Params(), actual.Params()); err != nil {
				return err
			}
			if err := CompatibleTypes(expected.Results(), actual.Results()); err != nil {
				return err
			}

			return nil
		}
	case *types.Interface:
		if actual, ok := actual.(*types.Interface); ok {
			if expected.NumMethods() != actual.NumMethods() {
				return fmt.Errorf("interface method count differs, %d != %d", expected.NumMethods(), actual.NumMethods())
			}

			for i := 0; i < expected.NumMethods(); i++ {
				if expected.Method(i).Name() != actual.Method(i).Name() {
					return fmt.Errorf("interface method %d name differs, %s != %s", i, expected.Method(i).Name(), actual.Method(i).Name())
				}
				if err := CompatibleTypes(expected.Method(i).Type(), actual.Method(i).Type()); err != nil {
					return err
				}
			}

			return nil
		}

	case *types.Map:
		if actual, ok := actual.(*types.Map); ok {
			if err := CompatibleTypes(expected.Key(), actual.Key()); err != nil {
				return err
			}

			if err := CompatibleTypes(expected.Elem(), actual.Elem()); err != nil {
				return err
			}

			return nil
		}

	case *types.Chan:
		if actual, ok := actual.(*types.Chan); ok {
			return CompatibleTypes(expected.Elem(), actual.Elem())
		}

	case *types.Named:
		if actual, ok := actual.(*types.Named); ok {
			if NormalizeVendor(expected.Obj().Pkg().Path()) != NormalizeVendor(actual.Obj().Pkg().Path()) {
				return fmt.Errorf(
					"package name of named type differs, %s != %s",
					NormalizeVendor(expected.Obj().Pkg().Path()),
					NormalizeVendor(actual.Obj().Pkg().Path()),
				)
			}

			if expected.Obj().Name() != actual.Obj().Name() {
				return fmt.Errorf(
					"named type name differs, %s != %s",
					NormalizeVendor(expected.Obj().Name()),
					NormalizeVendor(actual.Obj().Name()),
				)
			}

			return nil
		}

		// Before models are generated all missing references will be Invalid Basic references.
		// lets assume these are valid too.
		if actual, ok := actual.(*types.Basic); ok && actual.Kind() == types.Invalid {
			return nil
		}

	default:
		return fmt.Errorf("missing support for %T", expected)
	}

	return fmt.Errorf("type mismatch %T != %T", expected, actual)
}
