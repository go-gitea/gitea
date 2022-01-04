package diff

// CompatibilityPolicy decides which changes are breaking and which are not
type CompatibilityPolicy struct {
	ForResponse map[SpecChangeCode]Compatibility
	ForRequest  map[SpecChangeCode]Compatibility
	ForChange   map[SpecChangeCode]Compatibility
}

var compatibility CompatibilityPolicy

func init() {
	compatibility = CompatibilityPolicy{
		ForResponse: map[SpecChangeCode]Compatibility{
			AddedRequiredProperty:   Breaking,
			DeletedProperty:         Breaking,
			AddedProperty:           NonBreaking,
			DeletedResponse:         Breaking,
			AddedResponse:           NonBreaking,
			WidenedType:             NonBreaking,
			NarrowedType:            NonBreaking,
			ChangedType:             Breaking,
			ChangedToCompatibleType: NonBreaking,
			AddedEnumValue:          Breaking,
			DeletedEnumValue:        NonBreaking,
			AddedResponseHeader:     NonBreaking,
			ChangedResponseHeader:   Breaking,
			DeletedResponseHeader:   Breaking,
			ChangedDescripton:       NonBreaking,
			AddedDescripton:         NonBreaking,
			DeletedDescripton:       NonBreaking,
			ChangedTag:              NonBreaking,
			AddedTag:                NonBreaking,
			DeletedTag:              NonBreaking,
			DeletedConstraint:       Breaking,
			AddedConstraint:         NonBreaking,
		},
		ForRequest: map[SpecChangeCode]Compatibility{
			AddedRequiredProperty:     Breaking,
			DeletedProperty:           Breaking,
			AddedProperty:             Breaking,
			AddedOptionalParam:        NonBreaking,
			AddedRequiredParam:        Breaking,
			DeletedOptionalParam:      NonBreaking,
			DeletedRequiredParam:      NonBreaking,
			WidenedType:               NonBreaking,
			NarrowedType:              Breaking,
			ChangedType:               Breaking,
			ChangedToCompatibleType:   NonBreaking,
			ChangedOptionalToRequired: Breaking,
			ChangedRequiredToOptional: NonBreaking,
			AddedEnumValue:            NonBreaking,
			DeletedEnumValue:          Breaking,
			ChangedDescripton:         NonBreaking,
			AddedDescripton:           NonBreaking,
			DeletedDescripton:         NonBreaking,
			ChangedTag:                NonBreaking,
			AddedTag:                  NonBreaking,
			DeletedTag:                NonBreaking,
			DeletedConstraint:         NonBreaking,
			AddedConstraint:           Breaking,
		},
		ForChange: map[SpecChangeCode]Compatibility{
			NoChangeDetected:          NonBreaking,
			AddedEndpoint:             NonBreaking,
			DeletedEndpoint:           Breaking,
			DeletedDeprecatedEndpoint: NonBreaking,
			AddedConsumesFormat:       NonBreaking,
			DeletedConsumesFormat:     Breaking,
			AddedProducesFormat:       NonBreaking,
			DeletedProducesFormat:     Breaking,
			AddedSchemes:              NonBreaking,
			DeletedSchemes:            Breaking,
			ChangedHostURL:            Breaking,
			ChangedBasePath:           Breaking,
			ChangedDescripton:         NonBreaking,
			AddedDescripton:           NonBreaking,
			DeletedDescripton:         NonBreaking,
			ChangedTag:                NonBreaking,
			AddedTag:                  NonBreaking,
			DeletedTag:                NonBreaking,
			RefTargetChanged:          Breaking,
			RefTargetRenamed:          NonBreaking,
			AddedDefinition:           NonBreaking,
			DeletedDefinition:         NonBreaking,
		},
	}
}

func getCompatibilityForChange(diffCode SpecChangeCode, where DataDirection) Compatibility {
	if compat, commonChange := compatibility.ForChange[diffCode]; commonChange {
		return compat
	}
	if where == Request {
		return compatibility.ForRequest[diffCode]
	}
	return compatibility.ForResponse[diffCode]
}
