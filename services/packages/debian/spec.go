// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package debian

import (
	"context"
	"html/template"
	"strings"

	packages_model "gitea.dev/models/packages"
	"gitea.dev/modules/container"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/httplib"
	debian_module "gitea.dev/modules/packages/debian"
	"gitea.dev/modules/util"
	packages_service "gitea.dev/services/packages"
)

type Specialization struct {
	packages_service.SpecOnBeforeRemovePackageAll
	packages_service.SpecOnBeforeRemovePackageVersion
}

var _ packages_service.Specialization = (*Specialization)(nil)

type debianViewData struct {
	Distributions []string
	Components    []string
	Architectures []string
}

func (s Specialization) GetViewPackageVersionData(ctx context.Context, pd *packages_model.PackageDescriptor) (any, error) {
	distributions := make(container.Set[string])
	components := make(container.Set[string])
	architectures := make(container.Set[string])
	for _, f := range pd.Files {
		for _, pp := range f.Properties {
			switch pp.Name {
			case debian_module.PropertyDistribution:
				distributions.Add(pp.Value)
			case debian_module.PropertyComponent:
				components.Add(pp.Value)
			case debian_module.PropertyArchitecture:
				architectures.Add(pp.Value)
			}
		}
	}
	viewData := debianViewData{
		Distributions: util.Sorted(distributions.Values()),
		Components:    util.Sorted(components.Values()),
		Architectures: util.Sorted(architectures.Values()),
	}
	return viewData, nil
}

func writeVarSelect(w *htmlutil.HTMLBuilder, varName string, varItems []string) {
	w.WriteFormatf(`<div>%s=<select data-code-var="%s">`, varName, varName)
	for _, v := range varItems {
		w.WriteFormatf(`<option value="%s">%s</option>`, v, v)
	}
	w.WriteHTML("</select></div>")
}

func (s Specialization) RenderSetupManual(ctx context.Context, pkg *packages_model.PackageDescriptor, viewDataAny any) template.HTML {
	viewData := viewDataAny.(debianViewData) //nolint:forcetypeassert // must be valid
	const nl = "\n"
	appFullLink := strings.TrimSuffix(httplib.GuessCurrentAppURL(ctx), "/")

	w := &htmlutil.HTMLBuilder{}
	w.WriteHTML(`<div data-global-init="initPackagesManualVars">`)
	{
		w.WriteHTML(`<div class="package-manual-vars">`)
		writeVarSelect(w, "distribution", viewData.Distributions)
		writeVarSelect(w, "component", viewData.Components)
		w.WriteHTML(`</div>`)
	}
	{
		w.WriteHTML(`<div class="markup"><pre class="code-block"><code>`)
		w.WriteFormatf(`sudo curl %s/api/packages/%s/debian/repository.key -o /etc/apt/keyrings/gitea-%s.asc`+nl+nl, appFullLink, pkg.Owner.Name, pkg.Owner.Name)
		w.WriteFormatf(`echo "deb [signed-by=/etc/apt/keyrings/gitea-%s.asc] %s/api/packages/%s/debian <span>$distribution</span> <span>$component</span>" | sudo tee -a /etc/apt/sources.list.d/gitea.list`+nl+nl, pkg.Owner.Name, appFullLink, pkg.Owner.Name)
		w.WriteHTML(`sudo apt update` + nl)
		w.WriteHTML(`</code></pre></div>`)
	}
	w.WriteHTML(`</div>`)
	return w.HTMLString()
}
