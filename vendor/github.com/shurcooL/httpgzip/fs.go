package httpgzip

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"sort"
	"strings"
	"time"
)

// FileServer returns a handler that serves HTTP requests
// with the contents of the file system rooted at root.
// Additional optional behaviors can be controlled via opt.
func FileServer(root http.FileSystem, opt FileServerOptions) http.Handler {
	if opt.ServeError == nil {
		opt.ServeError = defaults.ServeError
	}
	return &fileServer{root: root, opt: opt}
}

var defaults = FileServerOptions{
	ServeError: NonSpecific,
}

// FileServerOptions specifies options for FileServer.
type FileServerOptions struct {
	// IndexHTML controls special handling of "index.html" file.
	IndexHTML bool

	// ServeError is used to serve errors coming from underlying file system.
	// If called, it's guaranteed to be before anything has been written
	// to w by FileServer, so it's safe to use http.Error.
	// If nil, then NonSpecific is used.
	ServeError func(w http.ResponseWriter, req *http.Request, err error)
}

var (
	// NonSpecific serves a non-specific HTTP error message and status code
	// for a given non-nil error value. It's important that NonSpecific does not
	// actually include err.Error(), since its goal is to not leak information
	// in error messages to users.
	NonSpecific = func(w http.ResponseWriter, req *http.Request, err error) {
		switch {
		case os.IsNotExist(err):
			http.Error(w, "404 Not Found", http.StatusNotFound)
		case os.IsPermission(err):
			http.Error(w, "403 Forbidden", http.StatusForbidden)
		default:
			http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
		}
	}

	// Detailed serves detailed HTTP error message and status code for a given
	// non-nil error value. Because err.Error() is displayed to users, it should
	// be used in development only, or if you're confident there won't be sensitive
	// information in the underlying file system error messages.
	Detailed = func(w http.ResponseWriter, req *http.Request, err error) {
		switch {
		case os.IsNotExist(err):
			http.Error(w, "404 Not Found\n\n"+err.Error(), http.StatusNotFound)
		case os.IsPermission(err):
			http.Error(w, "403 Forbidden\n\n"+err.Error(), http.StatusForbidden)
		default:
			http.Error(w, "500 Internal Server Error\n\n"+err.Error(), http.StatusInternalServerError)
		}
	}
)

type fileServer struct {
	root http.FileSystem
	opt  FileServerOptions
}

func (fs *fileServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		w.Header().Set("Allow", "GET")
		http.Error(w, "405 Method Not Allowed\n\nmethod should be GET", http.StatusMethodNotAllowed)
		return
	}

	// Already cleaned by net/http.cleanPath, but caller can be some middleware.
	path := pathpkg.Clean("/" + req.URL.Path)

	if fs.opt.IndexHTML {
		// Redirect .../index.html to .../.
		// Can't use Redirect() because that would make the path absolute,
		// which would be a problem running under StripPrefix.
		if strings.HasSuffix(path, "/index.html") {
			localRedirect(w, req, ".")
			return
		}
	}

	f, err := fs.root.Open(path)
	if err != nil {
		fs.opt.ServeError(w, req, err)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fs.opt.ServeError(w, req, err)
		return
	}

	// Redirect to canonical path: / at end of directory url.
	url := req.URL.Path
	if fi.IsDir() {
		if !strings.HasSuffix(url, "/") && url != "" {
			localRedirect(w, req, pathpkg.Base(url)+"/")
			return
		}
	} else {
		if strings.HasSuffix(url, "/") && url != "/" {
			localRedirect(w, req, "../"+pathpkg.Base(url))
			return
		}
	}

	if fs.opt.IndexHTML {
		// Use contents of index.html for directory, if present.
		if fi.IsDir() {
			indexPath := pathpkg.Join(path, "index.html")
			f0, err := fs.root.Open(indexPath)
			if err == nil {
				defer f0.Close()
				fi0, err := f0.Stat()
				if err == nil {
					path = indexPath
					f = f0
					fi = fi0
				}
			}
		}
	}

	// A directory?
	if fi.IsDir() {
		if checkLastModified(w, req, fi.ModTime()) {
			return
		}
		err := dirList(w, f, path == "/")
		if err != nil {
			fs.opt.ServeError(w, req, err)
		}
		return
	}

	ServeContent(w, req, fi.Name(), fi.ModTime(), f)
}

func dirList(w http.ResponseWriter, f http.File, root bool) error {
	dirs, err := f.Readdir(0)
	if err != nil {
		return fmt.Errorf("error reading directory: %v", err)
	}
	sort.Sort(byName(dirs))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintln(w, "<pre>")
	switch root {
	case true:
		fmt.Fprintln(w, `<a href=".">.</a>`)
	case false:
		fmt.Fprintln(w, `<a href="..">..</a>`)
	}
	for _, d := range dirs {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		// name may contain '?' or '#', which must be escaped to remain
		// part of the URL path, and not indicate the start of a query
		// string or fragment.
		url := url.URL{Path: name}
		fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", url.String(), html.EscapeString(name))
	}
	fmt.Fprintln(w, "</pre>")
	return nil
}

// localRedirect gives a Moved Permanently response.
// It does not convert relative paths to absolute paths like http.Redirect does.
func localRedirect(w http.ResponseWriter, req *http.Request, newPath string) {
	if req.URL.RawQuery != "" {
		newPath += "?" + req.URL.RawQuery
	}
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

var unixEpochTime = time.Unix(0, 0)

// modTime is the modification time of the resource to be served, or IsZero().
// return value is whether this request is now complete.
func checkLastModified(w http.ResponseWriter, req *http.Request, modTime time.Time) bool {
	if modTime.IsZero() || modTime.Equal(unixEpochTime) {
		// If the file doesn't have a modTime (IsZero), or the modTime
		// is obviously garbage (Unix time == 0), then ignore modtimes
		// and don't process the If-Modified-Since header.
		return false
	}

	// The Date-Modified header truncates sub-second precision, so
	// use mtime < t+1s instead of mtime <= t to check for unmodified.
	if t, err := time.Parse(http.TimeFormat, req.Header.Get("If-Modified-Since")); err == nil && modTime.Before(t.Add(1*time.Second)) {
		h := w.Header()
		delete(h, "Content-Type")
		delete(h, "Content-Length")
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	return false
}

// byName implements sort.Interface.
type byName []os.FileInfo

func (s byName) Len() int           { return len(s) }
func (s byName) Less(i, j int) bool { return s[i].Name() < s[j].Name() }
func (s byName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
