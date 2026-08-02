// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gitea.com/gitea/runner/act/exprparser"
	"go.yaml.in/yaml/v4"
)

// ExpressionEvaluator is copied from runner.expressionEvaluator,
// to avoid unnecessary dependencies
type ExpressionEvaluator struct {
	interpreter exprparser.Interpreter
}

func NewExpressionEvaluator(interpreter exprparser.Interpreter) *ExpressionEvaluator {
	return &ExpressionEvaluator{interpreter: interpreter}
}

func (ee ExpressionEvaluator) evaluate(in string, defaultStatusCheck exprparser.DefaultStatusCheck) (any, error) {
	evaluated, err := ee.interpreter.Evaluate(in, defaultStatusCheck)

	return evaluated, err
}

func (ee ExpressionEvaluator) evaluateScalarYamlNode(node *yaml.Node) error {
	var in string
	if err := node.Decode(&in); err != nil {
		return err
	}
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return nil
	}
	expr, err := rewriteSubExpression(in)
	if err != nil {
		return err
	}
	res, err := ee.evaluate(expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return err
	}
	return node.Encode(res)
}

func (ee ExpressionEvaluator) evaluateMappingYamlNode(node *yaml.Node) error {
	// GitHub has this undocumented feature to merge maps, called insert directive
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	for i := 0; i < len(node.Content)/2; {
		k := node.Content[i*2]
		v := node.Content[i*2+1]
		if err := ee.EvaluateYamlNode(v); err != nil {
			return err
		}
		var sk string
		// Merge the nested map of the insert directive
		if k.Decode(&sk) == nil && insertDirective.MatchString(sk) {
			node.Content = append(append(node.Content[:i*2], v.Content...), node.Content[(i+1)*2:]...)
			i += len(v.Content) / 2
		} else {
			if err := ee.EvaluateYamlNode(k); err != nil {
				return err
			}
			i++
		}
	}
	return nil
}

func (ee ExpressionEvaluator) evaluateSequenceYamlNode(node *yaml.Node) error {
	for i := 0; i < len(node.Content); {
		v := node.Content[i]
		// Preserve nested sequences
		wasseq := v.Kind == yaml.SequenceNode
		if err := ee.EvaluateYamlNode(v); err != nil {
			return err
		}
		// GitHub has this undocumented feature to merge sequences / arrays
		// We have a nested sequence via evaluation, merge the arrays
		if v.Kind == yaml.SequenceNode && !wasseq {
			node.Content = append(append(node.Content[:i], v.Content...), node.Content[i+1:]...)
			i += len(v.Content)
		} else {
			i++
		}
	}
	return nil
}

func (ee ExpressionEvaluator) EvaluateYamlNode(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return ee.evaluateScalarYamlNode(node)
	case yaml.MappingNode:
		return ee.evaluateMappingYamlNode(node)
	case yaml.SequenceNode:
		return ee.evaluateSequenceYamlNode(node)
	default:
		return nil
	}
}

func (ee ExpressionEvaluator) Interpolate(in string) string {
	out, err := interpolate(ee.interpreter, in)
	if err != nil {
		return ""
	}
	return out
}

// interpolate evaluates each part on its own, so that a malformed one such as
// `${{ 1) && (2 }}` cannot restructure the expressions around it.
func interpolate(interpreter exprparser.Interpreter, in string) (string, error) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in, nil
	}

	parts, err := splitSubExpressions(in)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	out.Grow(len(in))
	for _, part := range parts {
		if !part.isExpr {
			out.WriteString(part.text)
			continue
		}
		evaluated, err := interpreter.Evaluate(part.text, exprparser.DefaultStatusCheckNone)
		if err != nil {
			return "", err
		}
		out.WriteString(exprparser.CoerceToString(evaluated))
	}
	return out.String(), nil
}

func escapeFormatString(in string) string {
	return strings.ReplaceAll(strings.ReplaceAll(in, "{", "{{"), "}", "}}")
}

type exprPart struct {
	text   string
	isExpr bool
}

var subExpressionStringRegexp = regexp.MustCompile("(?:''|[^'])*'")

func splitSubExpressions(in string) ([]exprPart, error) {
	pos := 0
	exprStart := -1
	strStart := -1
	parts := make([]exprPart, 0, 2*strings.Count(in, "${{")+1)
	for pos < len(in) {
		if strStart > -1 {
			matches := subExpressionStringRegexp.FindStringIndex(in[pos:])
			if matches == nil {
				return nil, errors.New("unclosed string")
			}

			strStart = -1
			pos += matches[1]
		} else if exprStart > -1 {
			exprEnd := strings.Index(in[pos:], "}}")
			strStart = strings.Index(in[pos:], "'")

			if exprEnd > -1 && strStart > -1 {
				if exprEnd < strStart {
					strStart = -1
				} else {
					exprEnd = -1
				}
			}

			if exprEnd > -1 {
				parts = append(parts, exprPart{text: strings.TrimSpace(in[exprStart : pos+exprEnd]), isExpr: true})
				pos += exprEnd + 2
				exprStart = -1
			} else if strStart > -1 {
				pos += strStart + 1
			} else {
				return nil, errors.New("unclosed expression")
			}
		} else {
			exprStart = strings.Index(in[pos:], "${{")
			if exprStart != -1 {
				parts = append(parts, exprPart{text: in[pos : pos+exprStart]})
				exprStart = pos + exprStart + 3
				pos = exprStart
			} else {
				parts = append(parts, exprPart{text: in[pos:]})
				pos = len(in)
			}
		}
	}
	return parts, nil
}

func rewriteSubExpression(in string) (string, error) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in, nil
	}

	parts, err := splitSubExpressions(in)
	if err != nil {
		return "", err
	}

	var results []string
	var formatOut strings.Builder
	for _, part := range parts {
		if part.isExpr {
			fmt.Fprintf(&formatOut, "{%d}", len(results))
			results = append(results, part.text)
		} else {
			formatOut.WriteString(escapeFormatString(part.text))
		}
	}

	if len(results) == 1 && formatOut.String() == "{0}" {
		return in, nil
	}

	out := fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut.String(), "'", "''"), strings.Join(results, ", "))
	return out, nil
}
