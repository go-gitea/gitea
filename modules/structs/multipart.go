package structs

type MultipartObjectPart struct {
	Index int    `json:"index"`
	Pos   int64  `json:"pos"`
	Size  int64  `json:"size"`
	Etag  string `json:"etag,omitempty"`
	*MultipartEndpoint
}

type MultipartEndpoint struct {
	ExpiresIn         int                `json:"expires_in,omitempty"`
	Href              string             `json:"href,omitempty"`
	Method            string             `json:"method,omitempty"`
	Headers           *map[string]string `json:"headers,omitempty"`
	Params            *map[string]string `json:"params,omitempty"`
	AggregationParams *map[string]string `json:"aggregation_params,omitempty"`
}
