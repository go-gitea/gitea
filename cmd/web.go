// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Used for debugging if enabled and a web server is running
	"os"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"

	context2 "github.com/gorilla/context"
	"github.com/unknwon/com"
	"github.com/urfave/cli"
	"golang.org/x/crypto/acme/autocert"
	ini "gopkg.in/ini.v1"
)

// CmdWeb represents the available web sub-command.
var CmdWeb = cli.Command{
	Name:  "web",
	Usage: "Start Gitea web server",
	Description: `Gitea web server is the only thing you need to run,
and it takes care of all the other things for you`,
	Action: runWeb,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "port, p",
			Value: "3000",
			Usage: "Temporary port number to prevent conflict",
		},
		cli.StringFlag{
			Name:  "pid, P",
			Value: "/var/run/gitea.pid",
			Usage: "Custom pid file path",
		},
	},
}

func runHTTPRedirector() {
	source := fmt.Sprintf("%s:%s", setting.HTTPAddr, setting.PortToRedirect)
	dest := strings.TrimSuffix(setting.AppURL, "/")
	log.Info("Redirecting: %s to %s", source, dest)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := dest + r.URL.Path
		if len(r.URL.RawQuery) > 0 {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusTemporaryRedirect)
	})

	var err = runHTTP("tcp", source, context2.ClearHandler(handler))

	if err != nil {
		log.Fatal("Failed to start port redirection: %v", err)
	}
}

func runLetsEncrypt(listenAddr, domain, directory, email string, m http.Handler) error {
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache(directory),
		Email:      email,
	}
	go func() {
		log.Info("Running Let's Encrypt handler on %s", setting.HTTPAddr+":"+setting.PortToRedirect)
		// all traffic coming into HTTP will be redirect to HTTPS automatically (LE HTTP-01 validation happens here)
		var err = runHTTP("tcp", setting.HTTPAddr+":"+setting.PortToRedirect, certManager.HTTPHandler(http.HandlerFunc(runLetsEncryptFallbackHandler)))
		if err != nil {
			log.Fatal("Failed to start the Let's Encrypt handler on port %s: %v", setting.PortToRedirect, err)
		}
	}()
	return runHTTPSWithTLSConfig("tcp", listenAddr, certManager.TLSConfig(), context2.ClearHandler(m))
}

func runLetsEncryptFallbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "Use HTTPS", http.StatusBadRequest)
		return
	}
	// Remove the trailing slash at the end of setting.AppURL, the request
	// URI always contains a leading slash, which would result in a double
	// slash
	target := strings.TrimRight(setting.AppURL, "/") + r.URL.RequestURI()
	http.Redirect(w, r, target, http.StatusFound)
}

func runWeb(ctx *cli.Context) error {
	managerCtx, cancel := context.WithCancel(context.Background())
	graceful.InitManager(managerCtx)
	defer cancel()

	if os.Getppid() > 1 && len(os.Getenv("LISTEN_FDS")) > 0 {
		log.Info("Restarting Gitea on PID: %d from parent PID: %d", os.Getpid(), os.Getppid())
	} else {
		log.Info("Starting Gitea on PID: %d", os.Getpid())
	}

	// Set pid file setting
	if ctx.IsSet("pid") {
		setting.CustomPID = ctx.String("pid")
	}

	// Perform global initialization
	routers.GlobalInit(graceful.GetManager().HammerContext())

	// Set up Macaron
	m := routes.NewMacaron()
	routes.RegisterRoutes(m)

	// Flag for port number in case first time run conflict.
	if ctx.IsSet("port") {
		setting.AppURL = strings.Replace(setting.AppURL, setting.HTTPPort, ctx.String("port"), 1)
		setting.HTTPPort = ctx.String("port")

		switch setting.Protocol {
		case setting.UnixSocket:
		case setting.FCGI:
		case setting.FCGIUnix:
		default:
			// Save LOCAL_ROOT_URL if port changed
			cfg := ini.Empty()
			if com.IsFile(setting.CustomConf) {
				// Keeps custom settings if there is already something.
				if err := cfg.Append(setting.CustomConf); err != nil {
					return fmt.Errorf("Failed to load custom conf '%s': %v", setting.CustomConf, err)
				}
			}

			defaultLocalURL := string(setting.Protocol) + "://"
			if setting.HTTPAddr == "0.0.0.0" {
				defaultLocalURL += "localhost"
			} else {
				defaultLocalURL += setting.HTTPAddr
			}
			defaultLocalURL += ":" + setting.HTTPPort + "/"

			cfg.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)

			if err := cfg.SaveTo(setting.CustomConf); err != nil {
				return fmt.Errorf("Error saving generated JWT Secret to custom config: %v", err)
			}
		}
	}

	listenAddr := setting.HTTPAddr
	if setting.Protocol != setting.UnixSocket && setting.Protocol != setting.FCGIUnix {
		listenAddr += ":" + setting.HTTPPort
	}
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubURL)

	if setting.LFS.StartServer {
		log.Info("LFS server enabled")
	}

	if setting.EnablePprof {
		go func() {
			log.Info("Starting pprof server on localhost:6060")
			log.Info("%v", http.ListenAndServe("localhost:6060", nil))
		}()
	}

	var err error
	switch setting.Protocol {
	case setting.HTTP:
		NoHTTPRedirector()
		err = runHTTP("tcp", listenAddr, context2.ClearHandler(m))
	case setting.HTTPS:
		if setting.EnableLetsEncrypt {
			err = runLetsEncrypt(listenAddr, setting.Domain, setting.LetsEncryptDirectory, setting.LetsEncryptEmail, context2.ClearHandler(m))
			break
		}
		if setting.RedirectOtherPort {
			go runHTTPRedirector()
		} else {
			NoHTTPRedirector()
		}
		err = runHTTPS("tcp", listenAddr, setting.CertFile, setting.KeyFile, context2.ClearHandler(m))
	case setting.FCGI:
		NoHTTPRedirector()
		err = runFCGI("tcp", listenAddr, context2.ClearHandler(m))
	case setting.UnixSocket:
		NoHTTPRedirector()
		err = runHTTP("unix", listenAddr, context2.ClearHandler(m))
	case setting.FCGIUnix:
		NoHTTPRedirector()
		err = runFCGI("unix", listenAddr, context2.ClearHandler(m))
	default:
		log.Fatal("Invalid protocol: %s", setting.Protocol)
	}

	if err != nil {
		log.Critical("Failed to start server: %v", err)
	}
	log.Info("HTTP Listener: %s Closed", listenAddr)
	<-graceful.GetManager().Done()
	log.Info("PID: %d Gitea Web Finished", os.Getpid())
	log.Close()
	return nil
}
