package public

import (
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/storage"

	"gitea.com/macaron/macaron"
)

// AvatarOptions represents the available options to configure the AvatarHandler.
type AvatarOptions struct {
	Prefix      string
	SkipLogging bool
	// if set to true, will enable caching. Expires header will also be set to
	// expire after the defined time.
	ExpiresAfter time.Duration
}

// AvatarHandler implements the macaron handler for serving user avatars and repo-avatars.
func AvatarHandler(opts *AvatarOptions) macaron.Handler {
	// Normalize the prefix if provided
	if opts.Prefix != "" {
		// Ensure we have a leading '/'
		if opts.Prefix[0] != '/' {
			opts.Prefix = "/" + opts.Prefix
		}
		// Remove any trailing '/'
		opts.Prefix = strings.TrimRight(opts.Prefix, "/")
	}
	return func(ctx *macaron.Context, log *log.Logger) {
		opts.handle(ctx, log)
	}
}

func (opts *AvatarOptions) handle(ctx *macaron.Context, log *log.Logger) bool {
	if ctx.Req.Method != "GET" && ctx.Req.Method != "HEAD" {
		return false
	}

	avatarURL := ctx.Req.URL
	if opts.Prefix != avatarURL.Path {
		return false
	}
	objPath := avatarURL.Query().Get("obj")
	bucketURL := filepath.Dir(objPath) + "/"
	objKey := filepath.Base(objPath)

	fs := storage.FileStorage{
		Ctx:      ctx.Req.Context(),
		Path:     bucketURL,
		FileName: objKey,
	}

	attrs, err := fs.Attributes()
	if err != nil {
		ctx.Resp.WriteHeader(http.StatusNotFound)
		return true
	}

	if !opts.SkipLogging {
		log.Println("[Static] Serving " + objKey)
	}

	// Add an Expires header to the static content
	if opts.ExpiresAfter > 0 {
		ctx.Resp.Header().Set("Expires", time.Now().Add(opts.ExpiresAfter).UTC().Format(http.TimeFormat))
		tag := GenerateETag(string(attrs.Size), objKey, attrs.ModTime.UTC().Format(http.TimeFormat))
		ctx.Resp.Header().Set("ETag", tag)
		if ctx.Req.Header.Get("If-None-Match") == tag {
			ctx.Resp.WriteHeader(http.StatusNotModified)
			return false
		}
	}

	fr, err := fs.NewReader()
	if err != nil {
		ctx.WriteHeader(http.StatusNotFound)
		return true
	}
	defer fr.Close()
	if _, err := io.Copy(ctx.Resp, fr); err != nil {
		ctx.WriteHeader(http.StatusNotFound)
	}
	return true
}
