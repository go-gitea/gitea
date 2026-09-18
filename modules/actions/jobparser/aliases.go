// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"errors"
	"io"

	"gitea.dev/actionslib/pkg/model"

	"go.yaml.in/yaml/v4"
)

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
	doc, err := model.ReadWorkflowNode(bytes.NewReader(content))
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return doc, rejectMergeKeys(doc)
}

// rejectMergeKeys refuses `<<: *anchor`, same as GitHub does
func rejectMergeKeys(node *yaml.Node) error {
	for i, child := range node.Content {
		if node.Kind == yaml.MappingNode && i%2 == 0 && child.Tag == "!!merge" {
			return errors.New("merge keys (`<<`) are not supported, alias the whole value instead")
		}
		if err := rejectMergeKeys(child); err != nil {
			return err
		}
	}
	return nil
}
