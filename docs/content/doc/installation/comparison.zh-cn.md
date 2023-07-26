---
date: "2019-02-14T11:51:04+08:00"
title: "对比 Gitea 与其它 Git 托管工具"
slug: "comparison"
weight: 5
toc: false
draft: false
aliases:
  - /zh-cn/comparison
menu:
  sidebar:
    parent: "installation"
    name: "横向对比"
    weight: 5
    identifier: "comparison"
---

# 对比 Gitea 与其它 Git 托管工具

这里列出了 Gitea 与其它一些 Git 托管工具之间的异同，以便确认 Gitea 是否能够满足您的需求。

请注意，此列表中的某些表项可能已经过时，因为我们并没有定期检查其它产品的功能是否有所更改。你可以前往 [Github issue](https://github.com/go-gitea/gitea/issues) 来帮助我们更新过时的内容，感谢！

_表格中的符号含义:_

* _✓ - 支持_

* _⁄ - 部分支持_

* _✘ - 不支持_

* _? - 不确定_

* _⚙️ - 由第三方服务或插件支持_

#### 主要特性

| 特性                  | Gitea                                              | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket      | RhodeCode CE |
| --------------------- | -------------------------------------------------- | ---- | --------- | --------- | --------- | -------------- | ------------ |
| 开源免费              | ✓                                                  | ✓    | ✘         | ✓         | ✘         | ✘              | ✓            |
| 低资源开销 (RAM/CPU)  | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘              | ✘            |
| 支持多种数据库        | ✓                                                  | ✓    | ✘         | ⁄         | ⁄         | ✓              | ✓            |
| 支持多种操作系统      | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘              | ✓            |
| 升级简便              | ✓                                                  | ✓    | ✘         | ✓         | ✓         | ✘              | ✓            |
| 支持 Markdown         | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓              | ✓            |
| 支持 Orgmode          | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✘              | ?            |
| 支持 CSV              | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✓              | ?            |
| 支持第三方渲染工具    | ✓                                                  | ✘    | ✘         | ✘         | ✘         | ✓              | ?            |
| Git 驱动的静态 pages  | [⚙️][gitea-pages-server], [⚙️][gitea-caddy-plugin]   | ✘    | ✓         | ✓         | ✓         | ✘              | ✘            |
| Git 驱动的集成化 wiki | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓ (cloud only) | ✘            |
| 部署令牌              | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓              | ✓            |
| 仓库写权限令牌        | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓              | ✓            |
| 内置容器 Registry     | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✘              | ✘            |
| 外部 Git 镜像         | ✓                                                  | ✓    | ✘         | ✘         | ✓         | ✓              | ✓            |
| WebAuthn (2FA)        | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓              | ?            |
| 内置 CI/CD            | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✘              | ✘            |
| 子组织：组织内的组织  | [✘](https://github.com/go-gitea/gitea/issues/1872) | ✘    | ✘         | ✓         | ✓         | ✘              | ✓            |

#### 代码管理

| 特性                                     | Gitea                                            | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ---------------------------------------- | ------------------------------------------------ | ---- | --------- | --------- | --------- | --------- | ------------ |
| 仓库主题描述                             | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 仓库内代码搜索                           | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 全局代码搜索                             | ✓                                                | ✘    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Git LFS 2.0                              | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 组织里程碑                               | ✘                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| 细粒度用户角色 (例如 Code, Issues, Wiki) | ✓                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| 提交人的身份验证                         | ⁄                                                | ✘    | ?         | ✓         | ✓         | ✓         | ✘            |
| GPG 签名的提交                           | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| SSH 签名的提交                           | ✓                                                | ✘    | ✘         | ✘         | ✘         | ?         | ?            |
| 拒绝未用通过验证的提交                   | [✓](https://github.com/go-gitea/gitea/pull/9708) | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 仓库活跃度页面                           | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 分支管理                                 | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 建立新分支                               | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 在线代码编辑                             | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 提交的统计图表                           | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 模板仓库                                 | [✓](https://github.com/go-gitea/gitea/pull/8768) | ✘    | ✓         | ✘         | ✓         | ✓         | ✘            |

#### 工单管理

| 特性                | Gitea                                              | Gogs                                          | GitHub EE | GitLab CE                                                               | GitLab EE | BitBucket      | RhodeCode CE |
| ------------------- | -------------------------------------------------- | --------------------------------------------- | --------- | ----------------------------------------------------------------------- | --------- | -------------- | ------------ |
| 工单跟踪            | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✓ (cloud only) | ✘            |
| 工单模板            | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 标签                | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 时间跟踪            | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 支持多个负责人      | ✓                                                  | ✘                                             | ✓         | ✘                                                                       | ✓         | ✘              | ✘            |
| 关联的工单          | ✘                                                  | ✘                                             | ⁄         | [✓](https://docs.gitlab.com/ce/user/project/issues/related_issues.html) | ✓         | ✘              | ✘            |
| 私密工单            | [✘](https://github.com/go-gitea/gitea/issues/3217) | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| 评论反馈            | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 锁定讨论            | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 工单批处理          | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| 工单看板            | [✓](https://github.com/go-gitea/gitea/pull/8346)   | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| 从工单创建分支      | ✘                                                  | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| 工单搜索            | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓              | ✘            |
| 工单全局搜索        | [✘](https://github.com/go-gitea/gitea/issues/2434) | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓              | ✘            |
| 工单依赖关系        | ✓                                                  | ✘                                             | ✘         | ✘                                                                       | ✘         | ✘              | ✘            |
| 通过 Email 创建工单 | [✘](https://github.com/go-gitea/gitea/issues/6226) | [✘](https://github.com/gogs/gogs/issues/2602) | ✘         | ✓                                                                       | ✓         | ✓              | ✘            |
| 服务台              | [✘](https://github.com/go-gitea/gitea/issues/6219) | ✘                                             | ✘         | [✓](https://gitlab.com/groups/gitlab-org/-/epics/3103)                  | ✓         | ✘              | ✘            |

#### Pull/Merge requests

| 特性                                 | Gitea                                              | Gogs | GitHub EE | GitLab CE                                                                         | GitLab EE | BitBucket                                                                | RhodeCode CE |
| ------------------------------------ | -------------------------------------------------- | ---- | --------- | --------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------------ | ------------ |
| Pull/Merge requests                  | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| Squash merging                       | ✓                                                  | ✘    | ✓         | [✓](https://docs.gitlab.com/ce/user/project/merge_requests/squash_and_merge.html) | ✓         | ✓                                                                        | ✓            |
| Rebase merging                       | ✓                                                  | ✓    | ✓         | ✘                                                                                 | ⁄         | ✘                                                                        | ✓            |
| 评论 Pull/Merge request 中的某行代码 | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| 指定 Pull/Merge request 的审核人     | ✓                                                  | ✘    | ⁄         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| 解决 Merge 冲突                      | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| 限制某些用户的 push 和 merge 权限    | ✓                                                  | ✘    | ✓         | ⁄                                                                                 | ✓         | ✓                                                                        | ✓            |
| 回退某些 commits 或 merge request    | [✓](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| Pull/Merge requests 模板             | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| 查看 Cherry-picking 的更改           | [✓](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✘         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| 下载 Patch                           | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | [/](https://jira.atlassian.com/plugins/servlet/mobile#issue/BCLOUD-8323) | ✘            |

#### 第三方集成

| 特性                       | Gitea                                              | Gogs                                          | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| -------------------------- | -------------------------------------------------- | --------------------------------------------- | --------- | --------- | --------- | --------- | ------------ |
| 支持 Webhook               | ✓                                                  | ✓                                             | ✓         | ✓         | ✓         | ✓         | ✓            |
| 自定义 Git 钩子            | ✓                                                  | ✓                                             | ✓         | ✓         | ✓         | ✓         | ✓            |
| 集成 AD / LDAP             | ✓                                                  | ✓                                             | ✓         | ✓         | ✓         | ✓         | ✓            |
| 支持多个 LDAP / AD 服务    | ✓                                                  | ✓                                             | ✘         | ✘         | ✓         | ✓         | ✓            |
| LDAP 用户同步              | ✓                                                  | ✘                                             | ✓         | ✓         | ✓         | ✓         | ✓            |
| SAML 2.0 service provider  | [✘](https://github.com/go-gitea/gitea/issues/5512) | [✘](https://github.com/gogs/gogs/issues/1221) | ✓         | ✓         | ✓         | ✓         | ✘            |
| 支持 OpenId 连接           | ✓                                                  | ✘                                             | ✓         | ✓         | ✓         | ?         | ✘            |
| 集成 OAuth 2.0（外部授权） | ✓                                                  | ✘                                             | ⁄         | ✓         | ✓         | ?         | ✓            |
| 作为 OAuth 2.0 provider    | [✓](https://github.com/go-gitea/gitea/pull/5378)   | ✘                                             | ✓         | ✓         | ✓         | ✓         | ✘            |
| 二次验证 (2FA)             | ✓                                                  | ✓                                             | ✓         | ✓         | ✓         | ✓         | ✘            |
| 集成 Mattermost/Slack      | ✓                                                  | ✓                                             | ⁄         | ✓         | ✓         | ⁄         | ✓            |
| 集成 Discord               | ✓                                                  | ✓                                             | ✓         | ✓         | ✓         | ✘         | ✘            |
| 集成 Microsoft Teams       | ✓                                                  | ✘                                             | ✓         | ✓         | ✓         | ✓         | ✘            |
| 显示外部 CI/CD 的状态      | ✓                                                  | ✘                                             | ✓         | ✓         | ✓         | ✓         | ✓            |

[gitea-caddy-plugin]: https://github.com/42wim/caddy-gitea
[gitea-pages-server]: https://codeberg.org/Codeberg/pages-server
