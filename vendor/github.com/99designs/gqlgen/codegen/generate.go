package codegen

import (
	"github.com/99designs/gqlgen/codegen/templates"
)

func GenerateCode(data *Data) error {
	return templates.Render(templates.Options{
		PackageName:     data.Config.Exec.Package,
		Filename:        data.Config.Exec.Filename,
		Data:            data,
		RegionTags:      true,
		GeneratedHeader: true,
		Packages:        data.Config.Packages,
	})
}
