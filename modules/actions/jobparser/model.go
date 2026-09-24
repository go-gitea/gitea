// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gitea.dev/actionslib/pkg/expreval"
	"gitea.dev/actionslib/pkg/exprparser"
	"gitea.dev/actionslib/pkg/model"
	"gitea.dev/modules/util"

	"github.com/robfig/cron/v3"
	"go.yaml.in/yaml/v4"
)

// SingleWorkflow is a workflow with single job and single matrix
type SingleWorkflow struct {
	Name           string    `yaml:"name,omitempty"`
	RawOn          yaml.Node `yaml:"on,omitempty"`
	Env            yaml.Node `yaml:"env,omitempty"`
	RawJobs        yaml.Node `yaml:"jobs,omitempty"`
	Defaults       Defaults  `yaml:"defaults,omitempty"`
	RawPermissions yaml.Node `yaml:"permissions,omitempty"`
	RunName        string    `yaml:"run-name,omitempty"`
}

func (w *SingleWorkflow) Job() (string, *Job) {
	ids, jobs, _ := w.jobs()
	if len(ids) >= 1 {
		return ids[0], jobs[0]
	}
	return "", nil
}

// WorkflowDispatchConfig returns the `on: workflow_dispatch` declaration, nil if there is none.
func (w *SingleWorkflow) WorkflowDispatchConfig() *model.WorkflowDispatch {
	return (&model.Workflow{RawOn: w.RawOn}).WorkflowDispatchConfig()
}

func (w *SingleWorkflow) jobs() ([]string, []*Job, error) {
	ids, jobs, err := parseMappingNode[*Job](&w.RawJobs)
	if err != nil {
		return nil, nil, err
	}

	for _, job := range jobs {
		steps := make([]*Step, 0, len(job.Steps))
		for _, s := range job.Steps {
			if s != nil {
				steps = append(steps, s)
			}
		}
		job.Steps = steps
	}

	return ids, jobs, nil
}

func (w *SingleWorkflow) SetJob(id string, job *Job) error {
	m := map[string]*Job{
		id: job,
	}
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(m); err != nil {
		return err
	}
	encoder.Close()

	node := yaml.Node{}
	if err := yaml.Unmarshal(buf.Bytes(), &node); err != nil {
		return err
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("can not set job: %s", buf.String())
	}
	w.RawJobs = *node.Content[0]
	return nil
}

func (w *SingleWorkflow) Marshal() ([]byte, error) {
	// Encode with the same indentation SetJob uses (2). yaml.Marshal's default
	// indentation (4) makes the encoder emit multi-line block scalars (e.g. a
	// `run:` step that begins with blank lines) with a wrong explicit indentation
	// indicator (`run: |4`) that then fails to re-parse, which silently strands
	// the job during concurrency evaluation. Keeping both encoders at indent 2
	// makes the serialized single workflow round-trip.
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	payload := *w
	payload.RunName = "" // already interpolated into the run title, a runner would parse it as a template again
	if err := enc.Encode(&payload); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type Job struct {
	Name               string                `yaml:"name,omitempty"`
	RawNeeds           yaml.Node             `yaml:"needs,omitempty"`
	RawRunsOn          yaml.Node             `yaml:"runs-on,omitempty"`
	Env                yaml.Node             `yaml:"env,omitempty"`
	If                 yaml.Node             `yaml:"if,omitempty"`
	Steps              []*Step               `yaml:"steps,omitempty"`
	TimeoutMinutes     string                `yaml:"timeout-minutes,omitempty"`
	RawContinueOnError yaml.Node             `yaml:"continue-on-error,omitempty"`
	Services           yaml.Node             `yaml:"services,omitempty"`
	Strategy           Strategy              `yaml:"strategy,omitempty"`
	RawContainer       yaml.Node             `yaml:"container,omitempty"`
	Defaults           yaml.Node             `yaml:"defaults,omitempty"`
	Outputs            map[string]string     `yaml:"outputs,omitempty"`
	Uses               string                `yaml:"uses,omitempty"`
	With               yaml.Node             `yaml:"with,omitempty"`
	RawSecrets         yaml.Node             `yaml:"secrets,omitempty"`
	RawConcurrency     *model.RawConcurrency `yaml:"concurrency,omitempty"`
	RawPermissions     yaml.Node             `yaml:"permissions,omitempty"`
}

// GetContinueOnError decodes the continue-on-error field to a bool.
// The field may be a literal bool or an already-evaluated expression node.
func (j *Job) GetContinueOnError() bool {
	if j.RawContinueOnError.Kind == 0 {
		return false
	}
	var v bool
	if err := j.RawContinueOnError.Decode(&v); err != nil {
		return false
	}
	return v
}

func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	return &Job{
		Name:               j.Name,
		RawNeeds:           j.RawNeeds,
		RawRunsOn:          j.RawRunsOn,
		Env:                j.Env,
		If:                 j.If,
		Steps:              j.Steps,
		TimeoutMinutes:     j.TimeoutMinutes,
		RawContinueOnError: j.RawContinueOnError,
		Services:           j.Services,
		Strategy:           j.Strategy,
		RawContainer:       j.RawContainer,
		Defaults:           j.Defaults,
		Outputs:            j.Outputs,
		Uses:               j.Uses,
		With:               j.With,
		RawSecrets:         j.RawSecrets,
		RawConcurrency:     j.RawConcurrency,
		RawPermissions:     j.RawPermissions,
	}
}

func (j *Job) Needs() []string {
	return (&model.Job{RawNeeds: j.RawNeeds}).Needs()
}

func (j *Job) EraseNeeds() *Job {
	j.RawNeeds = yaml.Node{}
	return j
}

// RunsOn returns the labels Gitea matches runners against, unescaped like DisplayName.
func (j *Job) RunsOn() []string {
	runsOn := model.RunsOnFromNode(j.RawRunsOn)
	for i, label := range runsOn {
		runsOn[i] = unescapeExpressions(label)
	}
	return runsOn
}

// DisplayName is the name Gitea stores, without the escaping the payload keeps for runners.
func (j *Job) DisplayName() string {
	return util.EllipsisDisplayString(unescapeExpressions(j.Name), 255)
}

// BlockSafeString works around https://github.com/yaml/go-yaml/issues/399, quoting a value whose
// leading newline would cost a literal block scalar its indentation indicator.
type BlockSafeString string

func (s BlockSafeString) MarshalYAML() (any, error) {
	if !strings.HasPrefix(string(s), "\n") {
		return string(s), nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Value: string(s)}, nil
}

type Step struct {
	ID                 string          `yaml:"id,omitempty"`
	If                 yaml.Node       `yaml:"if,omitempty"`
	Name               BlockSafeString `yaml:"name,omitempty"`
	Uses               string          `yaml:"uses,omitempty"`
	Run                BlockSafeString `yaml:"run,omitempty"`
	WorkingDirectory   string          `yaml:"working-directory,omitempty"`
	Shell              string          `yaml:"shell,omitempty"`
	Env                yaml.Node       `yaml:"env,omitempty"`
	With               yaml.Node       `yaml:"with,omitempty"`
	RawContinueOnError yaml.Node       `yaml:"continue-on-error,omitempty"` // raw: the runner evaluates it with the steps context
	TimeoutMinutes     string          `yaml:"timeout-minutes,omitempty"`
}

// UnmarshalYAML canonicalizes booleans like continue-on-error
func (s *Step) UnmarshalYAML(node *yaml.Node) error {
	type rawStep Step
	if err := node.Decode((*rawStep)(s)); err != nil {
		return err
	}
	if raw := &s.RawContinueOnError; raw.Tag == "!!bool" {
		raw.Value = strings.ToLower(raw.Value)
	}
	return nil
}

// String gets the name of step
func (s *Step) String() string {
	if s == nil {
		return ""
	}
	return (&model.Step{
		ID:   s.ID,
		Name: string(s.Name),
		Uses: s.Uses,
		Run:  string(s.Run),
	}).String()
}

type Strategy struct {
	FailFastString    string    `yaml:"fail-fast,omitempty"`
	MaxParallelString string    `yaml:"max-parallel,omitempty"`
	RawMatrix         yaml.Node `yaml:"matrix,omitempty"`
	JobIndex          int       `yaml:"job-index,omitempty"` // set by buildMatrixCombos, read back from its payload
	JobTotal          int       `yaml:"job-total,omitempty"`
	RawExpression     yaml.Node `yaml:"-"` // a whole-value `strategy: ${{ }}`, see Strategy.resolve
}

type rawStrategy Strategy

func (s *Strategy) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*s = Strategy{RawExpression: *node}
		return nil
	}
	return node.Decode((*rawStrategy)(s))
}

func (s Strategy) MarshalYAML() (any, error) {
	if s.RawExpression.Kind != 0 {
		return &s.RawExpression, nil
	}
	return rawStrategy(s), nil
}

func (s Strategy) actStrategy() *model.Strategy {
	return &model.Strategy{FailFastString: s.FailFastString, MaxParallelString: s.MaxParallelString, RawMatrix: s.RawMatrix}
}

// context is the strategy context, combination 0 of 1 without a matrix as on GitHub, and without job-index for an unexpanded matrix.
func (s *Strategy) context() map[string]any {
	switch {
	case s == nil:
		return exprparser.StrategyContext(nil, 0, 0)
	case s.RawMatrix.Kind == 0 && s.RawExpression.Kind == 0:
		return exprparser.StrategyContext(s.actStrategy(), 0, 1)
	}
	return exprparser.StrategyContext(s.actStrategy(), s.JobIndex, s.JobTotal)
}

type Defaults struct {
	Run RunDefaults `yaml:"run,omitempty"`
}

type RunDefaults struct {
	Shell            string `yaml:"shell,omitempty"`
	WorkingDirectory string `yaml:"working-directory,omitempty"`
}

type Event struct {
	Name      string
	acts      map[string][]string
	schedules []map[string]string
}

func (evt *Event) IsSchedule() bool {
	return evt.schedules != nil
}

func (evt *Event) Acts() map[string][]string {
	return evt.acts
}

func (evt *Event) Schedules() []map[string]string {
	return evt.schedules
}

func ReadWorkflowRawConcurrency(content []byte) (*model.RawConcurrency, error) {
	w, err := ReadWorkflow(content)
	if err != nil {
		return nil, err
	}
	return w.RawConcurrency, nil
}

// newJobEvaluator evaluates against a stored job's contexts, with its single matrix combination.
func newJobEvaluator(jobID string, job *Job, gitCtx map[string]any, results map[string]*JobResult, vars map[string]string, inputs map[string]any) (expreval.Evaluator, error) {
	var strategy *Strategy
	var matrix map[string]any
	if job != nil {
		strategy = &job.Strategy
		rawMatrix := model.CloneYamlNode(job.Strategy.RawMatrix)
		replaceScalars(&rawMatrix, unescapeExpressions)
		matrixes, err := (&model.Job{Strategy: &model.Strategy{RawMatrix: rawMatrix}}).GetMatrixes()
		if err != nil {
			return expreval.Evaluator{}, err
		}
		if len(matrixes[0]) > 0 {
			matrix = matrixes[0]
		}
	}
	return expreval.New(NewInterpeter(jobID, strategy, matrix, model.GithubContextFromMap(gitCtx), results, vars, inputs).Evaluate), nil
}

func EvaluateConcurrency(rc *model.RawConcurrency, jobID string, job *Job, gitCtx map[string]any, results map[string]*JobResult, vars map[string]string, inputs map[string]any) (string, bool, error) {
	evaluator, err := newJobEvaluator(jobID, job, gitCtx, results, vars, inputs)
	if err != nil {
		return "", false, err
	}
	var node yaml.Node
	if err := node.Encode(rc); err != nil {
		return "", false, fmt.Errorf("failed to encode concurrency: %w", err)
	}
	if err := evaluator.EvaluateYamlNode(&node); err != nil {
		return "", false, fmt.Errorf("failed to evaluate concurrency: %w", err)
	}
	var evaluated model.RawConcurrency
	if err := node.Decode(&evaluated); err != nil {
		return "", false, fmt.Errorf("failed to unmarshal evaluated concurrency: %w", err)
	}
	if evaluated.RawExpression != "" {
		return evaluated.RawExpression, false, nil
	}
	return evaluated.Group, util.ParseYamlBool(evaluated.CancelInProgress), nil
}

// workflowCallEvent is only fired by another workflow's `uses:`, so it is excluded from trigger detection.
const workflowCallEvent = "workflow_call"

func ParseRawOn(rawOn *yaml.Node) ([]*Event, error) {
	switch rawOn.Kind {
	case yaml.ScalarNode:
		var val string
		err := rawOn.Decode(&val)
		if err != nil {
			return nil, err
		}
		if rawOn.ShortTag() != "!!str" || val == "" {
			return nil, fmt.Errorf("invalid event %q", val)
		}
		if val == workflowCallEvent {
			return []*Event{}, nil
		}
		return []*Event{
			{Name: val},
		}, nil
	case yaml.SequenceNode:
		var val []any
		err := rawOn.Decode(&val)
		if err != nil {
			return nil, err
		}
		res := make([]*Event, 0, len(val))
		for _, v := range val {
			switch t := v.(type) {
			case string:
				if t == workflowCallEvent {
					continue
				}
				res = append(res, &Event{Name: t})
			default:
				return nil, fmt.Errorf("invalid type %T", t)
			}
		}
		return res, nil
	case yaml.MappingNode:
		events, triggers, err := parseMappingNode[yaml.Node](rawOn)
		if err != nil {
			return nil, err
		}
		res := make([]*Event, 0, len(events))
		for i, k := range events {
			if k == workflowCallEvent {
				continue
			}
			v := triggers[i]
			switch v.Kind {
			case yaml.ScalarNode:
				res = append(res, &Event{
					Name: k,
				})
			case yaml.SequenceNode:
				var t []any
				err := v.Decode(&t)
				if err != nil {
					return nil, err
				}
				schedules := make([]map[string]string, len(t))
				if k == "schedule" {
					if len(t) == 0 {
						return nil, errors.New("schedule must contain at least one cron entry")
					}
					for i, tt := range t {
						vv, ok := tt.(map[string]any)
						if !ok {
							return nil, errors.New("unknown on type(schedule)")
						}
						schedules[i] = make(map[string]string, len(vv))
						for k, vvv := range vv {
							var ok bool
							if schedules[i][k], ok = vvv.(string); !ok {
								return nil, errors.New("unknown on type(schedule)")
							}
						}
						if _, err := cron.ParseStandard(schedules[i]["cron"]); err != nil {
							return nil, fmt.Errorf("invalid cron %q: %w", schedules[i]["cron"], err)
						}
					}
				}

				if len(schedules) == 0 {
					schedules = nil
				}
				res = append(res, &Event{
					Name:      k,
					schedules: schedules,
				})
			case yaml.MappingNode:
				// Keep combined include and ignore filters for existing Gitea workflows, although GitHub rejects them.
				acts := make(map[string][]string, len(v.Content)/2)
				expectedKey := true
				var act string
				for _, content := range v.Content {
					if expectedKey {
						if content.Kind != yaml.ScalarNode {
							return nil, errors.New("key type not string")
						}
						act = ""
						err := content.Decode(&act)
						if err != nil {
							return nil, err
						}
					} else {
						switch content.Kind {
						case yaml.SequenceNode:
							var t []string
							err := content.Decode(&t)
							if err != nil {
								return nil, err
							}
							acts[act] = t
						case yaml.ScalarNode:
							var t string
							err := content.Decode(&t)
							if err != nil {
								return nil, err
							}
							acts[act] = []string{t}
						case yaml.MappingNode:
							if k != "workflow_dispatch" || act != "inputs" {
								return nil, fmt.Errorf("map should only for workflow_dispatch but %s", act)
							}
							if err := content.Decode(new(map[string]model.WorkflowDispatchInput)); err != nil {
								return nil, err
							}
						default:
							return nil, fmt.Errorf("unknown on type for %s", act)
						}
					}
					expectedKey = !expectedKey
				}
				if len(acts) == 0 {
					acts = nil
				}
				res = append(res, &Event{
					Name: k,
					acts: acts,
				})
			default:
				return nil, fmt.Errorf("unknown on type: %v", v.Kind)
			}
		}
		return res, nil
	default:
		return nil, fmt.Errorf("unknown on type: %v", rawOn.Kind)
	}
}

// EvaluateJobIfExpression evaluates a job's `if:`, which github.com decides before the matrix, so without the matrix and strategy contexts.
func EvaluateJobIfExpression(jobID string, job *Job, gitCtx map[string]any, results map[string]*JobResult, vars map[string]string, inputs map[string]any) (bool, error) {
	if unavailable := unavailableContext(IfExpression(job.If.Value), jobConditionContexts); unavailable != "" { // only a job stored before its conditions were validated
		return false, fmt.Errorf("job %s: Unrecognized named-value: '%s', update the workflow and trigger a new run", jobID, unavailable)
	}
	evaluator := expreval.New(NewInterpeter(jobID, nil, nil, model.GithubContextFromMap(gitCtx), results, vars, inputs).Evaluate)
	return evaluator.EvalBool(job.If.Value, exprparser.DefaultStatusCheckSuccess)
}

// parseMappingNode parse a mapping node and preserve order.
func parseMappingNode[T any](node *yaml.Node) ([]string, []T, error) {
	if node.Kind != yaml.MappingNode {
		return nil, nil, errors.New("input node is not a mapping node")
	}

	var scalars []string
	var datas []T
	expectKey := true
	for _, item := range node.Content {
		if expectKey {
			if item.Kind != yaml.ScalarNode {
				return nil, nil, fmt.Errorf("not a valid scalar node: %v", item.Value)
			}
			scalars = append(scalars, item.Value)
			expectKey = false
		} else {
			var val T
			if err := item.Decode(&val); err != nil {
				return nil, nil, err
			}
			datas = append(datas, val)
			expectKey = true
		}
	}

	if len(scalars) != len(datas) {
		return nil, nil, fmt.Errorf("invalid definition of on: %v", node.Value)
	}

	return scalars, datas, nil
}
