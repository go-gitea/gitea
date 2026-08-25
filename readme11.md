# Gitea Setup and Pull Request — Loom Video Script

## 1. Introduction

Hello, my name is Abhishek.

In this video, I’ll demonstrate my Gitea project setup and the changes I made locally.

I’ll show how I cloned my fork, reviewed the project structure and documentation, configured the required environment, built and ran Gitea locally without Docker, verified the application, and finally pushed my changes to my fork and raised a Pull Request to the original repository.

Let’s begin.

---

## 2. Clone My Fork

I have already forked the original repository to my GitHub account.

I’ll now clone my fork to my local system.

I’ll use:

`git clone <MY-FORK-URL>`

Then I’ll enter the project directory:

`cd <PROJECT-DIRECTORY>`

I’ll verify the repository using:

`git remote -v`

Here, `origin` points to my GitHub fork.

---

## 3. Review the Project

Next, I’ll review the project structure.

I’ll use `ls` and inspect the important project files and directories.

I’ll also review the README and the project documentation to understand how Gitea should be configured, built, and run locally.

I’ll check the `go.mod` file to identify the required Go version and dependencies.

I’ll also inspect the Makefile to understand the available build commands.

---

## 4. Set Up Dependencies

Now I’ll verify my local environment.

First, I’ll check the Go version using:

`go version`

Then I’ll verify Node.js and npm if they are required by the project.

After that, I’ll download the Go dependencies using:

`go mod download`

and verify them with:

`go mod verify`

I’ll also install the required frontend dependencies according to the project documentation.

---

## 5. Build Gitea Without Docker

Now I’ll build Gitea locally.

One of the requirements of this task is to run Gitea without Docker, so I’m building and running the application directly on my local system.

I’ll use the build command specified in the repository documentation or Makefile.

After the build finishes, I’ll verify the generated Gitea executable using:

`./gitea --version`

This confirms that Gitea has been successfully built.

---

## 6. Run Gitea

Now I’ll start Gitea locally using:

`./gitea web`

As you can see, the Gitea server has started successfully.

I’ll open the application in my browser at:

`http://localhost:3000`

The Gitea interface is loading successfully, which confirms that the application is running locally.

---

## 7. Verify the Application

I’ll also verify the application from the terminal.

I can use:

`curl -I http://localhost:3000`

The successful HTTP response confirms that the application is responding correctly.

I can also verify the running process and listening port using commands such as:

`ps aux | grep '[g]itea'`

and:

`ss -lntp | grep 3000`

This confirms that Gitea is running directly on my system and not through Docker.

---

## 8. Create a Feature Branch

Now I’ll create a separate branch for my work instead of making changes directly on `main`.

I’ll use:

`git checkout -b <BRANCH-NAME>`

For example:

`git checkout -b local-gitea-setup`

I’ll then make and save the required changes.

After completing the changes, I’ll check:

`git status`

and review the differences using:

`git diff`

---

## 9. Commit the Changes

Once I’ve verified the changes, I’ll stage them:

`git add .`

Then I’ll create a descriptive commit:

`git commit -m "Set up Gitea for local development"`

I’ll verify the commit with:

`git log --oneline -5`

---

## 10. Push to My Fork

Now I’ll push my feature branch to my GitHub fork.

I’ll use:

`git push -u origin <BRANCH-NAME>`

The branch is now available on my GitHub fork.

---

## 11. Raise the Pull Request

Now I’ll open my GitHub fork in the browser.

GitHub detects that I have recently pushed a new branch and provides an option to create a Pull Request.

I’ll select **Compare & pull request**.

I’ll make sure that:

* The **base repository** is the original Gitea repository.
* The **base branch** is the required target branch, usually `main`.
* The **head repository** is my fork.
* The **compare branch** is my feature branch.

I’ll add a clear Pull Request title, for example:

**Set up Gitea for local development**

In the Pull Request description, I’ll briefly explain what I completed.

I’ll mention that I:

* Reviewed the project documentation.
* Set up the required dependencies.
* Built Gitea locally.
* Ran Gitea without Docker.
* Verified the application through localhost.
* Tested the application response.
* Pushed the changes to my fork.

Then I’ll click **Create pull request**.

---

## 12. Final Verification

The Pull Request has now been created successfully.

I’ll verify that the PR shows:

* My feature branch.
* The correct base repository.
* The correct target branch.
* My commits and changed files.
* The Pull Request description.

This completes the task.

## Conclusion

To summarize, I forked the repository, cloned my fork locally, reviewed the project, configured the required environment, built and ran Gitea without Docker, verified that it was working correctly, committed my changes to a feature branch, pushed the branch to my fork, and raised a Pull Request against the original repository.

Thank you.
