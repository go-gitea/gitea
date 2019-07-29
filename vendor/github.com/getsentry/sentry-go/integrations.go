package sentry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// ================================
// Modules Integration
// ================================

type modulesIntegration struct{}

var _modulesCache map[string]string // nolint: gochecknoglobals

func (mi *modulesIntegration) Name() string {
	return "Modules"
}

func (mi *modulesIntegration) SetupOnce(client *Client) {
	client.AddEventProcessor(mi.processor)
}

func (mi *modulesIntegration) processor(event *Event, hint *EventHint) *Event {
	if event.Modules == nil {
		event.Modules = extractModules()
	}

	return event
}

func extractModules() map[string]string {
	if _modulesCache != nil {
		return _modulesCache
	}

	extractedModules, err := getModules()
	if err != nil {
		Logger.Printf("ModuleIntegration wasn't able to extract modules: %v\n", err)
		return nil
	}

	_modulesCache = extractedModules

	return extractedModules
}

func getModules() (map[string]string, error) {
	if fileExists("go.mod") {
		return getModulesFromMod()
	}

	if fileExists("vendor") {
		// Priority given to vendor created by modules
		if fileExists("vendor/modules.txt") {
			return getModulesFromVendorTxt()
		}

		if fileExists("vendor/vendor.json") {
			return getModulesFromVendorJSON()
		}
	}

	return nil, fmt.Errorf("module integration failed")
}

func getModulesFromMod() (map[string]string, error) {
	modules := make(map[string]string)

	file, err := os.Open("go.mod")
	if err != nil {
		return nil, fmt.Errorf("unable to open mod file")
	}

	defer file.Close()

	areModulesPresent := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		splits := strings.Split(scanner.Text(), " ")

		if splits[0] == "require" {
			areModulesPresent = true

			// Mod file has only 1 dependency
			if len(splits) > 2 {
				modules[strings.TrimSpace(splits[1])] = splits[2]
				return modules, nil
			}
		} else if areModulesPresent && splits[0] != ")" {
			modules[strings.TrimSpace(splits[0])] = splits[1]
		}
	}

	if scannerErr := scanner.Err(); scannerErr != nil {
		return nil, scannerErr
	}

	return modules, nil
}

func getModulesFromVendorTxt() (map[string]string, error) {
	modules := make(map[string]string)

	file, err := os.Open("vendor/modules.txt")
	if err != nil {
		return nil, fmt.Errorf("unable to open vendor/modules.txt")
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		splits := strings.Split(scanner.Text(), " ")

		if splits[0] == "#" {
			modules[splits[1]] = splits[2]
		}
	}

	if scannerErr := scanner.Err(); scannerErr != nil {
		return nil, scannerErr
	}

	return modules, nil
}

func getModulesFromVendorJSON() (map[string]string, error) {
	modules := make(map[string]string)

	file, err := ioutil.ReadFile("vendor/vendor.json")

	if err != nil {
		return nil, fmt.Errorf("unable to open vendor/vendor.json")
	}

	var vendor map[string]interface{}
	if unmarshalErr := json.Unmarshal(file, &vendor); unmarshalErr != nil {
		return nil, unmarshalErr
	}

	packages := vendor["package"].([]interface{})

	// To avoid iterative dependencies, TODO: Change of default value
	lastPath := "\n"

	for _, value := range packages {
		path := value.(map[string]interface{})["path"].(string)

		if !strings.Contains(path, lastPath) {
			// No versions are available through vendor.json
			modules[path] = ""
			lastPath = path
		}
	}

	return modules, nil
}

// ================================
// Environment Integration
// ================================

type environmentIntegration struct{}

func (ei *environmentIntegration) Name() string {
	return "Environment"
}

func (ei *environmentIntegration) SetupOnce(client *Client) {
	client.AddEventProcessor(ei.processor)
}

func (ei *environmentIntegration) processor(event *Event, hint *EventHint) *Event {
	if event.Contexts == nil {
		event.Contexts = make(map[string]interface{})
	}

	event.Contexts["device"] = map[string]interface{}{
		"arch":    runtime.GOARCH,
		"num_cpu": runtime.NumCPU(),
	}

	event.Contexts["os"] = map[string]interface{}{
		"name": runtime.GOOS,
	}

	event.Contexts["runtime"] = map[string]interface{}{
		"name":    "go",
		"version": runtime.Version(),
	}

	return event
}

// ================================
// Ignore Errors Integration
// ================================

type ignoreErrorsIntegration struct {
	ignoreErrors []*regexp.Regexp
}

func (iei *ignoreErrorsIntegration) Name() string {
	return "IgnoreErrors"
}

func (iei *ignoreErrorsIntegration) SetupOnce(client *Client) {
	iei.ignoreErrors = transformStringsIntoRegexps(client.Options().IgnoreErrors)
	client.AddEventProcessor(iei.processor)
}

func (iei *ignoreErrorsIntegration) processor(event *Event, hint *EventHint) *Event {
	suspects := getIgnoreErrorsSuspects(event)

	for _, suspect := range suspects {
		for _, pattern := range iei.ignoreErrors {
			if pattern.Match([]byte(suspect)) {
				Logger.Printf("Event dropped due to being matched by `IgnoreErrors` option."+
					"| Value matched: %s | Filter used: %s", suspect, pattern)
				return nil
			}
		}
	}

	return event
}

func transformStringsIntoRegexps(strings []string) []*regexp.Regexp {
	var exprs []*regexp.Regexp

	for _, s := range strings {
		r, err := regexp.Compile(s)
		if err == nil {
			exprs = append(exprs, r)
		}
	}

	return exprs
}

func getIgnoreErrorsSuspects(event *Event) []string {
	suspects := []string{}

	if event.Message != "" {
		suspects = append(suspects, event.Message)
	}

	for _, ex := range event.Exception {
		suspects = append(suspects, ex.Type)
		suspects = append(suspects, ex.Value)
	}

	return suspects
}

// ================================
// Contextify Frames Integration
// ================================

type contextifyFramesIntegration struct {
	sr              sourceReader
	contextLines    int
	cachedLocations map[string]string
}

func (cfi *contextifyFramesIntegration) Name() string {
	return "ContextifyFrames"
}

func (cfi *contextifyFramesIntegration) SetupOnce(client *Client) {
	cfi.sr = newSourceReader()
	cfi.contextLines = 5
	cfi.cachedLocations = make(map[string]string)

	client.AddEventProcessor(cfi.processor)
}

func (cfi *contextifyFramesIntegration) processor(event *Event, hint *EventHint) *Event {
	// Range over all exceptions
	for _, ex := range event.Exception {
		// If it has no stacktrace, just bail out
		if ex.Stacktrace == nil {
			continue
		}

		// If it does, it should have frames, so try to contextify them
		ex.Stacktrace.Frames = cfi.contextify(ex.Stacktrace.Frames)
	}

	// Range over all threads
	for _, th := range event.Threads {
		// If it has no stacktrace, just bail out
		if th.Stacktrace == nil {
			continue
		}

		// If it does, it should have frames, so try to contextify them
		th.Stacktrace.Frames = cfi.contextify(th.Stacktrace.Frames)
	}

	return event
}

func (cfi *contextifyFramesIntegration) contextify(frames []Frame) []Frame {
	contextifiedFrames := make([]Frame, 0, len(frames))

	for _, frame := range frames {
		if !frame.InApp {
			contextifiedFrames = append(contextifiedFrames, frame)
			continue
		}

		var path string

		if cachedPath, ok := cfi.cachedLocations[frame.AbsPath]; ok {
			path = cachedPath
		} else {
			// Optimize for happy path here
			if fileExists(frame.AbsPath) {
				path = frame.AbsPath
			} else {
				path = cfi.findNearbySourceCodeLocation(frame.AbsPath)
			}
		}

		if path == "" {
			contextifiedFrames = append(contextifiedFrames, frame)
			continue
		}

		lines, contextLine := cfi.sr.readContextLines(path, frame.Lineno, cfi.contextLines)
		contextifiedFrames = append(contextifiedFrames, cfi.addContextLinesToFrame(frame, lines, contextLine))
	}

	return contextifiedFrames
}

func (cfi *contextifyFramesIntegration) findNearbySourceCodeLocation(originalPath string) string {
	trimmedPath := strings.TrimPrefix(originalPath, "/")
	components := strings.Split(trimmedPath, "/")

	for len(components) > 0 {
		components = components[1:]
		possibleLocation := strings.Join(components, "/")

		if fileExists(possibleLocation) {
			cfi.cachedLocations[originalPath] = possibleLocation
			return possibleLocation
		}
	}

	cfi.cachedLocations[originalPath] = ""
	return ""
}

func (cfi *contextifyFramesIntegration) addContextLinesToFrame(frame Frame, lines [][]byte, contextLine int) Frame {
	for i, line := range lines {
		switch {
		case i < contextLine:
			frame.PreContext = append(frame.PreContext, string(line))
		case i == contextLine:
			frame.ContextLine = string(line)
		default:
			frame.PostContext = append(frame.PostContext, string(line))
		}
	}
	return frame
}
