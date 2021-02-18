package transport

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/99designs/gqlgen/graphql"
)

// MultipartForm the Multipart request spec https://github.com/jaydenseric/graphql-multipart-request-spec
type MultipartForm struct {
	// MaxUploadSize sets the maximum number of bytes used to parse a request body
	// as multipart/form-data.
	MaxUploadSize int64

	// MaxMemory defines the maximum number of bytes used to parse a request body
	// as multipart/form-data in memory, with the remainder stored on disk in
	// temporary files.
	MaxMemory int64
}

var _ graphql.Transport = MultipartForm{}

func (f MultipartForm) Supports(r *http.Request) bool {
	if r.Header.Get("Upgrade") != "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}

	return r.Method == "POST" && mediaType == "multipart/form-data"
}

func (f MultipartForm) maxUploadSize() int64 {
	if f.MaxUploadSize == 0 {
		return 32 << 20
	}
	return f.MaxUploadSize
}

func (f MultipartForm) maxMemory() int64 {
	if f.MaxMemory == 0 {
		return 32 << 20
	}
	return f.MaxMemory
}

func (f MultipartForm) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	w.Header().Set("Content-Type", "application/json")

	start := graphql.Now()

	var err error
	if r.ContentLength > f.maxUploadSize() {
		writeJsonError(w, "failed to parse multipart form, request body too large")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, f.maxUploadSize())
	if err = r.ParseMultipartForm(f.maxMemory()); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		if strings.Contains(err.Error(), "request body too large") {
			writeJsonError(w, "failed to parse multipart form, request body too large")
			return
		}
		writeJsonError(w, "failed to parse multipart form")
		return
	}
	defer r.Body.Close()

	var params graphql.RawParams

	if err = jsonDecode(strings.NewReader(r.Form.Get("operations")), &params); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "operations form field could not be decoded")
		return
	}

	var uploadsMap = map[string][]string{}
	if err = json.Unmarshal([]byte(r.Form.Get("map")), &uploadsMap); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeJsonError(w, "map form field could not be decoded")
		return
	}

	var upload graphql.Upload
	for key, paths := range uploadsMap {
		if len(paths) == 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeJsonErrorf(w, "invalid empty operations paths list for key %s", key)
			return
		}
		file, header, err := r.FormFile(key)
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			writeJsonErrorf(w, "failed to get key %s from form", key)
			return
		}
		defer file.Close()

		if len(paths) == 1 {
			upload = graphql.Upload{
				File:        file,
				Size:        header.Size,
				Filename:    header.Filename,
				ContentType: header.Header.Get("Content-Type"),
			}

			if err := params.AddUpload(upload, key, paths[0]); err != nil {
				w.WriteHeader(http.StatusUnprocessableEntity)
				writeJsonGraphqlError(w, err)
				return
			}
		} else {
			if r.ContentLength < f.maxMemory() {
				fileBytes, err := ioutil.ReadAll(file)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonErrorf(w, "failed to read file for key %s", key)
					return
				}
				for _, path := range paths {
					upload = graphql.Upload{
						File:        &bytesReader{s: &fileBytes, i: 0, prevRune: -1},
						Size:        header.Size,
						Filename:    header.Filename,
						ContentType: header.Header.Get("Content-Type"),
					}

					if err := params.AddUpload(upload, key, path); err != nil {
						w.WriteHeader(http.StatusUnprocessableEntity)
						writeJsonGraphqlError(w, err)
						return
					}
				}
			} else {
				tmpFile, err := ioutil.TempFile(os.TempDir(), "gqlgen-")
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonErrorf(w, "failed to create temp file for key %s", key)
					return
				}
				tmpName := tmpFile.Name()
				defer func() {
					_ = os.Remove(tmpName)
				}()
				_, err = io.Copy(tmpFile, file)
				if err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					if err := tmpFile.Close(); err != nil {
						writeJsonErrorf(w, "failed to copy to temp file and close temp file for key %s", key)
						return
					}
					writeJsonErrorf(w, "failed to copy to temp file for key %s", key)
					return
				}
				if err := tmpFile.Close(); err != nil {
					w.WriteHeader(http.StatusUnprocessableEntity)
					writeJsonErrorf(w, "failed to close temp file for key %s", key)
					return
				}
				for _, path := range paths {
					pathTmpFile, err := os.Open(tmpName)
					if err != nil {
						w.WriteHeader(http.StatusUnprocessableEntity)
						writeJsonErrorf(w, "failed to open temp file for key %s", key)
						return
					}
					defer pathTmpFile.Close()
					upload = graphql.Upload{
						File:        pathTmpFile,
						Size:        header.Size,
						Filename:    header.Filename,
						ContentType: header.Header.Get("Content-Type"),
					}

					if err := params.AddUpload(upload, key, path); err != nil {
						w.WriteHeader(http.StatusUnprocessableEntity)
						writeJsonGraphqlError(w, err)
						return
					}
				}
			}
		}
	}

	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	rc, gerr := exec.CreateOperationContext(r.Context(), &params)
	if gerr != nil {
		resp := exec.DispatchError(graphql.WithOperationContext(r.Context(), rc), gerr)
		w.WriteHeader(statusFor(gerr))
		writeJson(w, resp)
		return
	}
	responses, ctx := exec.DispatchOperation(r.Context(), rc)
	writeJson(w, responses(ctx))
}
