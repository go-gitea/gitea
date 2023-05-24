package mirror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRemoteUpdateOutput(t *testing.T) {
	tests := []struct {
		input   string
		results []*mirrorSyncResult
	}{
		{
			// create tag
			input: "From https://xxx.com/xxx/xxx\n * [new tag]         v1.4       -> v1.4\n",
			results: []*mirrorSyncResult{
				{
					refName:     "refs/tags/v1.4",
					oldCommitID: gitShortEmptySha,
					newCommitID: "",
				},
			},
		},
		{
			// delete tag and create branch
			input: "From https://xxx.com/xxx/xxx\n - [deleted]         (none)     -> v1.0.1\n * [new branch]      test/t3    -> test/t3\n * [new branch]      test/t4    -> test/t4\n",
			results: []*mirrorSyncResult{
				{
					refName:     "v1.0.1",
					oldCommitID: "",
					newCommitID: gitShortEmptySha,
				},
				{
					refName:     "refs/heads/test/t3",
					oldCommitID: gitShortEmptySha,
					newCommitID: "",
				},
				{
					refName:     "refs/heads/test/t4",
					oldCommitID: gitShortEmptySha,
					newCommitID: "",
				},
			},
		},
		{
			// delete branch
			input: "From https://xxx.com/xxx/xxx\n - [deleted]         (none)     -> test/t2\n",
			results: []*mirrorSyncResult{
				{
					refName:     "test/t2",
					oldCommitID: "",
					newCommitID: gitShortEmptySha,
				},
			},
		},
		{
			// new commits
			input: "From https://xxx.com/xxx/xxx\n   aeb77a8..425ec44  test/t3    -> test/t3\n   93b2801..3e1d4c2  test/t4    -> test/t4\n",
			results: []*mirrorSyncResult{
				{
					refName:     "refs/heads/test/t3",
					oldCommitID: "aeb77a8",
					newCommitID: "425ec44",
				},
				{
					refName:     "refs/heads/test/t4",
					oldCommitID: "93b2801",
					newCommitID: "3e1d4c2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			results := parseRemoteUpdateOutput(test.input)
			assert.EqualValues(t, test.results, results, fmt.Sprintf("%#v", results))
		})
	}
}
