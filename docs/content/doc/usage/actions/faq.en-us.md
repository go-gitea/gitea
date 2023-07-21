---
date: "2023-04-27T15:00:00+08:00"
title: "Frequently Asked Questions of Gitea Actions"
slug: "faq"
weight: 100
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "FAQ"
    weight: 100
    identifier: "actions-faq"
---

# Frequently Asked Questions of Gitea Actions

This page contains some common questions and answers about Gitea Actions.

**Table of Contents**

{{< toc >}}

## Why is Actions not enabled by default?

We know it's annoying to enable Actions for the whole instance and each repository one by one, but not everyone likes or needs this feature.
We believe that more work needs to be done to improve Gitea Actions before it deserves any further special treatment.

## Is it possible to enable Actions for new repositories by default for my own instance?

Yes, when you enable Actions for the instance, you can choose to enable the `actions` unit for all new repositories by default.

```ini
[repository]
DEFAULT_REPO_UNITS = ...,repo.actions
```

## Should we use `${{ github.xyz }}` or `${{ gitea.xyz }}`  in workflow files?

You can use `github.xyz` and Gitea will work fine.
As mentioned, Gitea Actions is designed to be compatible with GitHub Actions.
However, we recommend using `gitea.xyz` in case Gitea adds something that GitHub does not have to avoid different kinds of secrets in your workflow file (and because you are using this workflow on Gitea, not GitHub).
Still, this is completely optional since both options have the same effect at the moment.

## Is it possible to register runners for a specific user (not organization)?

Not yet.
It is technically possible to implement, but we need to discuss whether it is necessary.

## Where will the runner download scripts when using actions such as `actions/checkout@v3`?

You may be aware that there are tens of thousands of [marketplace actions](https://github.com/marketplace?type=actions) in GitHub.
However, when you write `uses: actions/checkout@v3`, it actually downloads the scripts from [gitea.com/actions/checkout](http://gitea.com/actions/checkout) by default (not GitHub).
This is a mirror of [github.com/actions/checkout](http://github.com/actions/checkout), but it's impossible to mirror all of them.
That's why you may encounter failures when trying to use some actions that haven't been mirrored.

The good news is that you can specify the URL prefix to use actions from anywhere.
This is an extra syntax in Gitea Actions.
For example:

- `uses: https://github.com/xxx/xxx@xxx`
- `uses: https://gitea.com/xxx/xxx@xxx`
- `uses: http://your_gitea_instance.com/xxx@xxx`

Be careful, the `https://` or `http://` prefix is necessary!

Alternatively, if you want your runners to download actions from GitHub or your own Gitea instance by default, you can configure it by setting `[actions].DEFAULT_ACTIONS_URL`.
See [Configuration Cheat Sheet](https://docs.gitea.io/en-us/config-cheat-sheet/#actions-actions).

This is one of the differences from GitHub Actions, but it should allow users much more flexibility in how they run Actions.

## How to limit the permission of the runners?

Runners have no more permissions than simply connecting to your Gitea instance.
When any runner receives a job to run, it will temporarily gain limited permission to the repository associated with the job.
If you want to give more permissions to the runner, allowing it to access more private repositories or external systems, you can pass [secrets](https://docs.gitea.io/en-us/usage/secrets/) to it.

Refined permission control to Actions is a complicated job.
In the future, we will add more options to Gitea to make it more configurable, such as allowing more write access to repositories or read access to all repositories in the same organization.

## How to avoid being hacked?

There are two types of possible attacks: unknown runner stealing the code or secrets from your repository, or malicious scripts controlling your runner.

Avoiding the former means not allowing people you don't know to register runners for your repository, organization, or instance.

The latter is a bit more complicated.
If you're using a private Gitea instance for your company, you may not need to worry about security since you trust your colleagues and can hold them accountable.

For public instances, things are a little different.
Here's how we do it on [gitea.com](http://gitea.com/):

- We only register runners for the "gitea" organization, so our runners will not execute jobs from other repositories.
- Our runners always run jobs with isolated containers. While it is possible to do this directly on the host, we choose not to for more security.
- To run actions for fork pull requests, approval is required. See [#22803](https://github.com/go-gitea/gitea/pull/22803).
- If someone registers their own runner for their repository or organization on [gitea.com](http://gitea.com/), we have no objections and will just not use it in our org. However, they should take care to ensure that the runner is not used by other users they do not know.

## Which operating systems are supported by act runner?

It works well on Linux, macOS, and Windows.
While other operating systems are theoretically supported, they require further testing.

One thing to note is that if you choose to run jobs directly on the host instead of in job containers, the environmental differences between operating systems may cause unexpected failures.

For example, bash is not available on Windows in most cases, while act tries to use bash to run scripts by default.
Therefore, you need to specify `powershell` as the default shell in your workflow file, see [defaults.run](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#defaultsrun).

```yaml
defaults:
  run:
    shell: powershell
```

## Why choose GitHub Actions? Why not something compatible with GitLab CI/CD?

[@lunny](https://gitea.com/lunny) has explained this in the [issue to implement actions](https://github.com/go-gitea/gitea/issues/13539).
Furthermore, Actions is not only a CI/CD system but also an automation tool.

There have also been numerous [marketplace actions](https://github.com/marketplace?type=actions) implemented in the open-source world.
It is exciting to be able to reuse them.

## What if it runs on multiple labels, such as `runs-on: [label_a, label_b]`?

This is valid syntax.
It means that it should run on runners that have both the `label_a` **and** `label_b` labels, see [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idruns-on).
Unfortunately, act runner does not work this way.
As mentioned, we map labels to environments:

- `ubuntu` → `ubuntu:22.04`
- `centos` → `centos:8`

But we need to map label groups to environments instead, like so:

- `[ubuntu]` → `ubuntu:22.04`
- `[with-gpu]` → `linux:with-gpu`
- `[ubuntu, with-gpu]` → `ubuntu:22.04_with-gpu`

We also need to re-design how tasks are assigned to runners.
A runner with `ubuntu`, `centos`, or `with-gpu` does not necessarily indicate that it can accept jobs with `[centos, with-gpu]`.
Therefore, the runner should inform the Gitea instance that it can only accept jobs with `[ubuntu]`, `[centos]`, `[with-gpu]`, and `[ubuntu, with-gpu]`.
This is not a technical problem, it was just overlooked in the early design.
See [runtime.go#L65](https://gitea.com/gitea/act_runner/src/commit/90b8cc6a7a48f45cc28b5ef9660ebf4061fcb336/runtime/runtime.go#L65).

Currently, the act runner attempts to match everyone in the labels and uses the first match it finds.

## What is the difference between agent labels and custom labels for a runner?

![labels](/images/usage/actions/labels.png)

Agent labels are reported to the Gitea instance by the runner during registration.
Custom labels, on the other hand, are added manually by a Gitea administrator or owners of the organization or repository (depending on the level of the runner).

However, the design here needs improvement, as it currently has some rough edges.
You can add a custom label such as `centos` to a registered runner, which means the runner will receive jobs with `runs-on: centos`.
However, the runner may not know which environment to use for this label, resulting in it using a default image or leading to a logical dead end.
This default may not match user expectations.
See [runtime.go#L71](https://gitea.com/gitea/act_runner/src/commit/90b8cc6a7a48f45cc28b5ef9660ebf4061fcb336/runtime/runtime.go#L71).

In the meantime, we suggest that you re-register your runner if you want to change its labels.

## Will there be more implementations for Gitea Actions runner?

Although we would like to provide more options, our limited manpower means that act runner will be the only officially supported runner.
However, both Gitea and act runner are completely open source, so anyone can create a new/better implementation.
We support your choice, no matter how you decide.
In case you fork act runner to create your own version: Please contribute the changes back if you can and if you think your changes will help others as well.

## What workflow trigger events does Gitea support?

All events listed in this table are supported events and are compatible with GitHub.
For events supported only by GitHub, see GitHub's [documentation](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows).

| trigger event               | activity types                                                                                                           |
|-----------------------------|--------------------------------------------------------------------------------------------------------------------------|
| create                      | not applicable                                                                                                           |
| delete                      | not applicable                                                                                                           |
| fork                        | not applicable                                                                                                           |
| gollum                      | not applicable                                                                                                           |
| push                        | not applicable                                                                                                           |
| issues                      | `opened`, `edited`, `closed`, `reopened`, `assigned`, `unassigned`, `milestoned`, `demilestoned`, `labeled`, `unlabeled` |
| issue_comment               | `created`, `edited`, `deleted`                                                                                           |
| pull_request                | `opened`, `edited`, `closed`, `reopened`, `assigned`, `unassigned`, `synchronize`, `labeled`, `unlabeled`                |
| pull_request_review         | `submitted`, `edited`                                                                                                    |
| pull_request_review_comment | `created`, `edited`                                                                                                      |
| release                     | `published`, `edited`                                                                                                    |
| registry_package            | `published`                                                                                                              |
