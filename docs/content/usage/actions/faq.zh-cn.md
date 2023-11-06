---
date: "2023-05-24T15:00:00+08:00"
title: "Gitea Actions常见问题解答"
slug: "faq"
sidebar_position: 100
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "常见问题"
    sidebar_position: 100
    identifier: "actions-faq"
---

# Gitea Actions常见问题解答

本页面包含一些关于Gitea Actions的常见问题和答案。

## 为什么默认情况下不启用Actions？

我们知道为整个实例和每个仓库启用Actions可能很麻烦，但并不是每个人都喜欢或需要此功能。
在我们认为Gitea Actions值得被特别对待之前，我们认为还需要做更多的工作来改进它。

## 是否可以在我的实例中默认启用新仓库的Actions？

是的，当您为实例启用Actions时，您可以选择默认启用actions单元以适用于所有新仓库。

```ini
[repository]
DEFAULT_REPO_UNITS = ...,repo.actions
```

## 在工作流文件中应该使用`${{ github.xyz }}`还是`${{ gitea.xyz }}`？

您可以使用`github.xyz`，Gitea将正常工作。
如前所述，Gitea Actions的设计是与GitHub Actions兼容的。
然而，我们建议在工作流文件中使用`gitea.xyz`，以防止在工作流文件中出现不同类型的密钥（因为您在Gitea上使用此工作流，而不是GitHub）。
不过，这完全是可选的，因为目前这两个选项的效果是相同的。

## 是否可以为特定用户（而不是组织）注册Runner？

目前还不可以。
从技术上讲是可以实现的，但我们需要讨论是否有必要。

## 使用`actions/checkout@v3`等Actions时，Job容器会从何处下载脚本？

您可能知道GitHub上有成千上万个[Actions市场](https://github.com/marketplace?type=actions)。
然而，当您编写`uses: actions/checkout@v3`时，它实际上默认从[gitea.com/actions/checkout](http://gitea.com/actions/checkout)下载脚本（而不是从GitHub下载）。
这是[github.com/actions/checkout](http://github.com/actions/checkout)的镜像，但无法将它们全部镜像。
这就是为什么在尝试使用尚未镜像的某些Actions时可能会遇到失败的原因。

好消息是，您可以指定要从任何位置使用Actions的URL前缀。
这是Gitea Actions中的额外语法。
例如：

- `uses: https://github.com/xxx/xxx@xxx`
- `uses: https://gitea.com/xxx/xxx@xxx`
- `uses: http://your_gitea_instance.com/xxx@xxx`

注意，`https://`或`http://`前缀是必需的！

另外，如果您希望您的Runner默认从GitHub或您自己的Gitea实例下载Actions，可以通过设置 `[actions].DEFAULT_ACTIONS_URL`进行配置。
参见[配置速查表](administration/config-cheat-sheet.md#actions-actions)。

这是与GitHub Actions的一个区别，但它应该允许用户以更灵活的方式运行Actions。

## 如何限制Runner的权限？

Runner仅具有连接到您的Gitea实例的权限。
当任何Runner接收到要运行的Job时，它将临时获得与Job关联的仓库的有限权限。
如果您想为Runner提供更多权限，允许它访问更多私有仓库或外部系统，您可以向其传递[密钥](usage/secrets.md)。

对于 Actions 的细粒度权限控制是一项复杂的工作。
在未来，我们将添加更多选项以使Gitea更可配置，例如允许对仓库进行更多写访问或对同一组织中的所有仓库进行读访问。

## 如何避免被黑客攻击？

有两种可能的攻击类型：未知的Runner窃取您的仓库中的代码或密钥，或恶意脚本控制您的Runner。

避免前者意味着不允许您不认识的人为您的仓库、组织或实例注册Runner。

后者要复杂一些。
如果您为公司使用私有的Gitea实例，您可能不需要担心安全问题，因为您信任您的同事，并且可以追究他们的责任。

对于公共实例，情况略有不同。
以下是我们在 [gitea.com](http://gitea.com/)上的做法：

- 我们仅为 "gitea" 组织注册Runner，因此我们的Runner不会执行来自其他仓库的Job。
- 我们的Runner始终在隔离容器中运行Job。虽然可以直接在主机上进行这样的操作，但出于安全考虑，我们选择不这样做。
- 对于 fork 的拉取请求，需要获得批准才能运行Actions。参见[#22803](https://github.com/go-gitea/gitea/pull/22803)。
- 如果有人在[gitea.com](http://gitea.com/)为其仓库或组织注册自己的Runner，我们不会反对，只是不会在我们的组织中使用它。然而，他们应该注意确保该Runner不被他们不认识的其他用户使用。

## act runner支持哪些操作系统？

它在Linux、macOS和Windows上运行良好。
虽然理论上支持其他操作系统，但需要进一步测试。

需要注意的一点是，如果选择直接在主机上运行Job而不是在Job容器中运行，操作系统之间的环境差异可能会导致意外的失败。

例如，在大多数情况下，Windows上没有可用的bash，而act尝试默认使用bash运行脚本。
因此，您需要在工作流文件中将默认shell指定为`powershell`，参考[defaults.run](https://docs.github.com/zh/actions/using-workflows/workflow-syntax-for-github-actions#defaultsrun)。

```yaml
defaults:
  run:
    shell: powershell
```

## 为什么选择GitHub Actions？为什么不选择与GitLab CI/CD兼容的工具？

[@lunny](https://gitea.com/lunny)在实现Actions的[问题](https://github.com/go-gitea/gitea/issues/13539)中已经解释过这个问题。
此外，Actions不仅是一个CI/CD 系统，还是一个自动化工具。

在开源世界中，已经有许多[市场上的Actions](https://github.com/marketplace?type=actions)实现了。
能够重用它们是令人兴奋的。

## 如果它在多个标签上运行，例如 `runs-on: [label_a, label_b]`，会发生什么？

这是有效的语法。
它意味着它应该在具有`label_a` **和** `label_b`标签的Runner上运行，参考[GitHub Actions的工作流语法](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idruns-on)。
不幸的是，act runner 并不支持这种方式。
如上所述，我们将标签映射到环境：

- `ubuntu` → `ubuntu:22.04`
- `centos` → `centos:8`

但我们需要将标签组映射到环境，例如：

- `[ubuntu]` → `ubuntu:22.04`
- `[with-gpu]` → `linux:with-gpu`
- `[ubuntu, with-gpu]` → `ubuntu:22.04_with-gpu`

我们还需要重新设计任务分配给Runner的方式。
具有`ubuntu`、`centos`或`with-gpu`的Runner并不一定表示它可以接受`[centos, with-gpu]`的Job。
因此，Runner应该通知Gitea实例它只能接受具有 `[ubuntu]`、`[centos]`、`[with-gpu]` 和 `[ubuntu, with-gpu]`的Job。
这不是一个技术问题，只是在早期设计中被忽视了。
参见[runtime.go#L65](https://gitea.com/gitea/act_runner/src/commit/90b8cc6a7a48f45cc28b5ef9660ebf4061fcb336/runtime/runtime.go#L65)。

目前，act runner尝试匹配标签中的每一个，并使用找到的第一个匹配项。

## 代理标签和自定义标签对于Runner有什么区别？

![labels](/images/usage/actions/labels.png)

代理标签是由Runner在注册过程中向Gitea实例报告的。
而自定义标签则是由Gitea的管理员或组织或仓库的所有者手动添加的（取决于Runner所属的级别）。

然而，目前这方面的设计还有待改进，因为它目前存在一些不完善之处。
您可以向已注册的Runner添加自定义标签，比如 `centos`，这意味着该Runner将接收具有`runs-on: centos`的Job。
然而，Runner可能不知道要使用哪个环境来执行该标签，导致它使用默认镜像或导致逻辑死胡同。
这个默认值可能与用户的期望不符。
参见[runtime.go#L71](https://gitea.com/gitea/act_runner/src/commit/90b8cc6a7a48f45cc28b5ef9660ebf4061fcb336/runtime/runtime.go#L71)。

与此同时，如果您想更改Runner的标签，我们建议您重新注册Runner。

## Gitea Actions runner会有更多的实现吗？

虽然我们希望提供更多的选择，但由于我们有限的人力资源，act runner将是唯一受支持的官方Runner。
然而，无论您如何决定，Gitea 和act runner都是完全开源的，所以任何人都可以创建一个新的/更好的实现。
我们支持您的选择，无论您如何决定。
如果您选择分支act runner来创建自己的版本，请在您认为您的更改对其他人也有帮助的情况下贡献这些更改。

## Gitea 支持哪些工作流触发事件？

表格中列出的所有事件都是支持的，并且与 GitHub 兼容。
对于仅 GitHub 支持的事件，请参阅 GitHub 的[文档](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows)。

| 触发事件                    | 活动类型                                                                                                                 |
|-----------------------------|--------------------------------------------------------------------------------------------------------------------------|
| create                      | 不适用                                                                                                                   |
| delete                      | 不适用                                                                                                                   |
| fork                        | 不适用                                                                                                                   |
| gollum                      | 不适用                                                                                                                   |
| push                        | 不适用                                                                                                                   |
| issues                      | `opened`, `edited`, `closed`, `reopened`, `assigned`, `unassigned`, `milestoned`, `demilestoned`, `labeled`, `unlabeled` |
| issue_comment               | `created`, `edited`, `deleted`                                                                                           |
| pull_request                | `opened`, `edited`, `closed`, `reopened`, `assigned`, `unassigned`, `synchronize`, `labeled`, `unlabeled`                |
| pull_request_review         | `submitted`, `edited`                                                                                                    |
| pull_request_review_comment | `created`, `edited`                                                                                                      |
| release                     | `published`, `edited`                                                                                                    |
| registry_package            | `published`                                                                                                              |

> 对于 `pull_request` 事件，在 [GitHub Actions](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#pull_request) 中 `ref` 是 `refs/pull/:prNumber/merge`，它指向这个拉取请求合并提交的一个预览。但是 Gitea 没有这种 reference。
> 因此，Gitea Actions 中 `ref` 是 `refs/pull/:prNumber/head`，它指向这个拉取请求的头分支而不是合并提交的预览。
