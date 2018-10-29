---
date: "2016-12-01T16:00:00+02:00"
title: "Webhooks"
slug: "webhooks"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Webhooks"
    weight: 30
    identifier: "webhooks"
---

# Webhooks

Gitea supports web hooks for repository events, this can be found in the settings
page(`/:username/:reponame/settings/hooks`). All event pushes are POST requests.
The two methods currently supported are Gitea and Slack.

### Event information

The following is an example of event information that will be sent by Gitea to
a Payload URL:


```
X-Github-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Github-Event: push
X-Gogs-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gogs-Event: push
X-Gitea-Delivery: f6266f16-1bf3-46a5-9ea4-602e06ead473
X-Gitea-Event: push
```

```json
{
  "secret": "3gEsCfjlV2ugRwgpU#w1*WaW*wa4NXgGmpCfkbG3",
  "ref": "refs/heads/develop",
  "before": "28e1879d029cb852e4844d9c718537df08844e03",
  "after": "bffeb74224043ba2feb48d137756c8a9331c449a",
  "compare_url": "http://localhost:3000/gitea/webhooks/compare/28e1879d029cb852e4844d9c718537df08844e03...bffeb74224043ba2feb48d137756c8a9331c449a",
  "commits": [
    {
      "id": "bffeb74224043ba2feb48d137756c8a9331c449a",
      "message": "Webhooks Yay!",
      "url": "http://localhost:3000/gitea/webhooks/commit/bffeb74224043ba2feb48d137756c8a9331c449a",
      "author": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "committer": {
        "name": "Gitea",
        "email": "someone@gitea.io",
        "username": "gitea"
      },
      "timestamp": "2017-03-13T13:52:11-04:00"
    }
  ],
  "repository": {
    "id": 140,
    "owner": {
      "id": 1,
      "login": "gitea",
      "full_name": "Gitea",
      "email": "someone@gitea.io",
      "avatar_url": "https://localhost:3000/avatars/1",
      "username": "gitea"
    },
    "name": "webhooks",
    "full_name": "gitea/webhooks",
    "description": "",
    "private": false,
    "fork": false,
    "html_url": "http://localhost:3000/gitea/webhooks",
    "ssh_url": "ssh://gitea@localhost:2222/gitea/webhooks.git",
    "clone_url": "http://localhost:3000/gitea/webhooks.git",
    "website": "",
    "stars_count": 0,
    "forks_count": 1,
    "watchers_count": 1,
    "open_issues_count": 7,
    "default_branch": "master",
    "created_at": "2017-02-26T04:29:06-05:00",
    "updated_at": "2017-03-13T13:51:58-04:00"
  },
  "pusher": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  },
  "sender": {
    "id": 1,
    "login": "gitea",
    "full_name": "Gitea",
    "email": "someone@gitea.io",
    "avatar_url": "https://localhost:3000/avatars/1",
    "username": "gitea"
  }
}
```

### Example webhook receiver

Here is a simple webhook receiver, written in go.

```go
// Based on: https://github.com/soupdiver/go-gitlab-webhook
// Gitea SDK: https://godoc.org/code.gitea.io/sdk/gitea
// Gitea webhooks: https://docs.gitea.io/en-us/webhooks

package main

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"

	api "code.gitea.io/sdk/gitea"
)

//ConfigRepository represents a repository from the config file
type ConfigRepository struct {
	Secret   string
	Name     string
	Commands []string
}

//Config represents the config file
type Config struct {
	Logfile      string
	Address      string
	Port         int64
	Repositories []ConfigRepository
}

func panicIf(err error, what ...string) {
	if err != nil {
		if len(what) == 0 {
			panic(err)
		}

		panic(errors.New(err.Error() + (" " + what[0])))
	}
}

var config Config
var configFile string

func main() {
	args := os.Args

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)

	go func() {
		<-sigc
		config = loadConfig(configFile)
		log.Println("config reloaded")
	}()

	//if we have a "real" argument we take this as conf path to the config file
	if len(args) > 1 {
		configFile = args[1]
	} else {
		configFile = "config.json"
	}

	//load config
	config = loadConfig(configFile)

	//open log file
	writer, err := os.OpenFile(config.Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	panicIf(err)

	//close logfile on exit
	defer func() {
		writer.Close()
	}()

	//setting logging output
	log.SetOutput(writer)

	//setting handler
	http.HandleFunc("/", hookHandler)

	address := config.Address + ":" + strconv.FormatInt(config.Port, 10)

	log.Println("Listening on " + address)

	//starting server
	err = http.ListenAndServe(address, nil)
	if err != nil {
		log.Println(err)
	}
}

func loadConfig(configFile string) Config {
	var file, err = os.Open(configFile)
	panicIf(err)

	// close file on exit and check for its returned error
	defer func() {
		panicIf(file.Close())
	}()

	buffer := make([]byte, 1024)

	count, err := file.Read(buffer)
	panicIf(err)

	err = json.Unmarshal(buffer[:count], &config)
	panicIf(err)

	return config
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()

	//get the hook event from the headers
	event := r.Header.Get("X-Gogs-Event")
	if len(event) == 0 {
		event = r.Header.Get("X-Gitea-Event")
	}

	//only push events are currently supported
	if event != "push" {
		log.Printf("received unknown event \"%s\"\n", event)
		return
	}

	//read request body
	var data, err = ioutil.ReadAll(r.Body)
	panicIf(err, "while reading request body")

	//unmarshal request body
	var hook api.PushPayload
	err = json.Unmarshal(data, &hook)
	panicIf(err, fmt.Sprintf("while unmarshaling request base64(%s)", b64.StdEncoding.EncodeToString(data)))

	log.Printf("received webhook on %s", hook.Repo.FullName)

	//find matching config for repository name
	for _, repo := range config.Repositories {

		if repo.Name == hook.Repo.FullName || repo.Name == hook.Repo.HTMLURL {

			//check if the secret in the configuration matches the request
			if repo.Secret != hook.Secret {
				log.Printf("secret mismatch for repo %s\n", repo.Name)
				continue
			}

			//execute commands for repository
			for _, cmd := range repo.Commands {
				var command = exec.Command(cmd)
				out, err := command.Output()
				if err != nil {
					log.Println(err)
				} else {
					log.Println("Executed: " + cmd)
					log.Println("Output: " + string(out))
				}
			}
		}
	}
}
```

It reads `config.json` that allows you to configure what happens for various repositories:

```json
{
  "logfile": "go-gitea-webhook.log",
  "address": "0.0.0.0",
  "port": 3344,
  "repositories": [
    {
      "secret": "verysecret123",
      "name": "user/repo",
      "commands": [
        "/home/user/update_repo.sh"
      ]
    }
  ]
}
```
