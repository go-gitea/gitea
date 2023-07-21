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
	Before: PrepareConsoleLoggerLevel(log.INFO),
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

func serveInstall(ctx *cli.Context) error {
	log.Info("Gitea version: %s%s", setting.AppVer, setting.AppBuiltWith)
	log.Info("App path: %s", setting.AppPath)
	log.Info("Work path: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Config file: %s", setting.CustomConf)
	log.Info("Prepare to run install page")

	routers.InitWebInstallPage(graceful.GetManager().HammerContext())

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
	c := install.Routes()
	err := listen(c, false)
	if err != nil {
		log.Critical("Unable to open listener for installer. Is Gitea already running?")
		graceful.GetManager().DoGracefulShutdown()
	}
	select {
	case <-graceful.GetManager().IsShutdown():
		<-graceful.GetManager().Done()
		log.Info("PID: %d Gitea Web Finished", os.Getpid())
		log.GetManager().Close()
		return err
	default:
	}
	return nil
}

func serveInstalled(ctx *cli.Context) error {
	setting.InitCfgProvider(setting.CustomConf)
	setting.LoadCommonSettings()
	setting.MustInstalled()

	log.Info("Gitea version: %s%s", setting.AppVer, setting.AppBuiltWith)
	log.Info("App path: %s", setting.AppPath)
	log.Info("Work path: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Config file: %s", setting.CustomConf)
	log.Info("Run mode: %s", setting.RunMode)
	log.Info("Prepare to run web server")

	if setting.AppWorkPathMismatch {
		log.Error("WORK_PATH from config %q doesn't match other paths from environment variables or command arguments. "+
			"Only WORK_PATH in config should be set and used. Please remove the other outdated work paths from environment variables and command arguments", setting.CustomConf)
	}

	rootCfg := setting.CfgProvider
	if rootCfg.Section("").Key("WORK_PATH").String() == "" {
		saveCfg, err := rootCfg.PrepareSaving()
		if err != nil {
			log.Error("Unable to prepare saving WORK_PATH=%s to config %q: %v\nYou must set it manually, otherwise there might be bugs when accessing the git repositories.", setting.AppWorkPath, setting.CustomConf, err)
		} else {
			rootCfg.Section("").Key("WORK_PATH").SetValue(setting.AppWorkPath)
			saveCfg.Section("").Key("WORK_PATH").SetValue(setting.AppWorkPath)
			if err = saveCfg.Save(); err != nil {
				log.Error("Unable to update WORK_PATH=%s to config %q: %v\nYou must set it manually, otherwise there might be bugs when accessing the git repositories.", setting.AppWorkPath, setting.CustomConf, err)
			}
		}
	}

	routers.InitWebInstalled(graceful.GetManager().HammerContext())

	// We check that AppDataPath exists here (it should have been created during installation)
	// We can't check it in `InitWebInstalled`, because some integration tests
	// use cmd -> InitWebInstalled, but the AppDataPath doesn't exist during those tests.
	if _, err := os.Stat(setting.AppDataPath); err != nil {
		log.Fatal("Can not find APP_DATA_PATH %q", setting.AppDataPath)
	}

	// Override the provided port number within the configuration
	if ctx.IsSet("port") {
		if err := setPort(ctx.String("port")); err != nil {
			return err
		}
	}

	// Set up Chi routes
	c := routers.NormalRoutes()
	err := listen(c, true)
	<-graceful.GetManager().Done()
	log.Info("PID: %d Gitea Web Finished", os.Getpid())
	log.GetManager().Close()
	return err
}

func servePprof() {
	http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())
	_, _, finished := process.GetManager().AddTypedContext(context.Background(), "Web: PProf Server", process.SystemProcessType, true)
	// The pprof server is for debug purpose only, it shouldn't be exposed on public network. At the moment it's not worth to introduce a configurable option for it.
	log.Info("Starting pprof server on localhost:6060")
	log.Info("Stopped pprof server: %v", http.ListenAndServe("localhost:6060", nil))
	finished()
}

func runWeb(ctx *cli.Context) error {
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

	if !setting.InstallLock {
		if err := serveInstall(ctx); err != nil {
			return err
		}
	} else {
		NoInstallListener()
	}

	if setting.EnablePprof {
		go servePprof()
	}

	return serveInstalled(ctx)
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
		rootCfg := setting.CfgProvider
		saveCfg, err := rootCfg.PrepareSaving()
		if err != nil {
			return fmt.Errorf("failed to save config file: %v", err)
		}
		rootCfg.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)
		saveCfg.Section("server").Key("LOCAL_ROOT_URL").SetValue(defaultLocalURL)
		if err = saveCfg.Save(); err != nil {
			return fmt.Errorf("failed to save config file: %v", err)
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
