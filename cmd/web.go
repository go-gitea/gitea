// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "net/http/pprof" // Used for debugging if enabled and a web server is running

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/install"

	"github.com/felixge/fgprof"
	"github.com/urfave/cli"
)

// PIDFile could be set from build tag
var PIDFile = "/run/gitea.pid"

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
			Name:  "install-port",
			Value: "3000",
			Usage: "Temporary port number to run the install page on to prevent conflict",
		},
		cli.StringFlag{
			Name:  "pid, P",
			Value: PIDFile,
			Usage: "Custom pid file path",
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "Only display Fatal logging errors until logging is set-up",
		},
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Set initial logging to TRACE level until logging is properly set-up",
		},
	},
}

func runHTTPRedirector() {
	_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), "Web: HTTP Redirector", process.SystemProcessType, true)
	defer finished()

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

	err := runHTTP("tcp", source, "HTTP Redirector", handler, setting.RedirectorUseProxyProtocol)
	if err != nil {
		log.Fatal("Failed to start port redirection: %v", err)
	}
}

func createPIDFile(pidPath string) {
	currentPid := os.Getpid()
	if err := os.MkdirAll(filepath.Dir(pidPath), os.ModePerm); err != nil {
		log.Fatal("Failed to create PID folder: %v", err)
	}

	file, err := os.Create(pidPath)
	if err != nil {
		log.Fatal("Failed to create PID file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.FormatInt(int64(currentPid), 10)); err != nil {
		log.Fatal("Failed to write PID information: %v", err)
	}
}

func runWeb(ctx *cli.Context) error {
	if ctx.Bool("verbose") {
		_ = log.DelLogger("console")
		log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "trace", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))
	} else if ctx.Bool("quiet") {
		_ = log.DelLogger("console")
		log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "fatal", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))
	}
	defer func() {
		if panicked := recover(); panicked != nil {
			log.Fatal("PANIC: %v\n%s", panicked, log.Stack(2))
		}
	}()

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
		createPIDFile(ctx.String("pid"))
	}

	// Perform pre-initialization
	needsInstall := install.PreloadSettings(graceful.GetManager().HammerContext())
	if needsInstall {
		// Flag for port number in case first time run conflict
		if ctx.IsSet("port") {
			if err := setPort(ctx.String("port")); err != nil {
				return err
			}
		}
		if ctx.IsSet("install-port") {
			if err := setPort(ctx.String("install-port")); err != nil {
				return err
			}
		}
		installCtx, cancel := context.WithCancel(graceful.GetManager().HammerContext())
		c := install.Routes(installCtx)
		err := listen(c, false)
		cancel()
		if err != nil {
			log.Critical("Unable to open listener for installer. Is Gitea already running?")
			graceful.GetManager().DoGracefulShutdown()
		}
		select {
		case <-graceful.GetManager().IsShutdown():
			<-graceful.GetManager().Done()
			log.Info("PID: %d Gitea Web Finished", os.Getpid())
			log.Close()
			return err
		default:
		}
	} else {
		NoInstallListener()
	}

	if setting.EnablePprof {
		go func() {
			http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())
			_, _, finished := process.GetManager().AddTypedContext(context.Background(), "Web: PProf Server", process.SystemProcessType, true)
			// The pprof server is for debug purpose only, it shouldn't be exposed on public network. At the moment it's not worth to introduce a configurable option for it.
			log.Info("Starting pprof server on localhost:6060")
			log.Info("Stopped pprof server: %v", http.ListenAndServe("localhost:6060", nil))
			finished()
		}()
	}

	log.Info("Global init")
	// Perform global initialization
	setting.InitProviderFromExistingFile()
	setting.LoadCommonSettings()
	routers.GlobalInitInstalled(graceful.GetManager().HammerContext())

	// We check that AppDataPath exists here (it should have been created during installation)
	// We can't check it in `GlobalInitInstalled`, because some integration tests
	// use cmd -> GlobalInitInstalled, but the AppDataPath doesn't exist during those tests.
	if _, err := os.Stat(setting.AppDataPath); err != nil {
		log.Fatal("Can not find APP_DATA_PATH '%s'", setting.AppDataPath)
	}

	// Override the provided port number within the configuration
	if ctx.IsSet("port") {
		if err := setPort(ctx.String("port")); err != nil {
			return err
		}
	}

	// Set up Chi routes
	c := routers.NormalRoutes(graceful.GetManager().HammerContext())
	err := listen(c, true)
	<-graceful.GetManager().Done()
	log.Info("PID: %d Gitea Web Finished", os.Getpid())
	log.Close()
	return err
}

func setPort(port string) error {
	setting.AppURL = strings.Replace(setting.AppURL, setting.HTTPPort, port, 1)
	setting.HTTPPort = port

	switch setting.Protocol {
	case setting.HTTPUnix:
	case setting.FCGI:
	case setting.FCGIUnix:
	default:
		defaultLocalURL := string(setting.Protocol) + "://"
		if setting.HTTPAddr == "0.0.0.0" {
			defaultLocalURL += "localhost"
		} else {
			defaultLocalURL += setting.HTTPAddr
		}
		defaultLocalURL += ":" + setting.HTTPPort + "/"

		// Save LOCAL_ROOT_URL if port changed
		setting.CfgProvider.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)
		if err := setting.CfgProvider.Save(); err != nil {
			return fmt.Errorf("Failed to save config file: %v", err)
		}
	}
	return nil
}

func listen(m http.Handler, handleRedirector bool) error {
	listenAddr := setting.HTTPAddr
	if setting.Protocol != setting.HTTPUnix && setting.Protocol != setting.FCGIUnix {
		listenAddr = net.JoinHostPort(listenAddr, setting.HTTPPort)
	}
	_, _, finished := process.GetManager().AddTypedContext(graceful.GetManager().HammerContext(), "Web: Gitea Server", process.SystemProcessType, true)
	defer finished()
	log.Info("Listen: %v://%s%s", setting.Protocol, listenAddr, setting.AppSubURL)
	// This can be useful for users, many users do wrong to their config and get strange behaviors behind a reverse-proxy.
	// A user may fix the configuration mistake when he sees this log.
	// And this is also very helpful to maintainers to provide help to users to resolve their configuration problems.
	log.Info("AppURL(ROOT_URL): %s", setting.AppURL)

	if setting.LFS.StartServer {
		log.Info("LFS server enabled")
	}

	var err error
	switch setting.Protocol {
	case setting.HTTP:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runHTTP("tcp", listenAddr, "Web", m, setting.UseProxyProtocol)
	case setting.HTTPS:
		if setting.EnableAcme {
			err = runACME(listenAddr, m)
			break
		}
		if handleRedirector {
			if setting.RedirectOtherPort {
				go runHTTPRedirector()
			} else {
				NoHTTPRedirector()
			}
		}
		err = runHTTPS("tcp", listenAddr, "Web", setting.CertFile, setting.KeyFile, m, setting.UseProxyProtocol, setting.ProxyProtocolTLSBridging)
	case setting.FCGI:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runFCGI("tcp", listenAddr, "FCGI Web", m, setting.UseProxyProtocol)
	case setting.HTTPUnix:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runHTTP("unix", listenAddr, "Web", m, setting.UseProxyProtocol)
	case setting.FCGIUnix:
		if handleRedirector {
			NoHTTPRedirector()
		}
		err = runFCGI("unix", listenAddr, "Web", m, setting.UseProxyProtocol)
	default:
		log.Fatal("Invalid protocol: %s", setting.Protocol)
	}
	if err != nil {
		log.Critical("Failed to start server: %v", err)
	}
	log.Info("HTTP Listener: %s Closed", listenAddr)
	return err
}
