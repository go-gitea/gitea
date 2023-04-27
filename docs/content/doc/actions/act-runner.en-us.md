---
date: "2023-04-27T15:00:00+08:00"
title: "Act Runner"
slug: "usage/actions/overview"
weight: 20
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "Act Runner"
    weight: 20
    identifier: "actions-runner"
---

#### Is it possible to register runners for a specific organization or a repository?

Yes, it depends on where you obtain the registration token.

- `/admin/runners`: This is for instance-level runners, which will run jobs for all repositories in the instance.
- `/org/<org>/settings/runners`: This is for organization-level runners, which will run jobs for all repositories in the organization.
- `/<owner>/<repo>/settings/runners`: This is for repository-level runners, which will run jobs for the repository they belong to.

If you cannot see the settings page, please make sure that you have the right permissions and that Actions have been enabled.

Please note that the repository may still use instance-level or organization-level runners even if it has its own repository-level runners.
We may provide options to control this in the future.


#### What are the labels for runners used for?

You may have noticed that every runner has labels such as `ubuntu-latest`, `ubuntu-22.04`, `ubuntu-20.04`, and `ubuntu-18.04`.
These labels are used for job matching.

For example, `runs-on: ubuntu-latest` in a workflow file means that the job will be run on a runner with the `ubuntu-latest` label.
You can also add custom labels to a runner when registering it, which we will discuss later.
