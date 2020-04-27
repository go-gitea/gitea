package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SpecChangeCode enumerates the various types of diffs from one spec to another
type SpecChangeCode int

const (
	// NoChangeDetected - the specs have no changes
	NoChangeDetected SpecChangeCode = iota
	// DeletedProperty - A message property has been deleted in the new spec
	DeletedProperty
	// AddedProperty - A message property has been added in the new spec
	AddedProperty
	// AddedRequiredProperty - A required message property has been added in the new spec
	AddedRequiredProperty
	// DeletedOptionalParam - An endpoint parameter has been deleted in the new spec
	DeletedOptionalParam
	// ChangedDescripton - Changed a description
	ChangedDescripton
	// AddedDescripton - Added a description
	AddedDescripton
	// DeletedDescripton - Deleted a description
	DeletedDescripton
	// ChangedTag - Changed a tag
	ChangedTag
	// AddedTag - Added a tag
	AddedTag
	// DeletedTag - Deleted a tag
	DeletedTag
	// DeletedResponse - An endpoint response has been deleted in the new spec
	DeletedResponse
	// DeletedEndpoint - An endpoint has been deleted in the new spec
	DeletedEndpoint
	// DeletedDeprecatedEndpoint - A deprecated endpoint has been deleted in the new spec
	DeletedDeprecatedEndpoint
	// AddedRequiredParam - A required parameter has been added in the new spec
	AddedRequiredParam
	// DeletedRequiredParam - A required parameter has been deleted in the new spec
	DeletedRequiredParam
	// ChangedRequiredToOptional - A required parameter has been made optional in the new spec
	ChangedRequiredToOptional
	// AddedEndpoint - An endpoint has been added in the new spec
	AddedEndpoint
	// WidenedType - An type has been changed to a more permissive type eg int->string
	WidenedType
	// NarrowedType - An type has been changed to a less permissive type eg string->int
	NarrowedType
	// ChangedToCompatibleType - An type has been changed to a compatible type eg password->string
	ChangedToCompatibleType
	// ChangedType - An type has been changed to a type whose relative compatibility cannot be determined
	ChangedType
	// AddedEnumValue - An enum type has had a new potential value added to it
	AddedEnumValue
	// DeletedEnumValue - An enum type has had a existing value removed from it
	DeletedEnumValue
	// AddedOptionalParam - A new optional parameter has been added to the new spec
	AddedOptionalParam
	// ChangedOptionalToRequiredParam - An optional parameter is now required in the new spec
	ChangedOptionalToRequiredParam
	// ChangedRequiredToOptionalParam - An required parameter is now optional in the new spec
	ChangedRequiredToOptionalParam
	// AddedResponse An endpoint has new response code in the new spec
	AddedResponse
	// AddedConsumesFormat - a new consumes format (json/xml/yaml etc) has been added in the new spec
	AddedConsumesFormat
	// DeletedConsumesFormat - an existing format has been removed in the new spec
	DeletedConsumesFormat
	// AddedProducesFormat - a new produces format (json/xml/yaml etc) has been added in the new spec
	AddedProducesFormat
	// DeletedProducesFormat - an existing produces format has been removed in the new spec
	DeletedProducesFormat
	// AddedSchemes - a new scheme has been added to the new spec
	AddedSchemes
	// DeletedSchemes - a scheme has been removed from the new spec
	DeletedSchemes
	// ChangedHostURL - the host url has been changed. If this is used in the client generation, then clients will break.
	ChangedHostURL
	// ChangedBasePath - the host base path has been changed. If this is used in the client generation, then clients will break.
	ChangedBasePath
	// AddedResponseHeader Added a header Item
	AddedResponseHeader
	// ChangedResponseHeader Added a header Item
	ChangedResponseHeader
	// DeletedResponseHeader Added a header Item
	DeletedResponseHeader
)

var toLongStringSpecChangeCode = map[SpecChangeCode]string{
	NoChangeDetected:               "No Change detected",
	AddedEndpoint:                  "Added endpoint",
	DeletedEndpoint:                "Deleted endpoint",
	DeletedDeprecatedEndpoint:      "Deleted a deprecated endpoint",
	AddedRequiredProperty:          "Added required property",
	DeletedProperty:                "Deleted property",
	ChangedDescripton:              "Changed a description",
	AddedDescripton:                "Added a description",
	DeletedDescripton:              "Deleted a description",
	ChangedTag:                     "Changed a tag",
	AddedTag:                       "Added a tag",
	DeletedTag:                     "Deleted a tag",
	AddedProperty:                  "Added property",
	AddedOptionalParam:             "Added optional param",
	AddedRequiredParam:             "Added required param",
	DeletedOptionalParam:           "Deleted optional param",
	DeletedRequiredParam:           "Deleted required param",
	DeletedResponse:                "Deleted response",
	AddedResponse:                  "Added response",
	WidenedType:                    "Widened type",
	NarrowedType:                   "Narrowed type",
	ChangedType:                    "Changed type",
	ChangedToCompatibleType:        "Changed type to equivalent type",
	ChangedOptionalToRequiredParam: "Changed optional param to required",
	ChangedRequiredToOptionalParam: "Changed required param to optional",
	AddedEnumValue:                 "Added possible enumeration(s)",
	DeletedEnumValue:               "Deleted possible enumeration(s)",
	AddedConsumesFormat:            "Added a consumes format",
	DeletedConsumesFormat:          "Deleted a consumes format",
	AddedProducesFormat:            "Added produces format",
	DeletedProducesFormat:          "Deleted produces format",
	AddedSchemes:                   "Added schemes",
	DeletedSchemes:                 "Deleted schemes",
	ChangedHostURL:                 "Changed host URL",
	ChangedBasePath:                "Changed base path",
	AddedResponseHeader:            "Added response header",
	ChangedResponseHeader:          "Changed response header",
	DeletedResponseHeader:          "Deleted response header",
}

var toStringSpecChangeCode = map[SpecChangeCode]string{
	AddedEndpoint:                  "AddedEndpoint",
	NoChangeDetected:               "NoChangeDetected",
	DeletedEndpoint:                "DeletedEndpoint",
	DeletedDeprecatedEndpoint:      "DeletedDeprecatedEndpoint",
	AddedRequiredProperty:          "AddedRequiredProperty",
	DeletedProperty:                "DeletedProperty",
	AddedProperty:                  "AddedProperty",
	ChangedDescripton:              "ChangedDescription",
	AddedDescripton:                "AddedDescription",
	DeletedDescripton:              "DeletedDescription",
	ChangedTag:                     "ChangedTag",
	AddedTag:                       "AddedTag",
	DeletedTag:                     "DeletedTag",
	AddedOptionalParam:             "AddedOptionalParam",
	AddedRequiredParam:             "AddedRequiredParam",
	DeletedOptionalParam:           "DeletedRequiredParam",
	DeletedRequiredParam:           "Deleted required param",
	DeletedResponse:                "DeletedResponse",
	AddedResponse:                  "AddedResponse",
	WidenedType:                    "WidenedType",
	NarrowedType:                   "NarrowedType",
	ChangedType:                    "ChangedType",
	ChangedToCompatibleType:        "ChangedToCompatibleType",
	ChangedOptionalToRequiredParam: "ChangedOptionalToRequiredParam",
	ChangedRequiredToOptionalParam: "ChangedRequiredToOptionalParam",
	AddedEnumValue:                 "AddedEnumValue",
	DeletedEnumValue:               "DeletedEnumValue",
	AddedConsumesFormat:            "AddedConsumesFormat",
	DeletedConsumesFormat:          "DeletedConsumesFormat",
	AddedProducesFormat:            "AddedProducesFormat",
	DeletedProducesFormat:          "DeletedProducesFormat",
	AddedSchemes:                   "AddedSchemes",
	DeletedSchemes:                 "DeletedSchemes",
	ChangedHostURL:                 "ChangedHostURL",
	ChangedBasePath:                "ChangedBasePath",
	AddedResponseHeader:            "AddedResponseHeader",
	ChangedResponseHeader:          "ChangedResponseHeader",
	DeletedResponseHeader:          "DeletedResponseHeader",
}

var toIDSpecChangeCode = map[string]SpecChangeCode{}

// Description returns an english version of this error
func (s *SpecChangeCode) Description() (result string) {
	result, ok := toLongStringSpecChangeCode[*s]
	if !ok {
		fmt.Printf("WARNING: No description for %v", *s)
		result = "UNDEFINED"
	}
	return
}

// MarshalJSON marshals the enum as a quoted json string
func (s *SpecChangeCode) MarshalJSON() ([]byte, error) {
	return stringAsQuotedBytes(toStringSpecChangeCode[*s])
}

// UnmarshalJSON unmashalls a quoted json string to the enum value
func (s *SpecChangeCode) UnmarshalJSON(b []byte) error {
	str, err := readStringFromByteStream(b)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will return an error to the caller.
	val, ok := toIDSpecChangeCode[str]

	if ok {
		*s = val
	} else {
		return fmt.Errorf("unknown enum value. cannot unmarshal '%s'", str)
	}
	return nil
}

// Compatibility - whether this is a breaking or non-breaking change
type Compatibility int

const (
	// Breaking this change could break existing clients
	Breaking Compatibility = iota
	// NonBreaking This is a backwards-compatible API change
	NonBreaking
)

func (s Compatibility) String() string {
	return toStringCompatibility[s]
}

var toStringCompatibility = map[Compatibility]string{
	Breaking:    "Breaking",
	NonBreaking: "NonBreaking",
}

var toIDCompatibility = map[string]Compatibility{}

// MarshalJSON marshals the enum as a quoted json string
func (s *Compatibility) MarshalJSON() ([]byte, error) {
	return stringAsQuotedBytes(toStringCompatibility[*s])
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (s *Compatibility) UnmarshalJSON(b []byte) error {
	str, err := readStringFromByteStream(b)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will return an error to the caller.
	val, ok := toIDCompatibility[str]

	if ok {
		*s = val
	} else {
		return fmt.Errorf("unknown enum value. cannot unmarshal '%s'", str)
	}
	return nil
}

func stringAsQuotedBytes(str string) ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(str)
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func readStringFromByteStream(b []byte) (string, error) {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return "", err
	}
	return j, nil
}

func init() {
	for key, val := range toStringSpecChangeCode {
		toIDSpecChangeCode[val] = key
	}
	for key, val := range toStringCompatibility {
		toIDCompatibility[val] = key
	}

}
