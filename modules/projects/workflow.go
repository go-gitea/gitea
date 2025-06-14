// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package projects

// Action represents an action that can be taken in a workflow
type Action struct {
	SetValue string
}

const (
	// Project workflow event names
	EventItemAddedToProject = "item_added_to_project"
	EventItemClosed         = "item_closed"
	EventItem
)

type Event struct {
	Name    string
	Types   []string
	Actions []Action
}

type Workflow struct {
	Name      string
	Events    []Event
	ProjectID int64
}

func ParseWorkflow(content string) (*Workflow, error) {
	return &Workflow{}, nil
}

func (w *Workflow) FireAction(evtName string, f func(action Action) error) error {
	for _, evt := range w.Events {
		if evt.Name == evtName {
			for _, action := range evt.Actions {
				// Do something with action
				if err := f(action); err != nil {
					return err
				}
			}
			break
		}
	}
	return nil
}
