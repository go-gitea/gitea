package generator

import (
	"github.com/go-openapi/analysis"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/swag"
)

type discInfo struct {
	Discriminators map[string]discor
	Discriminated  map[string]discee
}

type discor struct {
	FieldName string   `json:"fieldName"`
	GoType    string   `json:"goType"`
	JSONName  string   `json:"jsonName"`
	Children  []discee `json:"children"`
}

type discee struct {
	FieldName  string   `json:"fieldName"`
	FieldValue string   `json:"fieldValue"`
	GoType     string   `json:"goType"`
	JSONName   string   `json:"jsonName"`
	Ref        spec.Ref `json:"ref"`
	ParentRef  spec.Ref `json:"parentRef"`
}

func discriminatorInfo(doc *analysis.Spec) *discInfo {
	baseTypes := make(map[string]discor)
	for _, sch := range doc.AllDefinitions() {
		if sch.Schema.Discriminator != "" {
			tpe, _ := sch.Schema.Extensions.GetString(xGoName)
			if tpe == "" {
				tpe = swag.ToGoName(sch.Name)
			}
			baseTypes[sch.Ref.String()] = discor{
				FieldName: sch.Schema.Discriminator,
				GoType:    tpe,
				JSONName:  sch.Name,
			}
		}
	}

	subTypes := make(map[string]discee)
	for _, sch := range doc.SchemasWithAllOf() {
		for _, ao := range sch.Schema.AllOf {
			if ao.Ref.String() != "" {
				if bt, ok := baseTypes[ao.Ref.String()]; ok {
					name, _ := sch.Schema.Extensions.GetString(xClass)
					if name == "" {
						name = sch.Name
					}
					tpe, _ := sch.Schema.Extensions.GetString(xGoName)
					if tpe == "" {
						tpe = swag.ToGoName(sch.Name)
					}
					dce := discee{
						FieldName:  bt.FieldName,
						FieldValue: name,
						Ref:        sch.Ref,
						ParentRef:  ao.Ref,
						JSONName:   sch.Name,
						GoType:     tpe,
					}
					subTypes[sch.Ref.String()] = dce
					bt.Children = append(bt.Children, dce)
					baseTypes[ao.Ref.String()] = bt
				}
			}
		}
	}
	return &discInfo{Discriminators: baseTypes, Discriminated: subTypes}
}
