package cicdfeedback

type WorkflowInfo struct {
	ID            string `json:"id"`
	PipelineURI   string `json:"pipeline_uri"`
	Authorization string `json:"authorization,omitempty"`
}
