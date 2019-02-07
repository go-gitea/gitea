package main

/*
Checkout a PR and load the tests data into sqlite database
*/

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"code.gitea.io/git"
)

var envVars = []string{
	"TAGS=bindata sqlite sqlite_unlock_notify",
}

func main() {

	if len(os.Args) != 2 {
		log.Fatal("Need only one arg: the PR number")
	}
	pr := os.Args[1]

	branch := fmt.Sprintf("pr-%s-%d", pr, time.Now().Unix())
	log.Printf("Checkout PR #%s in %s\n", pr, branch)

	exec.Command("git", "fetch", "origin", fmt.Sprintf("pull/%s/head:%s", pr, branch)).Run()

	err := git.Checkout(".", git.CheckoutOptions{
		Branch: branch,
	})
	if err != nil {
		log.Fatalf("Failed to checkout pr-%s : %v", pr, err)
	}

	log.Printf("Building ...\n")
	run("make", "clean")
	run("make", "generate")
	run("make", "build")

	log.Printf("Setting up testing env ...\n")
	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	runDir, err := ioutil.TempDir("", "gitea-"+branch)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(runDir) // clean up
		if err != nil {
			log.Fatal(err)
		}
	}()

	bin := "gitea"
	if runtime.GOOS == "windows" {
		bin = "gitea.exe"
	}
	//Copy binary
	exec.Command("cp", "-a", bin, filepath.Join(runDir, bin)).Run()
	/*
		err = os.Rename("gitea", filepath.Join(runDir, "gitea"))
		if err != nil {
			log.Fatal(err)
		}
	*/
	curUser, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	//Write config file
	content := []byte(`
APP_NAME = Gitea: Git with a cup of tea
RUN_USER = ` + curUser.Username + `
RUN_MODE = prod

[security]
INTERNAL_TOKEN = eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYmYiOjE1NDk0ODIwNTN9.iHCXDtMYKSSceKL5LGrY8gcqSwVoNxQawlas799_8lM
INSTALL_LOCK   = true
SECRET_KEY     = nLGL8s8ODKfnhvebTD5VnOD41dvrpfhg8pYmIxWFhUskme8ni1wuWz7YCvAXSrp5

[database]
DB_TYPE  = sqlite3
PATH     = ./data/gitea.db

[repository]
ROOT = ./repositories

[server]
SSH_DOMAIN       = localhost
DOMAIN           = localhost
HTTP_PORT        = 3000
ROOT_URL         = http://localhost:3000/
DISABLE_SSH      = false
SSH_PORT         = 22
LFS_START_SERVER = true
LFS_CONTENT_PATH = ./data/lfs
LFS_JWT_SECRET   = QGraF0QfWLE7C_ykSyU_GsJMhShIzis3VkrR07Uw2ww
OFFLINE_MODE     = true

[mailer]
ENABLED = false

[service]
REGISTER_EMAIL_CONFIRM            = false
ENABLE_NOTIFY_MAIL                = false
DISABLE_REGISTRATION              = true
ALLOW_ONLY_EXTERNAL_REGISTRATION  = false
ENABLE_CAPTCHA                    = false
REQUIRE_SIGNIN_VIEW               = false
DEFAULT_KEEP_EMAIL_PRIVATE        = false
DEFAULT_ALLOW_CREATE_ORGANIZATION = true
DEFAULT_ENABLE_TIMETRACKING       = true
NO_REPLY_ADDRESS                  = noreply.example.org

[picture]
DISABLE_GRAVATAR        = true
ENABLE_FEDERATED_AVATAR = false

[openid]
ENABLE_OPENID_SIGNIN = true
ENABLE_OPENID_SIGNUP = false

[session]
PROVIDER = file

[log]
MODE      = file
LEVEL     = Info
ROOT_PATH = ./log	
`)
	err = os.MkdirAll(filepath.Join(runDir, "custom/conf"), 0755)
	if err != nil {
		log.Fatal(err)
	}
	confFile := filepath.Join(runDir, "custom/conf/app.ini")
	if err := ioutil.WriteFile(confFile, content, 0644); err != nil {
		log.Fatal(err)
	}

	log.Printf("Moving to env ready at '%s' ...\n", runDir)
	os.Chdir(runDir)

	log.Printf("Starting gitea ...\n")
	log.Printf("User: gitea\n")
	log.Printf("Pass: gitea\n")
	log.Printf("Mail: gitea@localhost\n")
	log.Printf("End the execution by doing $> pkill gitea\n")

	//Prepare setup
	go func() {
		time.Sleep(5 * time.Second)
		log.Printf("Starting gitea setup...\n")
		run("./"+bin, "admin", "create-user", "--admin", "--name", "gitea", "--password", "gitea", "--email", "gitea@localhost")
	}()

	run("./"+bin, "web")
	//TODO better use https://github.com/go-gitea/gitea/blob/6759237eda5b7ddfe9284c81900cc9deed1f6bf9/models/unit_tests.go#L39

	log.Printf("Moving back to gitea repo at '%s' ...\n", curDir)
	os.Chdir(curDir)
}
func run(cmd ...string) {
	log.Printf("Executing : %s ...\n", cmd)
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Env = append(os.Environ(), envVars...)
	if err := c.Run(); err != nil {
		log.Panicln(err)
	}
}
