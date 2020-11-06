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
		},
		ForRequest: map[SpecChangeCode]Compatibility{
			AddedRequiredProperty:          Breaking,
			DeletedProperty:                Breaking,
			AddedProperty:                  Breaking,
			AddedOptionalParam:             NonBreaking,
			AddedRequiredParam:             Breaking,
			DeletedOptionalParam:           NonBreaking,
			DeletedRequiredParam:           NonBreaking,
			WidenedType:                    NonBreaking,
			NarrowedType:                   Breaking,
			ChangedType:                    Breaking,
			ChangedToCompatibleType:        NonBreaking,
			ChangedOptionalToRequiredParam: Breaking,
			ChangedRequiredToOptionalParam: NonBreaking,
			AddedEnumValue:                 NonBreaking,
			DeletedEnumValue:               Breaking,
			ChangedDescripton:              NonBreaking,
			AddedDescripton:                NonBreaking,
			DeletedDescripton:              NonBreaking,
			ChangedTag:                     NonBreaking,
			AddedTag:                       NonBreaking,
			DeletedTag:                     NonBreaking,
		},
		ForChange: map[SpecChangeCode]Compatibility{
			NoChangeDetected:          NonBreaking,
			AddedEndpoint:             NonBreaking,
			DeletedEndpoint:           Breaking,
			DeletedDeprecatedEndpoint: NonBreaking,
			AddedConsumesFormat:       NonBreaking,
			DeletedConsumesFormat:     Breaking,
			AddedProducesFormat:       Breaking,
			DeletedProducesFormat:     NonBreaking,
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
