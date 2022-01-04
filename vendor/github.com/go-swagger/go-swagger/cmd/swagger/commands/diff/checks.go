package diff

import (
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
)

// CompareEnums returns added, deleted enum values
func CompareEnums(left, right []interface{}) []TypeDiff {
	diffs := []TypeDiff{}

	leftStrs := []string{}
	rightStrs := []string{}
	for _, eachLeft := range left {
		leftStrs = append(leftStrs, fmt.Sprintf("%v", eachLeft))
	}
	for _, eachRight := range right {
		rightStrs = append(rightStrs, fmt.Sprintf("%v", eachRight))
	}
	added, deleted, _ := fromStringArray(leftStrs).DiffsTo(rightStrs)
	if len(added) > 0 {
		typeChange := strings.Join(added, ",")
		diffs = append(diffs, TypeDiff{Change: AddedEnumValue, Description: typeChange})
	}
	if len(deleted) > 0 {
		typeChange := strings.Join(deleted, ",")
		diffs = append(diffs, TypeDiff{Change: DeletedEnumValue, Description: typeChange})
	}

	return diffs
}

// CompareProperties recursive property comparison
func CompareProperties(location DifferenceLocation, schema1 *spec.Schema, schema2 *spec.Schema, getRefFn1 SchemaFromRefFn, getRefFn2 SchemaFromRefFn, cmp CompareSchemaFn) []SpecDifference {
	propDiffs := []SpecDifference{}

	if schema1.Properties == nil && schema2.Properties == nil {
		return propDiffs
	}

	schema1Props := propertiesFor(schema1, getRefFn1)
	schema2Props := propertiesFor(schema2, getRefFn2)
	// find deleted and changed properties

	for eachProp1Name, eachProp1 := range schema1Props {
		eachProp1 := eachProp1
		childLoc := addChildDiffNode(location, eachProp1Name, eachProp1.Schema)

		if eachProp2, ok := schema2Props[eachProp1Name]; ok {
			diffs := CheckToFromRequired(eachProp1.Required, eachProp2.Required)
			if len(diffs) > 0 {
				for _, diff := range diffs {
					propDiffs = append(propDiffs, SpecDifference{DifferenceLocation: childLoc, Code: diff.Change})
				}
			}
			cmp(childLoc, eachProp1.Schema, eachProp2.Schema)
		} else {
			propDiffs = append(propDiffs, SpecDifference{DifferenceLocation: childLoc, Code: DeletedProperty})
		}
	}

	// find added properties
	for eachProp2Name, eachProp2 := range schema2.Properties {
		eachProp2 := eachProp2
		if _, ok := schema1.Properties[eachProp2Name]; !ok {
			childLoc := addChildDiffNode(location, eachProp2Name, &eachProp2)
			propDiffs = append(propDiffs, SpecDifference{DifferenceLocation: childLoc, Code: AddedProperty})
		}
	}
	return propDiffs

}

// CompareFloatValues compares a float data item
func CompareFloatValues(fieldName string, val1 *float64, val2 *float64, ifGreaterCode SpecChangeCode, ifLessCode SpecChangeCode) []TypeDiff {
	diffs := []TypeDiff{}
	if val1 != nil && val2 != nil {
		if *val2 > *val1 {
			diffs = append(diffs, TypeDiff{Change: ifGreaterCode, Description: fmt.Sprintf("%s %f->%f", fieldName, *val1, *val2)})
		} else if *val2 < *val1 {
			diffs = append(diffs, TypeDiff{Change: ifLessCode, Description: fmt.Sprintf("%s %f->%f", fieldName, *val1, *val2)})
		}
	} else {
		if val1 != val2 {
			if val1 != nil {
				diffs = append(diffs, TypeDiff{Change: DeletedConstraint, Description: fmt.Sprintf("%s(%f)", fieldName, *val1)})
			} else {
				diffs = append(diffs, TypeDiff{Change: AddedConstraint, Description: fmt.Sprintf("%s(%f)", fieldName, *val2)})
			}
		}
	}
	return diffs
}

// CompareIntValues compares to int data items
func CompareIntValues(fieldName string, val1 *int64, val2 *int64, ifGreaterCode SpecChangeCode, ifLessCode SpecChangeCode) []TypeDiff {
	diffs := []TypeDiff{}
	if val1 != nil && val2 != nil {
		if *val2 > *val1 {
			diffs = append(diffs, TypeDiff{Change: ifGreaterCode, Description: fmt.Sprintf("%s %d->%d", fieldName, *val1, *val2)})
		} else if *val2 < *val1 {
			diffs = append(diffs, TypeDiff{Change: ifLessCode, Description: fmt.Sprintf("%s %d->%d", fieldName, *val1, *val2)})
		}
	} else {
		if val1 != val2 {
			if val1 != nil {
				diffs = append(diffs, TypeDiff{Change: DeletedConstraint, Description: fmt.Sprintf("%s(%d)", fieldName, *val1)})
			} else {
				diffs = append(diffs, TypeDiff{Change: AddedConstraint, Description: fmt.Sprintf("%s(%d)", fieldName, *val2)})
			}
		}
	}
	return diffs
}

// CheckToFromPrimitiveType check for diff to or from a primitive
func CheckToFromPrimitiveType(diffs []TypeDiff, type1, type2 interface{}) []TypeDiff {

	type1IsPrimitive := isPrimitive(type1)
	type2IsPrimitive := isPrimitive(type2)

	// Primitive to Obj or Obj to Primitive
	if type1IsPrimitive != type2IsPrimitive {
		typeStr1, isarray1 := getSchemaType(type1)
		typeStr2, isarray2 := getSchemaType(type2)
		return addTypeDiff(diffs, TypeDiff{Change: ChangedType, FromType: formatTypeString(typeStr1, isarray1), ToType: formatTypeString(typeStr2, isarray2)})
	}

	return diffs
}

// CheckRefChange has the property ref changed
func CheckRefChange(diffs []TypeDiff, type1, type2 interface{}) (diffReturn []TypeDiff) {

	diffReturn = diffs
	if isRefType(type1) && isRefType(type2) {
		// both refs but to different objects (TODO detect renamed object)
		ref1 := definitionFromRef(getRef(type1))
		ref2 := definitionFromRef(getRef(type2))
		if ref1 != ref2 {
			diffReturn = addTypeDiff(diffReturn, TypeDiff{Change: RefTargetChanged, FromType: getSchemaTypeStr(type1), ToType: getSchemaTypeStr(type2)})
		}
	} else if isRefType(type1) != isRefType(type2) {
		diffReturn = addTypeDiff(diffReturn, TypeDiff{Change: ChangedType, FromType: getSchemaTypeStr(type1), ToType: getSchemaTypeStr(type2)})
	}
	return
}

// checkNumericTypeChanges checks for changes to or from a numeric type
func checkNumericTypeChanges(diffs []TypeDiff, type1, type2 *spec.SchemaProps) []TypeDiff {
	// Number
	_, type1IsNumeric := numberWideness[type1.Type[0]]
	_, type2IsNumeric := numberWideness[type2.Type[0]]

	if type1IsNumeric && type2IsNumeric {
		foundDiff := false
		if type1.ExclusiveMaximum && !type2.ExclusiveMaximum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: WidenedType, Description: fmt.Sprintf("Exclusive Maximum Removed:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
			foundDiff = true
		}
		if !type1.ExclusiveMaximum && type2.ExclusiveMaximum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: NarrowedType, Description: fmt.Sprintf("Exclusive Maximum Added:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
			foundDiff = true
		}
		if type1.ExclusiveMinimum && !type2.ExclusiveMinimum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: WidenedType, Description: fmt.Sprintf("Exclusive Minimum Removed:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
			foundDiff = true
		}
		if !type1.ExclusiveMinimum && type2.ExclusiveMinimum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: NarrowedType, Description: fmt.Sprintf("Exclusive Minimum Added:%v->%v", type1.ExclusiveMinimum, type2.ExclusiveMinimum)})
			foundDiff = true
		}
		if !foundDiff {
			maxDiffs := CompareFloatValues("Maximum", type1.Maximum, type2.Maximum, WidenedType, NarrowedType)
			diffs = append(diffs, maxDiffs...)
			minDiffs := CompareFloatValues("Minimum", type1.Minimum, type2.Minimum, NarrowedType, WidenedType)
			diffs = append(diffs, minDiffs...)
		}
	}
	return diffs
}

// CheckStringTypeChanges checks for changes to or from a string type
func CheckStringTypeChanges(diffs []TypeDiff, type1, type2 *spec.SchemaProps) []TypeDiff {
	// string changes
	if type1.Type[0] == StringType &&
		type2.Type[0] == StringType {
		minLengthDiffs := CompareIntValues("MinLength", type1.MinLength, type2.MinLength, NarrowedType, WidenedType)
		diffs = append(diffs, minLengthDiffs...)
		maxLengthDiffs := CompareIntValues("MaxLength", type1.MinLength, type2.MinLength, WidenedType, NarrowedType)
		diffs = append(diffs, maxLengthDiffs...)
		if type1.Pattern != type2.Pattern {
			diffs = addTypeDiff(diffs, TypeDiff{Change: ChangedType, Description: fmt.Sprintf("Pattern Changed:%s->%s", type1.Pattern, type2.Pattern)})
		}
		if type1.Type[0] == StringType {
			if len(type1.Enum) > 0 {
				enumDiffs := CompareEnums(type1.Enum, type2.Enum)
				diffs = append(diffs, enumDiffs...)
			}
		}
	}
	return diffs
}

// CheckToFromRequired checks for changes to or from a required property
func CheckToFromRequired(required1, required2 bool) (diffs []TypeDiff) {
	if required1 != required2 {
		code := ChangedOptionalToRequired
		if required1 {
			code = ChangedRequiredToOptional
		}
		diffs = addTypeDiff(diffs, TypeDiff{Change: code})
	}
	return diffs
}

const objType = "object"

func getTypeHierarchyChange(type1, type2 string) TypeDiff {
	fromType := type1
	if fromType == "" {
		fromType = objType
	}
	toType := type2
	if toType == "" {
		toType = objType
	}
	diffDescription := fmt.Sprintf("%s -> %s", fromType, toType)
	if isStringType(type1) && !isStringType(type2) {
		return TypeDiff{Change: NarrowedType, Description: diffDescription}
	}
	if !isStringType(type1) && isStringType(type2) {
		return TypeDiff{Change: WidenedType, Description: diffDescription}
	}
	type1Wideness, type1IsNumeric := numberWideness[type1]
	type2Wideness, type2IsNumeric := numberWideness[type2]
	if type1IsNumeric && type2IsNumeric {
		if type1Wideness == type2Wideness {
			return TypeDiff{Change: ChangedToCompatibleType, Description: diffDescription}
		}
		if type1Wideness > type2Wideness {
			return TypeDiff{Change: NarrowedType, Description: diffDescription}
		}
		if type1Wideness < type2Wideness {
			return TypeDiff{Change: WidenedType, Description: diffDescription}
		}
	}
	return TypeDiff{Change: ChangedType, Description: diffDescription}
}

func isRefType(item interface{}) bool {
	switch s := item.(type) {
	case spec.Refable:
		return s.Ref.String() != ""
	case *spec.Schema:
		return s.Ref.String() != ""
	case *spec.SchemaProps:
		return s.Ref.String() != ""
	case *spec.SimpleSchema:
		return false
	default:
		return false
	}
}
