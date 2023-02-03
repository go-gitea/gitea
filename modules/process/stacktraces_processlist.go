// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package process

import (
	"bytes"
	"fmt"
	"io"
)

// WriteProcesses writes out processes to a provided writer
func WriteProcesses(out io.Writer, processes []*Process, processCount int, goroutineCount int64, indent string, flat bool) error {
	if goroutineCount > 0 {
		if _, err := fmt.Fprintf(out, "%sTotal Number of Goroutines: %d\n", indent, goroutineCount); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "%sTotal Number of Processes: %d\n", indent, processCount); err != nil {
		return err
	}
	if len(processes) > 0 {
		if err := WriteProcess(out, processes[0], "  ", flat); err != nil {
			return err
		}
	}
	if len(processes) > 1 {
		for _, process := range processes[1:] {
			if _, err := fmt.Fprintf(out, "%s  | \n", indent); err != nil {
				return err
			}
			if err := WriteProcess(out, process, "  ", flat); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteProcess writes out a process to a provided writer
func WriteProcess(out io.Writer, process *Process, indent string, flat bool) error {
	sb := &bytes.Buffer{}
	if flat {
		if process.ParentPID != "" {
			_, _ = fmt.Fprintf(sb, "%s+ PID: %s\t\tType: %s\n", indent, process.PID, process.Type)
		} else {
			_, _ = fmt.Fprintf(sb, "%s+ PID: %s:%s\tType: %s\n", indent, process.ParentPID, process.PID, process.Type)
		}
	} else {
		_, _ = fmt.Fprintf(sb, "%s+ PID: %s\tType: %s\n", indent, process.PID, process.Type)
	}
	indent += "| "

	_, _ = fmt.Fprintf(sb, "%sDescription: %s\n", indent, process.Description)
	_, _ = fmt.Fprintf(sb, "%sStart:       %s\n", indent, process.Start)

	if len(process.Stacks) > 0 {
		_, _ = fmt.Fprintf(sb, "%sGoroutines:\n", indent)
		for _, stack := range process.Stacks {
			indent := indent + "  "
			_, _ = fmt.Fprintf(sb, "%s+ Description: %s", indent, stack.Description)
			if stack.Count > 1 {
				_, _ = fmt.Fprintf(sb, "* %d", stack.Count)
			}
			_, _ = fmt.Fprintf(sb, "\n")
			indent += "| "
			if len(stack.Labels) > 0 {
				_, _ = fmt.Fprintf(sb, "%sLabels:      %q:%q", indent, stack.Labels[0].Name, stack.Labels[0].Value)

				if len(stack.Labels) > 1 {
					for _, label := range stack.Labels[1:] {
						_, _ = fmt.Fprintf(sb, ", %q:%q", label.Name, label.Value)
					}
				}
				_, _ = fmt.Fprintf(sb, "\n")
			}
			_, _ = fmt.Fprintf(sb, "%sStack:\n", indent)
			indent += "  "
			for _, entry := range stack.Entry {
				_, _ = fmt.Fprintf(sb, "%s+ %s\n", indent, entry.Function)
				_, _ = fmt.Fprintf(sb, "%s| %s:%d\n", indent, entry.File, entry.Line)
			}
		}
	}
	if _, err := out.Write(sb.Bytes()); err != nil {
		return err
	}
	sb.Reset()
	if len(process.Children) > 0 {
		if _, err := fmt.Fprintf(out, "%sChildren:\n", indent); err != nil {
			return err
		}
		for _, child := range process.Children {
			if err := WriteProcess(out, child, indent+"  ", flat); err != nil {
				return err
			}
		}
	}
	return nil
}
