package transport

import (
	"mime"
	"net/http"

	"github.com/99designs/gqlgen/graphql"
)

// POST implements the POST side of the default HTTP transport
// defined in https://github.com/APIs-guru/graphql-over-http#post
type POST struct{}

var _ graphql.Transport = POST{}

func (h POST) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}

	return r.Method == "POST" && mediaType == "application/json"
}

func (h POST) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	var params *graphql.RawParams
	start := graphql.Now()
	if err := jsonDecode(r.Body, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJsonErrorf(w, "json body could not be decoded: "+err.Error())
		return
	}
	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, err := exec.CreateOperationContext(r.Context(), params)
	if err != nil {
		w.WriteHeader(statusFor(err))
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), err)
		writeJson(w, resp)
		return
	}
	ctx := graphql.WithOperationContext(r.Context(), rc)
	responses, ctx := exec.DispatchOperation(ctx, rc)
	writeJson(w, responses(ctx))
}
