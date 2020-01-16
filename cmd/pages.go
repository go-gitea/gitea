// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"

	"github.com/urfave/cli"
)

var pageURL = regexp.MustCompile(`^/([\w-]+)/([\w-]+)/?(.*)$`)

// CmdPages represents the available pagees sub-command.
var CmdPages = cli.Command{
	Name:        "pages",
	Usage:       "Serve a branch on repositories as a static sites",
	Description: "A command proxy a branch to serve as pages service (static webserver).",
	Action:      runPages,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "remote, r",
			Value: "http://127.0.0.1:3000",
			Usage: "Gitea instance url",
		},
		cli.StringFlag{
			Name:  "branch, b",
			Value: "pages",
			Usage: "Branch name to serve",
		},
		cli.StringFlag{
			Name:  "listen, l",
			Value: "0.0.0.0:8080",
			Usage: "Listening address:port",
		},
		cli.StringFlag{
			Name:  "token, t",
			Value: "",
			Usage: "AccessToken to use", //TODO doesn't seem to works and may need fix on raw gitea handleling
		},
		/*
			cli.StringFlag{
				Name:  "tls",
				Usage: "Activate tls", //TODO "" -> http "file://filepath" -> https "le://domain" -> https + let's encrypt
			},
		*/
		//TODO remove default flag --custom-path --config --work-path
	},
}

func runPages(ctx *cli.Context) error {
	managerCtx, cancel := context.WithCancel(context.Background())
	graceful.InitManager(managerCtx)
	defer cancel()

	listenAddr := ctx.String("listen")
	log.Info("Listen: http://%s", listenAddr)
	err := runHTTP("tcp", listenAddr, &pagesHandler{
		remote: ctx.String("remote"),
		branch: ctx.String("branch"),
		token:  ctx.String("token"),
	})
	if err != nil {
		log.Critical("Failed to start server: %v", err)
	}
	log.Info("HTTP Listener: %s Closed", listenAddr)
	<-graceful.GetManager().Done()
	log.Info("PID: %d Gitea Pages Finished", os.Getpid())
	log.Close()
	return nil
}

type pagesHandler struct {
	remote string
	branch string
	token  string
}

func (h *pagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info("%s %s", r.Method, r.URL.String())
	match := pageURL.FindAllStringSubmatch(filepath.Clean(r.URL.String()), 1)
	//log.Info("%#v", match)

	if len(match) != 1 {
		err := pageErrorHandler(w)
		if err != nil {
			log.Error("%#v", err)
		}
		return
	}
	//Manage default index.html
	if match[0][3] == "" {
		match[0][3] = "index.html"
	}

	//Retrieve data from raw api
	url := fmt.Sprintf("%s/%s/%s/raw/branch/%s/%s", h.remote, match[0][1], match[0][2], h.branch, match[0][3])
	log.Debug("Retrieving %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error("%#v", err)
		err := pageErrorHandler(w)
		if err != nil {
			log.Error("%#v", err)
		}
		return
	}
	if h.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", h.token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("%#v", err)
		err := pageErrorHandler(w)
		if err != nil {
			log.Error("%#v", err)
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := pageErrorHandler(w)
		if err != nil {
			log.Error("%#v", err)
		}
		return
	}

	//Response
	if mimeType := mime.TypeByExtension(filepath.Ext(match[0][3])); mimeType != "" {
		w.Header().Set("Content-Type", mimeType)
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Error("%#v", err)
	}
}

func pageErrorHandler(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotFound)
	_, err := fmt.Fprint(w, "404 File Not Found")
	return err
}
