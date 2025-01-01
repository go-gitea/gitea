package jobparser

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nektos/act/pkg/model"
)

func Parse(content []byte, options ...ParseOption) ([]*SingleWorkflow, error) {
	origin, err := model.ReadWorkflow(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("model.ReadWorkflow: %w", err)
	}

	workflow := &SingleWorkflow{}
	if err := yaml.Unmarshal(content, workflow); err != nil {
		return nil, fmt.Errorf("yaml.Unmarshal: %w", err)
	}

	pc := &parseContext{}
	for _, o := range options {
		o(pc)
	}
	results := map[string]*JobResult{}
	for id, job := range origin.Jobs {
		results[id] = &JobResult{
			Needs:   job.Needs(),
			Result:  pc.jobResults[id],
			Outputs: nil, // not supported yet
		}
	}

	var ret []*SingleWorkflow
	ids, jobs, err := workflow.jobs()
	if err != nil {
		return nil, fmt.Errorf("invalid jobs: %w", err)
	}
	for i, id := range ids {
		job := jobs[i]
		matricxes, err := getMatrixes(origin.GetJob(id))
		if err != nil {
			return nil, fmt.Errorf("getMatrixes: %w", err)
		}
		for _, matrix := range matricxes {
			job := job.Clone()
			if job.Name == "" {
				job.Name = id
			}
			job.Strategy.RawMatrix = encodeMatrix(matrix)
			evaluator := NewExpressionEvaluator(NewInterpeter(id, origin.GetJob(id), matrix, pc.gitContext, results, pc.vars))
			job.Name = nameWithMatrix(job.Name, matrix, evaluator)
			runsOn := origin.GetJob(id).RunsOn()
			for i, v := range runsOn {
				runsOn[i] = evaluator.Interpolate(v)
			}
			job.RawRunsOn = encodeRunsOn(runsOn)
			swf := &SingleWorkflow{
				Name:     workflow.Name,
				RawOn:    workflow.RawOn,
				Env:      workflow.Env,
				Defaults: workflow.Defaults,
			}
			if err := swf.SetJob(id, job); err != nil {
				return nil, fmt.Errorf("SetJob: %w", err)
			}
			ret = append(ret, swf)
		}
	}
	return ret, nil
}

func WithJobResults(results map[string]string) ParseOption {
	return func(c *parseContext) {
		c.jobResults = results
	}
}

func WithGitContext(context *model.GithubContext) ParseOption {
	return func(c *parseContext) {
		c.gitContext = context
	}
}

func WithVars(vars map[string]string) ParseOption {
	return func(c *parseContext) {
		c.vars = vars
	}
}

type parseContext struct {
	jobResults map[string]string
	gitContext *model.GithubContext
	vars       map[string]string
}

type ParseOption func(c *parseContext)

func getMatrixes(job *model.Job) ([]map[string]interface{}, error) {
	ret, err := job.GetMatrixes()
	if err != nil {
		return nil, fmt.Errorf("GetMatrixes: %w", err)
	}
	sort.Slice(ret, func(i, j int) bool {
		return matrixName(ret[i]) < matrixName(ret[j])
	})
	return ret, nil
}

func encodeMatrix(matrix map[string]interface{}) yaml.Node {
	if len(matrix) == 0 {
		return yaml.Node{}
	}
	value := map[string][]interface{}{}
	for k, v := range matrix {
		value[k] = []interface{}{v}
	}
	node := yaml.Node{}
	_ = node.Encode(value)
	return node
}

func encodeRunsOn(runsOn []string) yaml.Node {
	node := yaml.Node{}
	if len(runsOn) == 1 {
		_ = node.Encode(runsOn[0])
	} else {
		_ = node.Encode(runsOn)
	}
	return node
}

func nameWithMatrix(name string, m map[string]interface{}, evaluator *ExpressionEvaluator) string {
	if len(m) == 0 {
		return name
	}

	if !strings.Contains(name, "${{") || !strings.Contains(name, "}}") {
		return name + " " + matrixName(m)
	}

	return evaluator.Interpolate(name)
}

func matrixName(m map[string]interface{}) string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	vs := make([]string, 0, len(m))
	for _, v := range ks {
		vs = append(vs, fmt.Sprint(m[v]))
	}

	return fmt.Sprintf("(%s)", strings.Join(vs, ", "))
}
