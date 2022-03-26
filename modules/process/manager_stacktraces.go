// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"fmt"
	"io"
	"runtime/pprof"
	"sort"
	"time"

	"github.com/google/pprof/profile"
)

// StackEntry is an entry on a stacktrace
type StackEntry struct {
	Function string
	File     string
	Line     int
}

// Label represents a pprof label assigned to goroutine stack
type Label struct {
	Name  string
	Value string
}

// Stack is a stacktrace relating to a goroutine. (Multiple goroutines may have the same stacktrace)
type Stack struct {
	Count       int64 // Number of goroutines with this stack trace
	Description string
	Labels      []*Label      `json:",omitempty"`
	Entry       []*StackEntry `json:",omitempty"`
}

// A Process is a combined representation of a Process and a Stacktrace for the goroutines associated with it
type Process struct {
	PID         IDType
	ParentPID   IDType
	Description string
	Start       time.Time
	Type        string

	Children []*Process `json:",omitempty"`
	Stacks   []*Stack   `json:",omitempty"`
}

// Processes gets the processes in a thread safe manner
func (pm *Manager) Processes(onlyRoots, noSystem bool, runInLock func()) []*Process {
	pm.mutex.Lock()
	processes := make([]*Process, 0, len(pm.processes))
	if onlyRoots {
		for _, process := range pm.processes {
			if noSystem && process.Type == SystemProcessType {
				continue
			}
			if parent, has := pm.processes[process.ParentPID]; !has ||
				(noSystem && parent.Type == SystemProcessType) {
				processes = append(processes, process.ToProcess(true))
			}
		}
	} else {
		for _, process := range pm.processes {
			if noSystem && process.Type == SystemProcessType {
				continue
			}
			processes = append(processes, process.ToProcess(false))
		}
	}
	if runInLock != nil {
		runInLock()
	}
	pm.mutex.Unlock()

	sort.Slice(processes, func(i, j int) bool {
		left, right := processes[i], processes[j]

		return left.Start.Before(right.Start)
	})

	return processes
}

// ProcessStacktraces gets the processes and stacktraces in a thread safe manner
func (pm *Manager) ProcessStacktraces(flat, onlyRequests bool) ([]*Process, int64, error) {
	var stacks *profile.Profile
	var err error
	processes := pm.Processes(false, false, func() {
		reader, writer := io.Pipe()
		defer reader.Close()
		go func() {
			err := pprof.Lookup("goroutine").WriteTo(writer, 0)
			_ = writer.CloseWithError(err)
		}()
		stacks, err = profile.Parse(reader)
		if err != nil {
			return
		}
	})
	if err != nil {
		return nil, 0, err
	}

	// We cannot use the process pidmaps here because we have released the mutex ...
	pidMap := map[IDType]*Process{}
	processStacks := make([]*Process, 0, len(processes))
	for _, process := range processes {
		pStack := &Process{
			PID:         process.PID,
			ParentPID:   process.ParentPID,
			Description: process.Description,
			Start:       process.Start,
			Type:        process.Type,
		}

		pidMap[process.PID] = pStack
		if flat {
			processStacks = append(processStacks, pStack)
		} else if parent, ok := pidMap[process.ParentPID]; ok {
			parent.Children = append(parent.Children, pStack)
		} else {
			processStacks = append(processStacks, pStack)
		}
	}

	goroutineCount := int64(0)

	// Now walk through the "Sample" slice in the goroutines stack
	for _, sample := range stacks.Sample {
		stack := &Stack{}

		// Add the labels
		for name, value := range sample.Label {
			if name == DescriptionPProfLabel || name == PIDPProfLabel || (!flat && name == PPIDPProfLabel) || name == ProcessTypePProfLabel {
				continue
			}
			if len(value) != 1 {
				// Unexpected...
				return nil, 0, fmt.Errorf("label: %s in goroutine stack with unexpected number of values: %v", name, value)
			}

			stack.Labels = append(stack.Labels, &Label{Name: name, Value: value[0]})
		}

		stack.Count = sample.Value[0]
		goroutineCount += stack.Count

		// Now get the processStack for this goroutine sample
		var processStack *Process
		if pidvalue, ok := sample.Label[PIDPProfLabel]; ok && len(pidvalue) == 1 {
			pid := IDType(pidvalue[0])
			processStack, ok = pidMap[pid]
			if !ok && pid != "" {
				ppid := IDType("")
				if value, ok := sample.Label[PPIDPProfLabel]; ok && len(value) == 1 {
					ppid = IDType(value[0])
				}
				description := "(dead process)"
				if value, ok := sample.Label[DescriptionPProfLabel]; ok && len(value) == 1 {
					description = value[0] + " " + description
				}
				ptype := "code"
				if value, ok := sample.Label[ProcessTypePProfLabel]; ok && len(value) == 1 {
					stack.Labels = append(stack.Labels, &Label{Name: ProcessTypePProfLabel, Value: value[0]})
				}
				processStack = &Process{
					PID:         pid,
					ParentPID:   ppid,
					Description: description,
					Type:        ptype,
				}

				pidMap[processStack.PID] = processStack
				added := false
				if processStack.ParentPID != "" && !flat {
					if parent, ok := pidMap[processStack.ParentPID]; ok {
						parent.Children = append(parent.Children, processStack)
						added = true
					}
				}
				if !added {
					processStacks = append(processStacks, processStack)
				}
			}
		}
		if processStack == nil {
			var ok bool
			processStack, ok = pidMap[""]
			if !ok {
				processStack = &Process{
					Description: "(unassociated)",
					Type:        "code",
				}
				pidMap[processStack.PID] = processStack
				processStacks = append(processStacks, processStack)
			}
		}

		// Now walk through the locations...
		for _, location := range sample.Location {
			for _, line := range location.Line {
				entry := &StackEntry{
					Function: line.Function.Name,
					File:     line.Function.Filename,
					Line:     int(line.Line),
				}
				stack.Entry = append(stack.Entry, entry)
			}
		}
		stack.Description = "(unknown)"
		if len(stack.Entry) > 0 {
			stack.Description = stack.Entry[len(stack.Entry)-1].Function
		}

		processStack.Stacks = append(processStack.Stacks, stack)
	}

	if onlyRequests {
		var requestStacks []*Process
		i := len(processStacks) - 1
		for i >= 0 {
			processStack := processStacks[i]
			if processStack.Type == RequestProcessType {
				requestStacks = append(requestStacks, processStack)
				i--
				continue
			}
			if len(processStack.Children) > 0 {
				processStacks = processStacks[:i]
				processStacks = append(processStacks, processStack.Children...)
				i = len(processStacks) - 1
				continue
			}
			i--
		}
		processStacks = requestStacks
	}

	// Now finally re-sort the processstacks so the newest processes are at the top
	after := func(processStacks []*Process) func(i, j int) bool {
		return func(i, j int) bool {
			left, right := processStacks[i], processStacks[j]
			return left.Start.After(right.Start)
		}
	}
	sort.Slice(processStacks, after(processStacks))
	if !flat {
		for _, processStack := range processStacks {
			sort.Slice(processStack.Children, after(processStack.Children))
		}
	}

	return processStacks, goroutineCount, err
}
