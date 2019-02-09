package main

/*
Checkout a PR and load the tests data into sqlite database
*/

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"time"

	"code.gitea.io/gitea/modules/markup/external"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/routes"
	"github.com/Unknwon/com"
	"github.com/facebookgo/grace/gracehttp"
	context2 "github.com/gorilla/context"
	"gopkg.in/testfixtures.v2"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
)

func runPR() {
	log.Printf("[PR] Starting gitea ...\n")
	setting.NewContext()
	models.MainTestSetup(".")
	setting.AppURL = "http://localhost:8080/"
	setting.HTTPPort = "8080"
	setting.SSH.Domain = "localhost"
	setting.SSH.Port = 3000
	setting.InstallLock = true
	setting.SecretKey = "9pCviYTWSb"
	setting.InternalToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYmYiOjE0OTI3OTU5ODN9.OQkH5UmzID2XBdwQ9TAI6Jj2t1X-wElVTjbE7aoN4I8"
	curUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	setting.RunUser = curUser.Username
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("[PR] Loading fixtures data ...\n")
	var helper testfixtures.Helper
	helper = &testfixtures.SQLite{}
	err = models.InitFixtures(
		helper,
		path.Join(curDir, "models/fixtures/"),
	)
	if err != nil {
		fmt.Printf("Error initializing test database: %v\n", err)
		os.Exit(1)
	}
	models.LoadFixtures()
	os.RemoveAll(setting.RepoRootPath)
	os.RemoveAll(models.LocalCopyPath())
	os.RemoveAll(models.LocalWikiPath())
	com.CopyDir(path.Join(curDir, "integrations/gitea-repositories-meta"), setting.RepoRootPath)

	log.Printf("[PR] Setting up router\n")
	setting.CheckLFSVersion()
	models.LoadConfigs()
	//routers.GlobalInit()
	routers.NewServices()
	external.RegisterParsers()
	m := routes.NewMacaron()
	routes.RegisterRoutes(m)

	log.Printf("[PR] Ready for testing !\n")
	log.Printf("[PR] Login with user1, user2, user3, ... with pass: passsword\n")
	/*
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
	*/
	gracehttp.Serve(&http.Server{
		Addr:    ":8080",
		Handler: context2.ClearHandler(m),
	})
	//time.Sleep(5 * time.Minute) //TODO wait for input

	log.Printf("[PR] Cleaning up ...\n")
	if err = os.RemoveAll(setting.Indexer.IssuePath); err != nil {
		fmt.Printf("os.RemoveAll: %v\n", err)
		os.Exit(1)
	}
	if err = os.RemoveAll(setting.Indexer.RepoPath); err != nil {
		fmt.Printf("Unable to remove repo indexer: %v\n", err)
		os.Exit(1)
	}
	models.MainTestCleanup(0)
}

func main() {
	var runPRFlag = flag.Bool("run", false, "Run the PR code")
	flag.Parse()
	if *runPRFlag {
		runPR()
		return
	}

	//Otherwise checkout PR
	if len(os.Args) != 2 {
		log.Fatal("Need only one arg: the PR number")
	}
	pr := os.Args[1]

	branch := fmt.Sprintf("pr-%s-%d", pr, time.Now().Unix())
	log.Printf("Checkout PR #%s in %s\n", pr, branch)
	runCmd("git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", pr, branch))
	err := git.Checkout(".", git.CheckoutOptions{
		Branch: branch,
	})
	if err != nil {
		log.Fatalf("Failed to checkout pr-%s : %v", pr, err)
	}

	//Start with integration test
	runCmd("go", "run", "-tags='sqlite sqlite_unlock_notify'", "contrib/pr/checkout.go", "-run")
}
func runCmd(cmd ...string) {
	log.Printf("Executing : %s ...\n", cmd)
	c := exec.Command(cmd[0], cmd[1:]...)
	if err := c.Run(); err != nil {
		log.Panicln(err)
	}
}
