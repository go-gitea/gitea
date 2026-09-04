// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"errors"
	"io"

	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

// maxExpandedNodes bounds how many nodes alias expansion may create. go-yaml's own alias guard does
// not cover us: it only counts while decoding into values, and a workflow is kept as raw yaml.Nodes.
const maxExpandedNodes = 50000

var errTooManyYamlNodes = errors.New("maximum YAML nodes exceeded")

// ReadWorkflow decodes a workflow file with its aliases expanded. Callers inspect the workflow's
// raw nodes by kind, and an alias is a kind none of them expect.
func ReadWorkflow(content []byte) (*model.Workflow, error) {
	doc, err := resolveYamlAliases(content)
	if err != nil {
		return nil, err
	}
	return readWorkflowDoc(doc)
}

func readWorkflowDoc(doc *yaml.Node) (*model.Workflow, error) {
	if doc.Kind == 0 {
		return nil, io.EOF // what a yaml decoder reports for an empty file
	}
	w := new(model.Workflow)
	return w, doc.Decode(w)
}

// decodeResolved is yaml.Unmarshal with aliases expanded first.
func decodeResolved(content []byte, out any) error {
	doc, err := resolveYamlAliases(content)
	if err != nil {
		return err
	}
	return decodeYamlDoc(doc, out)
}

func decodeYamlDoc(doc *yaml.Node, out any) error {
	if doc.Kind == 0 {
		return nil // an empty document, as yaml.Unmarshal treats it
	}
	return doc.Decode(out)
}

// resolveYamlAliases parses content and replaces every alias with a copy of the node its anchor names.
func resolveYamlAliases(content []byte) (*yaml.Node, error) {
	doc := &yaml.Node{}
	if err := yaml.Unmarshal(content, doc); err != nil {
		return nil, err
	}
	budget := maxExpandedNodes
	return doc, expandAliases(doc, &budget)
}

// expandAliases replaces node's alias descendants in place.
func expandAliases(node *yaml.Node, budget *int) error {
	node.Anchor = "" // a name for a node, not part of the workflow: keep it out of the payloads
	if err := rejectMergeKeys(node); err != nil {
		return err
	}
	for i, child := range node.Content {
		if child.Kind != yaml.AliasNode {
			if err := expandAliases(child, budget); err != nil {
				return err
			}
			continue
		}
		copied, err := copyExpanded(child.Alias, budget)
		if err != nil {
			return err
		}
		node.Content[i] = copied
	}
	return nil
}

// copyExpanded deep copies a node expandAliases already expanded and validated, since an anchor is
// declared before the alias naming it. An anchor aliased from inside itself is the exception, and
// recurses here until it exhausts budget.
func copyExpanded(node *yaml.Node, budget *int) (*yaml.Node, error) {
	if *budget--; *budget < 0 {
		return nil, errTooManyYamlNodes
	}
	if node.Kind == yaml.AliasNode {
		return copyExpanded(node.Alias, budget)
	}

	copied := *node
	copied.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		child, err := copyExpanded(child, budget)
		if err != nil {
			return nil, err
		}
		copied.Content[i] = child
	}
	return &copied, nil
}

// rejectMergeKeys refuses `<<: *anchor`, same as GitHub does
func rejectMergeKeys(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Tag == "!!merge" {
			return errors.New("merge keys (`<<`) are not supported, alias the whole value instead")
		}
	}
	return nil
}
