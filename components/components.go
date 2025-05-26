package components

import (
	"code.gitea.io/gitea/modules/svg"
	g "maragu.dev/gomponents"
)

func If(condition bool, node g.Node) g.Node {
	if condition {
		return node
	}
	return nil
}

func SVG(icon string, others ...any) g.Node {
	return g.Raw(string(svg.RenderHTML(icon)))
}

// Utility to add "active" class if condition is true
func classIf(condition bool, class string) string {
	if condition {
		return class
	}
	return ""
}
