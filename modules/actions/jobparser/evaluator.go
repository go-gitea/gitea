package jobparser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nektos/act/pkg/exprparser"
	"gopkg.in/yaml.v3"
)

// ExpressionEvaluator is copied from runner.expressionEvaluator,
// to avoid unnecessary dependencies
type ExpressionEvaluator struct {
	interpreter exprparser.Interpreter
}

func NewExpressionEvaluator(interpreter exprparser.Interpreter) *ExpressionEvaluator {
	return &ExpressionEvaluator{interpreter: interpreter}
}

func (ee ExpressionEvaluator) evaluate(in string, defaultStatusCheck exprparser.DefaultStatusCheck) (interface{}, error) {
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
	expr, _ := rewriteSubExpression(in, false)
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
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in
	}

	expr, _ := rewriteSubExpression(in, true)
	evaluated, err := ee.evaluate(expr, exprparser.DefaultStatusCheckNone)
	if err != nil {
		return ""
	}

	value, ok := evaluated.(string)
	if !ok {
		panic(fmt.Sprintf("Expression %s did not evaluate to a string", expr))
	}

	return value
}

func escapeFormatString(in string) string {
	return strings.ReplaceAll(strings.ReplaceAll(in, "{", "{{"), "}", "}}")
}

func rewriteSubExpression(in string, forceFormat bool) (string, error) {
	if !strings.Contains(in, "${{") || !strings.Contains(in, "}}") {
		return in, nil
	}

	strPattern := regexp.MustCompile("(?:''|[^'])*'")
	pos := 0
	exprStart := -1
	strStart := -1
	var results []string
	formatOut := ""
	for pos < len(in) {
		if strStart > -1 {
			matches := strPattern.FindStringIndex(in[pos:])
			if matches == nil {
				panic("unclosed string.")
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
				formatOut += fmt.Sprintf("{%d}", len(results))
				results = append(results, strings.TrimSpace(in[exprStart:pos+exprEnd]))
				pos += exprEnd + 2
				exprStart = -1
			} else if strStart > -1 {
				pos += strStart + 1
			} else {
				panic("unclosed expression.")
			}
		} else {
			exprStart = strings.Index(in[pos:], "${{")
			if exprStart != -1 {
				formatOut += escapeFormatString(in[pos : pos+exprStart])
				exprStart = pos + exprStart + 3
				pos = exprStart
			} else {
				formatOut += escapeFormatString(in[pos:])
				pos = len(in)
			}
		}
	}

	if len(results) == 1 && formatOut == "{0}" && !forceFormat {
		return in, nil
	}

	out := fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut, "'", "''"), strings.Join(results, ", "))
	return out, nil
}
