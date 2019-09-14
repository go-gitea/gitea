package diff

import (
	"fmt"
	"strings"

	"github.com/go-openapi/spec"
)

const StringType = "string"

// URLMethodResponse encapsulates these three elements to act as a map key
type URLMethodResponse struct {
	Path     string `json:"path"`
	Method   string `json:"method"`
	Response string `json:"response"`
}

// MarshalText - for serializing as a map key
func (p URLMethod) MarshalText() (text []byte, err error) {
	return []byte(fmt.Sprintf("%s %s", p.Path, p.Method)), nil
}

// URLMethods allows iteration of endpoints based on url and method
type URLMethods map[URLMethod]*PathItemOp

// SpecAnalyser contains all the differences for a Spec
type SpecAnalyser struct {
	Diffs                      SpecDifferences
	urlMethods1                URLMethods
	urlMethods2                URLMethods
	Definitions1               spec.Definitions
	Definitions2               spec.Definitions
	AlreadyComparedDefinitions map[string]bool
}

// NewSpecAnalyser returns an empty SpecDiffs
func NewSpecAnalyser() *SpecAnalyser {
	return &SpecAnalyser{
		Diffs: SpecDifferences{},
	}
}

// Analyse the differences in two specs
func (sd *SpecAnalyser) Analyse(spec1, spec2 *spec.Swagger) error {
	sd.Definitions1 = spec1.Definitions
	sd.Definitions2 = spec2.Definitions
	sd.urlMethods1 = getURLMethodsFor(spec1)
	sd.urlMethods2 = getURLMethodsFor(spec2)

	sd.analyseSpecMetadata(spec1, spec2)
	sd.analyseEndpoints()
	sd.analyseParams()
	sd.analyseEndpointData()
	sd.analyseResponseParams()

	return nil
}

func (sd *SpecAnalyser) analyseSpecMetadata(spec1, spec2 *spec.Swagger) {
	// breaking if it no longer consumes any formats
	added, deleted, _ := FromStringArray(spec1.Consumes).DiffsTo(spec2.Consumes)

	node := getNameOnlyDiffNode("Spec")
	location := DifferenceLocation{Node: node}
	consumesLoation := location.AddNode(getNameOnlyDiffNode("consumes"))

	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(
			SpecDifference{DifferenceLocation: consumesLoation, Code: AddedConsumesFormat, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: consumesLoation, Code: DeletedConsumesFormat, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// // breaking if it no longer produces any formats
	added, deleted, _ = FromStringArray(spec1.Produces).DiffsTo(spec2.Produces)
	producesLocation := location.AddNode(getNameOnlyDiffNode("produces"))
	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: producesLocation, Code: AddedProducesFormat, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: producesLocation, Code: DeletedProducesFormat, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// // breaking if it no longer supports a scheme
	added, deleted, _ = FromStringArray(spec1.Schemes).DiffsTo(spec2.Schemes)
	schemesLocation := location.AddNode(getNameOnlyDiffNode("schemes"))

	for _, eachAdded := range added {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: schemesLocation, Code: AddedSchemes, Compatibility: NonBreaking, DiffInfo: eachAdded})
	}
	for _, eachDeleted := range deleted {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: schemesLocation, Code: DeletedSchemes, Compatibility: Breaking, DiffInfo: eachDeleted})
	}

	// // host should be able to change without any issues?
	sd.analyseMetaDataProperty(spec1.Info.Description, spec2.Info.Description, ChangedDescripton, NonBreaking)

	// // host should be able to change without any issues?
	sd.analyseMetaDataProperty(spec1.Host, spec2.Host, ChangedHostURL, Breaking)
	// sd.Host = compareStrings(spec1.Host, spec2.Host)

	// // Base Path change will break non generated clients
	sd.analyseMetaDataProperty(spec1.BasePath, spec2.BasePath, ChangedBasePath, Breaking)

	// TODO: what to do about security?
	// Missing security scheme will break a client
	// Security            []map[string][]string  `json:"security,omitempty"`
	// Tags                []Tag                  `json:"tags,omitempty"`
	// ExternalDocs        *ExternalDocumentation `json:"externalDocs,omitempty"`
}

func (sd *SpecAnalyser) analyseEndpoints() {
	sd.findDeletedEndpoints()
	sd.findAddedEndpoints()
}

func (sd *SpecAnalyser) analyseEndpointData() {

	for URLMethod, op2 := range sd.urlMethods2 {
		if op1, ok := sd.urlMethods1[URLMethod]; ok {
			addedTags, deletedTags, _ := FromStringArray(op1.Operation.Tags).DiffsTo(op2.Operation.Tags)
			location := DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method}

			for _, eachAddedTag := range addedTags {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: AddedTag, DiffInfo: eachAddedTag})
			}
			for _, eachDeletedTag := range deletedTags {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: DeletedTag, DiffInfo: eachDeletedTag})
			}

			sd.compareDescripton(location, op1.Operation.Description, op2.Operation.Description)

		}
	}

}

func (sd *SpecAnalyser) analyseParams() {
	locations := []string{"query", "path", "body", "header"}

	for _, paramLocation := range locations {
		rootNode := getNameOnlyDiffNode(strings.Title(paramLocation))
		for URLMethod, op2 := range sd.urlMethods2 {
			if op1, ok := sd.urlMethods1[URLMethod]; ok {

				params1 := getParams(op1.ParentPathItem.Parameters, op1.Operation.Parameters, paramLocation)
				params2 := getParams(op2.ParentPathItem.Parameters, op2.Operation.Parameters, paramLocation)

				location := DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method, Node: rootNode}

				// detect deleted params
				for paramName1, param1 := range params1 {
					if _, ok := params2[paramName1]; !ok {
						childLocation := location.AddNode(getSchemaDiffNode(paramName1, param1.Schema))
						code := DeletedOptionalParam
						if param1.Required {
							code = DeletedRequiredParam
						}
						sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLocation, Code: code})
					}
				}
				// detect added changed params
				for paramName2, param2 := range params2 {
					//changed?
					if param1, ok := params1[paramName2]; ok {
						sd.compareParams(URLMethod, paramLocation, paramName2, param1, param2)
					} else {
						// Added
						childLocation := location.AddNode(getSchemaDiffNode(paramName2, param2.Schema))
						code := AddedOptionalParam
						if param2.Required {
							code = AddedRequiredParam
						}
						sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLocation, Code: code})
					}
				}
			}
		}
	}
}

func (sd *SpecAnalyser) analyseResponseParams() {
	// Loop through url+methods in spec 2 - check deleted and changed
	for URLMethod2, op2 := range sd.urlMethods2 {
		if op1, ok := sd.urlMethods1[URLMethod2]; ok {
			// compare responses for url and method
			op1Responses := op1.Operation.Responses.StatusCodeResponses
			op2Responses := op2.Operation.Responses.StatusCodeResponses

			// deleted responses
			for code1 := range op1Responses {
				if _, ok := op2Responses[code1]; !ok {
					location := DifferenceLocation{URL: URLMethod2.Path, Method: URLMethod2.Method, Response: code1}
					sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: DeletedResponse})
				}
			}
			// Added updated Response Codes
			for code2, op2Response := range op2Responses {

				if op1Response, ok := op1Responses[code2]; ok {
					op1Headers := op1Response.ResponseProps.Headers
					headerRootNode := getNameOnlyDiffNode("Headers")
					location := DifferenceLocation{URL: URLMethod2.Path, Method: URLMethod2.Method, Response: code2, Node: headerRootNode}

					// Iterate Spec2 Headers looking for added and updated
					for op2HeaderName, op2Header := range op2Response.ResponseProps.Headers {
						if op1Header, ok := op1Headers[op2HeaderName]; ok {
							sd.compareSimpleSchema(location.AddNode(getNameOnlyDiffNode(op2HeaderName)),
								&op1Header.SimpleSchema,
								&op2Header.SimpleSchema, false, false)
						} else {
							sd.Diffs = sd.Diffs.addDiff(SpecDifference{
								DifferenceLocation: location.AddNode(getNameOnlyDiffNode(op2HeaderName)),
								Code:               AddedResponseHeader})
						}
					}
					for op1HeaderName := range op1Response.ResponseProps.Headers {
						if _, ok := op2Response.ResponseProps.Headers[op1HeaderName]; !ok {
							sd.Diffs = sd.Diffs.addDiff(SpecDifference{
								DifferenceLocation: location.AddNode(getNameOnlyDiffNode(op1HeaderName)),
								Code:               DeletedResponseHeader})
						}
					}
					responseLocation := DifferenceLocation{URL: URLMethod2.Path, Method: URLMethod2.Method, Response: code2}
					sd.compareDescripton(responseLocation, op1Response.Description, op2Response.Description)

					if op1Response.Schema != nil {
						sd.compareSchema(
							DifferenceLocation{URL: URLMethod2.Path, Method: URLMethod2.Method, Response: code2},
							op1Response.Schema,
							op2Response.Schema, true, true)
					}
				} else {
					sd.Diffs = sd.Diffs.addDiff(SpecDifference{
						DifferenceLocation: DifferenceLocation{URL: URLMethod2.Path, Method: URLMethod2.Method, Response: code2},
						Code:               AddedResponse})
				}
			}
		}
	}
}

func addTypeDiff(diffs []TypeDiff, diff TypeDiff) []TypeDiff {
	if diff.Change != NoChangeDetected {
		diffs = append(diffs, diff)
	}
	return diffs
}

// CheckToFromPrimitiveType check for diff to or from a primitive
func (sd *SpecAnalyser) CheckToFromPrimitiveType(diffs []TypeDiff, type1, type2 spec.SchemaProps) []TypeDiff {

	type1IsPrimitive := len(type1.Type) > 0
	type2IsPrimitive := len(type2.Type) > 0

	// Primitive to Obj or Obj to Primitive
	if type1IsPrimitive && !type2IsPrimitive {
		return addTypeDiff(diffs, TypeDiff{Change: ChangedType, FromType: type1.Type[0], ToType: "obj"})
	}

	if !type1IsPrimitive && type2IsPrimitive {
		return addTypeDiff(diffs, TypeDiff{Change: ChangedType, FromType: type2.Type[0], ToType: "obj"})
	}

	return diffs
}

// CheckToFromArrayType check for changes to or from an Array type
func (sd *SpecAnalyser) CheckToFromArrayType(diffs []TypeDiff, type1, type2 spec.SchemaProps) []TypeDiff {
	// Single to Array or Array to Single
	type1Array := type1.Type[0] == ArrayType
	type2Array := type2.Type[0] == ArrayType

	if type1Array && !type2Array {
		return addTypeDiff(diffs, TypeDiff{Change: ChangedType, FromType: "obj", ToType: type2.Type[0]})
	}

	if !type1Array && type2Array {
		return addTypeDiff(diffs, TypeDiff{Change: ChangedType, FromType: type1.Type[0], ToType: ArrayType})
	}

	if type1Array && type2Array {
		// array
		// TODO: Items??
		diffs = addTypeDiff(diffs, compareIntValues("MaxItems", type1.MaxItems, type2.MaxItems, WidenedType, NarrowedType))
		diffs = addTypeDiff(diffs, compareIntValues("MinItems", type1.MinItems, type2.MinItems, NarrowedType, WidenedType))

	}
	return diffs
}

// CheckStringTypeChanges checks for changes to or from a string type
func (sd *SpecAnalyser) CheckStringTypeChanges(diffs []TypeDiff, type1, type2 spec.SchemaProps) []TypeDiff {
	// string changes
	if type1.Type[0] == StringType &&
		type2.Type[0] == StringType {
		diffs = addTypeDiff(diffs, compareIntValues("MinLength", type1.MinLength, type2.MinLength, NarrowedType, WidenedType))
		diffs = addTypeDiff(diffs, compareIntValues("MaxLength", type1.MinLength, type2.MinLength, WidenedType, NarrowedType))
		if type1.Pattern != type2.Pattern {
			diffs = addTypeDiff(diffs, TypeDiff{Change: ChangedType, Description: fmt.Sprintf("Pattern Changed:%s->%s", type1.Pattern, type2.Pattern)})
		}
		if type1.Type[0] == StringType {
			if len(type1.Enum) > 0 {
				enumDiffs := sd.compareEnums(type1.Enum, type2.Enum)
				diffs = append(diffs, enumDiffs...)
			}
		}
	}
	return diffs
}

// CheckNumericTypeChanges checks for changes to or from a numeric type
func (sd *SpecAnalyser) CheckNumericTypeChanges(diffs []TypeDiff, type1, type2 spec.SchemaProps) []TypeDiff {
	// Number
	_, type1IsNumeric := numberWideness[type1.Type[0]]
	_, type2IsNumeric := numberWideness[type2.Type[0]]

	if type1IsNumeric && type2IsNumeric {
		diffs = addTypeDiff(diffs, compareFloatValues("Maximum", type1.Maximum, type2.Maximum, WidenedType, NarrowedType))
		diffs = addTypeDiff(diffs, compareFloatValues("Minimum", type1.Minimum, type2.Minimum, NarrowedType, WidenedType))
		if type1.ExclusiveMaximum && !type2.ExclusiveMaximum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: WidenedType, Description: fmt.Sprintf("Exclusive Maximum Removed:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
		}
		if !type1.ExclusiveMaximum && type2.ExclusiveMaximum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: NarrowedType, Description: fmt.Sprintf("Exclusive Maximum Added:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
		}
		if type1.ExclusiveMinimum && !type2.ExclusiveMinimum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: WidenedType, Description: fmt.Sprintf("Exclusive Minimum Removed:%v->%v", type1.ExclusiveMaximum, type2.ExclusiveMaximum)})
		}
		if !type1.ExclusiveMinimum && type2.ExclusiveMinimum {
			diffs = addTypeDiff(diffs, TypeDiff{Change: NarrowedType, Description: fmt.Sprintf("Exclusive Minimum Added:%v->%v", type1.ExclusiveMinimum, type2.ExclusiveMinimum)})
		}
	}
	return diffs
}

// CompareTypes computes type specific property diffs
func (sd *SpecAnalyser) CompareTypes(type1, type2 spec.SchemaProps) []TypeDiff {

	diffs := []TypeDiff{}

	diffs = sd.CheckToFromPrimitiveType(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	diffs = sd.CheckToFromArrayType(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	// check type hierarchy change eg string -> integer = NarrowedChange
	//Type
	//Format
	if type1.Type[0] != type2.Type[0] ||
		type1.Format != type2.Format {
		diff := getTypeHierarchyChange(primitiveTypeString(type1.Type[0], type1.Format), primitiveTypeString(type2.Type[0], type2.Format))
		diffs = addTypeDiff(diffs, diff)
	}

	diffs = sd.CheckStringTypeChanges(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	diffs = sd.CheckNumericTypeChanges(diffs, type1, type2)

	if len(diffs) > 0 {
		return diffs
	}

	return diffs
}

func (sd *SpecAnalyser) compareParams(urlMethod URLMethod, location string, name string, param1, param2 spec.Parameter) {
	diffLocation := DifferenceLocation{URL: urlMethod.Path, Method: urlMethod.Method}

	childLocation := diffLocation.AddNode(getNameOnlyDiffNode(strings.Title(location)))
	paramLocation := diffLocation.AddNode(getNameOnlyDiffNode(name))
	sd.compareDescripton(paramLocation, param1.Description, param2.Description)

	if param1.Schema != nil && param2.Schema != nil {
		childLocation = childLocation.AddNode(getSchemaDiffNode(name, param2.Schema))
		sd.compareSchema(childLocation, param1.Schema, param2.Schema, param1.Required, param2.Required)
	}
	diffs := sd.CompareTypes(forParam(param1), forParam(param2))

	childLocation = childLocation.AddNode(getSchemaDiffNode(name, param2.Schema))
	for _, eachDiff := range diffs {
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{
			DifferenceLocation: childLocation,
			Code:               eachDiff.Change,
			DiffInfo:           eachDiff.Description})
	}
	if param1.Required != param2.Required {
		code := ChangedRequiredToOptionalParam
		if param2.Required {
			code = ChangedOptionalToRequiredParam
		}
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLocation, Code: code})
	}
}

func (sd *SpecAnalyser) compareSimpleSchema(location DifferenceLocation, schema1, schema2 *spec.SimpleSchema, required1, required2 bool) {
	if schema1 == nil || schema2 == nil {
		return
	}

	if schema1.Type == ArrayType {
		refSchema1 := schema1.Items.SimpleSchema
		refSchema2 := schema2.Items.SimpleSchema

		childLocation := location.AddNode(getSimpleSchemaDiffNode("", schema1))
		sd.compareSimpleSchema(childLocation, &refSchema1, &refSchema2, required1, required2)
		return
	}
	if required1 != required2 {
		code := AddedRequiredProperty
		if required1 {
			code = ChangedRequiredToOptional

		}
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: code})
	}

}

func (sd *SpecAnalyser) compareDescripton(location DifferenceLocation, desc1, desc2 string) {
	if desc1 != desc2 {
		code := ChangedDescripton
		if len(desc1) > 0 {
			code = DeletedDescripton
		} else if len(desc2) > 0 {
			code = AddedDescripton
		}
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: code})
	}

}

func (sd *SpecAnalyser) compareSchema(location DifferenceLocation, schema1, schema2 *spec.Schema, required1, required2 bool) {

	if schema1 == nil || schema2 == nil {
		return
	}

	sd.compareDescripton(location, schema1.Description, schema2.Description)

	if len(schema1.Type) == 0 {
		refSchema1, definition1 := sd.schemaFromRef(schema1, &sd.Definitions1)
		refSchema2, definition2 := sd.schemaFromRef(schema2, &sd.Definitions2)

		if len(definition1) > 0 {
			info := fmt.Sprintf("[%s -> %s]", definition1, definition2)

			if definition1 != definition2 {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location,
					Code:     ChangedType,
					DiffInfo: info,
				})
			}
			sd.compareSchema(location, refSchema1, refSchema2, required1, required2)
			return
		}
	} else {
		if schema1.Type[0] == ArrayType {
			refSchema1, definition1 := sd.schemaFromRef(schema1.Items.Schema, &sd.Definitions1)
			refSchema2, _ := sd.schemaFromRef(schema2.Items.Schema, &sd.Definitions2)

			if len(definition1) > 0 {
				childLocation := location.AddNode(getSchemaDiffNode("", schema1))
				sd.compareSchema(childLocation, refSchema1, refSchema2, required1, required2)
				return
			}

		}
		diffs := sd.CompareTypes(schema1.SchemaProps, schema2.SchemaProps)

		for _, eachTypeDiff := range diffs {
			if eachTypeDiff.Change != NoChangeDetected {
				sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: eachTypeDiff.Change, DiffInfo: eachTypeDiff.Description})
			}
		}
	}

	if required1 != required2 {
		code := AddedRequiredProperty
		if required1 {
			code = ChangedRequiredToOptional

		}
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: location, Code: code})
	}
	requiredProps2 := sliceToStrMap(schema2.Required)
	requiredProps1 := sliceToStrMap(schema1.Required)
	schema1Props := sd.propertiesFor(schema1, &sd.Definitions1)
	schema2Props := sd.propertiesFor(schema2, &sd.Definitions2)
	// find deleted and changed properties
	for eachProp1Name, eachProp1 := range schema1Props {
		eachProp1 := eachProp1
		_, required1 := requiredProps1[eachProp1Name]
		_, required2 := requiredProps2[eachProp1Name]
		childLoc := sd.addChildDiffNode(location, eachProp1Name, &eachProp1)

		if eachProp2, ok := schema2Props[eachProp1Name]; ok {
			sd.compareSchema(childLoc, &eachProp1, &eachProp2, required1, required2)
			sd.compareDescripton(childLoc, eachProp1.Description, eachProp2.Description)
		} else {
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLoc, Code: DeletedProperty})
		}
	}

	// find added properties
	for eachProp2Name, eachProp2 := range schema2.Properties {
		eachProp2 := eachProp2
		if _, ok := schema1.Properties[eachProp2Name]; !ok {
			childLoc := sd.addChildDiffNode(location, eachProp2Name, &eachProp2)
			_, required2 := requiredProps2[eachProp2Name]
			code := AddedProperty
			if required2 {
				code = AddedRequiredProperty
			}
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: childLoc, Code: code})
		}
	}
}

func (sd *SpecAnalyser) addChildDiffNode(location DifferenceLocation, propName string, propSchema *spec.Schema) DifferenceLocation {
	newLoc := location
	if newLoc.Node != nil {
		newLoc.Node = newLoc.Node.Copy()
	}

	childNode := sd.fromSchemaProps(propName, &propSchema.SchemaProps)
	if newLoc.Node != nil {
		newLoc.Node.AddLeafNode(&childNode)
	} else {
		newLoc.Node = &childNode
	}
	return newLoc
}

func (sd *SpecAnalyser) fromSchemaProps(fieldName string, props *spec.SchemaProps) Node {
	node := Node{}
	node.IsArray = props.Type[0] == ArrayType
	if !node.IsArray {
		node.TypeName = props.Type[0]
	}
	node.Field = fieldName
	return node
}

func (sd *SpecAnalyser) compareEnums(left, right []interface{}) []TypeDiff {
	diffs := []TypeDiff{}

	leftStrs := []string{}
	rightStrs := []string{}
	for _, eachLeft := range left {
		leftStrs = append(leftStrs, fmt.Sprintf("%v", eachLeft))
	}
	for _, eachRight := range right {
		rightStrs = append(rightStrs, fmt.Sprintf("%v", eachRight))
	}
	added, deleted, _ := FromStringArray(leftStrs).DiffsTo(rightStrs)
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

func (sd *SpecAnalyser) findAddedEndpoints() {
	for URLMethod := range sd.urlMethods2 {
		if _, ok := sd.urlMethods1[URLMethod]; !ok {
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{URL: URLMethod.Path, Method: URLMethod.Method}, Code: AddedEndpoint})
		}
	}
}

func (sd *SpecAnalyser) findDeletedEndpoints() {
	for eachURLMethod, operation1 := range sd.urlMethods1 {
		code := DeletedEndpoint
		if (operation1.ParentPathItem.Options != nil && operation1.ParentPathItem.Options.Deprecated) ||
			(operation1.Operation.Deprecated) {
			code = DeletedDeprecatedEndpoint
		}
		if _, ok := sd.urlMethods2[eachURLMethod]; !ok {
			sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{URL: eachURLMethod.Path, Method: eachURLMethod.Method}, Code: code})
		}
	}
}

func (sd *SpecAnalyser) analyseMetaDataProperty(item1, item2 string, codeIfDiff SpecChangeCode, compatIfDiff Compatibility) {
	if item1 != item2 {
		diffSpec := fmt.Sprintf("%s -> %s", item1, item2)
		sd.Diffs = sd.Diffs.addDiff(SpecDifference{DifferenceLocation: DifferenceLocation{Node: &Node{Field: "Spec Metadata"}}, Code: codeIfDiff, Compatibility: compatIfDiff, DiffInfo: diffSpec})
	}
}

func (sd *SpecAnalyser) schemaFromRef(schema *spec.Schema, defns *spec.Definitions) (actualSchema *spec.Schema, definitionName string) {
	ref := schema.Ref
	url := ref.GetURL()
	if url == nil {
		return schema, ""
	}
	fragmentParts := strings.Split(url.Fragment, "/")
	numParts := len(fragmentParts)
	if numParts == 0 {
		return schema, ""
	}

	definitionName = fragmentParts[numParts-1]
	foundSchema, ok := (*defns)[definitionName]
	if !ok {
		return nil, definitionName
	}
	actualSchema = &foundSchema
	return

}

func (sd *SpecAnalyser) propertiesFor(schema *spec.Schema, defns *spec.Definitions) map[string]spec.Schema {
	schemaFromRef, _ := sd.schemaFromRef(schema, defns)
	schema = schemaFromRef
	props := map[string]spec.Schema{}

	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			prop := prop
			eachProp, _ := sd.schemaFromRef(&prop, defns)
			props[name] = *eachProp
		}
	}
	for _, eachAllOf := range schema.AllOf {
		eachAllOf := eachAllOf
		eachAllOfActual, _ := sd.schemaFromRef(&eachAllOf, defns)
		for name, prop := range eachAllOfActual.Properties {
			prop := prop
			eachProp, _ := sd.schemaFromRef(&prop, defns)
			props[name] = *eachProp
		}
	}
	return props
}
