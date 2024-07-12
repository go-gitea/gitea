// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
func (pm *Manager) Processes(flat, noSystem bool) ([]*Process, int) {
	pm.mutex.Lock()
	processCount := len(pm.processMap)
	processes := make([]*Process, 0, len(pm.processMap))
	if flat {
		for _, process := range pm.processMap {
			if noSystem && process.Type == SystemProcessType {
				continue
			}
			processes = append(processes, process.toProcess())
		}
	} else {
		// We need our own processMap
		processMap := map[IDType]*Process{}
		for _, internalProcess := range pm.processMap {
			process, ok := processMap[internalProcess.PID]
			if !ok {
				process = internalProcess.toProcess()
				processMap[process.PID] = process
			}

			// Check its parent
			if process.ParentPID == "" {
				processes = append(processes, process)
				continue
			}

			internalParentProcess, ok := pm.processMap[internalProcess.ParentPID]
			if ok {
				parentProcess, ok := processMap[process.ParentPID]
				if !ok {
					parentProcess = internalParentProcess.toProcess()
					processMap[parentProcess.PID] = parentProcess
				}
				parentProcess.Children = append(parentProcess.Children, process)
				continue
			}

			processes = append(processes, process)
		}
	}
	pm.mutex.Unlock()

	if !flat && noSystem {
		for i := 0; i < len(processes); i++ {
			process := processes[i]
			if process.Type != SystemProcessType {
				continue
			}
			processes[len(processes)-1], processes[i] = processes[i], processes[len(processes)-1]
			processes = append(processes[:len(processes)-1], process.Children...)
			i--
		}
	}

	// Sort by process' start time. Oldest process appears first.
	sort.Slice(processes, func(i, j int) bool {
		left, right := processes[i], processes[j]

		return left.Start.Before(right.Start)
	})

	return processes, processCount
}

// ProcessStacktraces gets the processes and stacktraces in a thread safe manner
func (pm *Manager) ProcessStacktraces(flat, noSystem bool) ([]*Process, int, int64, error) {
	var stacks *profile.Profile
	var err error

	// We cannot use the pm.ProcessMap here because we will release the mutex ...
	processMap := map[IDType]*Process{}
	var processCount int

	// Lock the manager
	pm.mutex.Lock()
	processCount = len(pm.processMap)

	// Add a defer to unlock in case there is a panic
	unlocked := false
	defer func() {
		if !unlocked {
			pm.mutex.Unlock()
		}
	}()

	processes := make([]*Process, 0, len(pm.processMap))
	if flat {
		for _, internalProcess := range pm.processMap {
			process := internalProcess.toProcess()
			processMap[process.PID] = process
			if noSystem && internalProcess.Type == SystemProcessType {
				continue
			}
			processes = append(processes, process)
		}
	} else {
		for _, internalProcess := range pm.processMap {
			process, ok := processMap[internalProcess.PID]
			if !ok {
				process = internalProcess.toProcess()
				processMap[process.PID] = process
			}

			// Check its parent
			if process.ParentPID == "" {
				processes = append(processes, process)
				continue
			}

			internalParentProcess, ok := pm.processMap[internalProcess.ParentPID]
			if ok {
				parentProcess, ok := processMap[process.ParentPID]
				if !ok {
					parentProcess = internalParentProcess.toProcess()
					processMap[parentProcess.PID] = parentProcess
				}
				parentProcess.Children = append(parentProcess.Children, process)
				continue
			}

			processes = append(processes, process)
		}
	}

	// Now from within the lock we need to get the goroutines.
	// Why? If we release the lock then between between filling the above map and getting
	// the stacktraces another process could be created which would then look like a dead process below
	reader, writer := io.Pipe()
	defer reader.Close()
	go func() {
		err := pprof.Lookup("goroutine").WriteTo(writer, 0)
		_ = writer.CloseWithError(err)
	}()
	stacks, err = profile.Parse(reader)
	if err != nil {
		return nil, 0, 0, err
	}

	// Unlock the mutex
	pm.mutex.Unlock()
	unlocked = true

	goroutineCount := int64(0)

	// Now walk through the "Sample" slice in the goroutines stack
	for _, sample := range stacks.Sample {
		// In the "goroutine" pprof profile each sample represents one or more goroutines
		// with the same labels and stacktraces.

		// We will represent each goroutine by a `Stack`
		stack := &Stack{}

		// Add the non-process associated labels from the goroutine sample to the Stack
		for name, value := range sample.Label {
			if name == DescriptionPProfLabel || name == PIDPProfLabel || (!flat && name == PPIDPProfLabel) || name == ProcessTypePProfLabel {
				continue
			}

			// Labels from the "goroutine" pprof profile only have one value.
			// This is because the underlying representation is a map[string]string
			if len(value) != 1 {
				// Unexpected...
				return nil, 0, 0, fmt.Errorf("label: %s in goroutine stack with unexpected number of values: %v", name, value)
			}

			stack.Labels = append(stack.Labels, &Label{Name: name, Value: value[0]})
		}

		// The number of goroutines that this sample represents is the `stack.Value[0]`
		stack.Count = sample.Value[0]
		goroutineCount += stack.Count

		// Now we want to associate this Stack with a Process.
		var process *Process

		// Try to get the PID from the goroutine labels
		if pidvalue, ok := sample.Label[PIDPProfLabel]; ok && len(pidvalue) == 1 {
			pid := IDType(pidvalue[0])

			// Now try to get the process from our map
			process, ok = processMap[pid]
			if !ok && pid != "" {
				// This means that no process has been found in the process map - but there was a process PID
				// Therefore this goroutine belongs to a dead process and it has escaped control of the process as it
				// should have died with the process context cancellation.

				// We need to create a dead process holder for this process and label it appropriately

				// get the parent PID
				ppid := IDType("")
				if value, ok := sample.Label[PPIDPProfLabel]; ok && len(value) == 1 {
					ppid = IDType(value[0])
				}

				// format the description
				description := "(dead process)"
				if value, ok := sample.Label[DescriptionPProfLabel]; ok && len(value) == 1 {
					description = value[0] + " " + description
				}

				// override the type of the process to "code" but add the old type as a label on the first stack
				ptype := NoneProcessType
				if value, ok := sample.Label[ProcessTypePProfLabel]; ok && len(value) == 1 {
					stack.Labels = append(stack.Labels, &Label{Name: ProcessTypePProfLabel, Value: value[0]})
				}
				process = &Process{
					PID:         pid,
					ParentPID:   ppid,
					Description: description,
					Type:        ptype,
				}

				// Now add the dead process back to the map and tree so we don't go back through this again.
				processMap[process.PID] = process
				added := false
				if process.ParentPID != "" && !flat {
					if parent, ok := processMap[process.ParentPID]; ok {
						parent.Children = append(parent.Children, process)
						added = true
					}
				}
				if !added {
					processes = append(processes, process)
				}
			}
		}

		if process == nil {
			// This means that the sample we're looking has no PID label
			var ok bool
			process, ok = processMap[""]
			if !ok {
				// this is the first time we've come acrross an unassociated goroutine so create a "process" to hold them
				process = &Process{
					Description: "(unassociated)",
					Type:        NoneProcessType,
				}
				processMap[process.PID] = process
				processes = append(processes, process)
			}
		}

		// The sample.Location represents a stack trace for this goroutine,
		// however each Location can represent multiple lines (mostly due to inlining)
		// so we need to walk the lines too
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

		// Now we need a short-descriptive name to call the stack trace if when it is folded and
		// assuming the stack trace has some lines we'll choose the bottom of the stack (i.e. the
		// initial function that started the stack trace.) The top of the stack is unlikely to
		// be very helpful as a lot of the time it will be runtime.select or some other call into
		// a std library.
		stack.Description = "(unknown)"
		if len(stack.Entry) > 0 {
			stack.Description = stack.Entry[len(stack.Entry)-1].Function
		}

		process.Stacks = append(process.Stacks, stack)
	}

	// restrict to not show system processes
	if noSystem {
		for i := 0; i < len(processes); i++ {
			process := processes[i]
			if process.Type != SystemProcessType && process.Type != NoneProcessType {
				continue
			}
			processes[len(processes)-1], processes[i] = processes[i], processes[len(processes)-1]
			processes = append(processes[:len(processes)-1], process.Children...)
			i--
		}
	}

	// Now finally re-sort the processes. Newest process appears first
	after := func(processes []*Process) func(i, j int) bool {
		return func(i, j int) bool {
			left, right := processes[i], processes[j]
			return left.Start.After(right.Start)
		}
	}
	sort.Slice(processes, after(processes))
	if !flat {
		var sortChildren func(process *Process)

		sortChildren = func(process *Process) {
			sort.Slice(process.Children, after(process.Children))
			for _, child := range process.Children {
				sortChildren(child)
			}
		}
	}

	return processes, processCount, goroutineCount, err
}
