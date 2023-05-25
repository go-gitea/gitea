---
date: "2023-05-24T15:00:00+08:00"
title: "Gitea Actions设计"
slug: "design"
weight: 40
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "设计"
    weight: 40
    identifier: "actions-design"
---

# Gitea Actions设计

Gitea Actions由多个组件组成。本文档将对它们进行逐个描述。

**目录**

{{< toc >}}

## Act

[nektos/act](https://github.com/nektos/act) 项目是一个优秀的工具，允许你在本地运行GitHub Actions。
我们受到了它的启发，并思考它是否可能为Gitea运行Actions。

然而，尽管[nektos/act](https://github.com/nektos/act)被设计为一个命令行工具，但我们实际上需要的是一个专为Gitea修改的Go库。
因此，我们在[gitea/act](https://gitea.com/gitea/act)基础上进行了分叉。

这是一个软分叉，将定期跟进上游。
虽然添加了一些自定义提交，但我们会尽力避免对原始代码进行太多更改。

分叉的 act 只是Gitea特定用途的桥接或适配器。
还添加了一些额外的提交，例如：

- 将执行日志输出到日志记录器钩子，以便报告给Gitea
- 禁用 GraphQL URL，因为Gitea不支持它
- 每个Job启动一个新的容器，而不是重复使用，以确保隔离性。

这些修改没有理由合并到上游。
如果用户只想在本地运行可信的Actions，它们是没有意义的。

然而，将来可能会出现重叠，例如两个项目都需要的必要错误修复或新功能。
在这些情况下，我们将向上游仓库贡献变更。

## act runner

Gitea的Runner被称为act runner，因为它基于act。

与其他CIRunner一样，我们将其设计为Gitea的外部部分，这意味着它应该在与Gitea不同的服务器上运行。

为了确保Runner连接到正确的Gitea实例，我们需要使用令牌注册它。
此外，Runner通过声明自己的标签向Gitea报告它可以运行的Job类型。

之前，我们提到工作流文件中的 `runs-on: ubuntu-latest` 表示该Job将在具有`ubuntu-latest`标签的Runner上运行。
但是，Runner如何知道要运行 `ubuntu-latest`？答案在于将标签映射到环境。
这就是为什么在注册过程中添加自定义标签时，需要输入一些复杂内容，比如`my_custom_label:docker://centos:7`。
这意味着Runner可以接受需要在`my_custom_label`上运行的Job，并通过使用`centos:7`镜像的Docker容器来运行它。

然而，Docker不是唯一的选择。
act 也支持直接在主机上运行Job。
这是通过像`linux_arm:host`这样的标签实现的。
这个标签表示Runner可以接受需要在`linux_arm`上运行的Job，并直接在主机上运行它们。

标签的设计遵循格式`label[:schema[:args]]`。
如果省略了schema，则默认为`host`。

因此，

- `my_custom_label:docker://node:18`：使用`node:18 Docker`镜像运行带有`my_custom_label`标签的Job。
- `my_custom_label:host`：在主机上直接运行带有`my_custom_label`标签的Job。
- `my_custom_label`：等同于`my_custom_label:host`。
- `my_custom_label:vm:ubuntu-latest`：（仅为示例，未实现）使用带有`ubuntu-latest` ISO的虚拟机运行带有`my_custom_label`标签的Job。

## 通信协议

由于act runner是Gitea的独立部分，我们需要一种协议让Runner与Gitea实例进行通信。
然而，我们不认为让Gitea监听一个新端口是个好主意。
相反，我们希望重用HTTP端口，这意味着我们需要一个与HTTP兼容的协议。
因此，我们选择使用基于HTTP的gRPC。

我们使用[actions-proto-def](https://gitea.com/gitea/actions-proto-def) 和 [actions-proto-go](https://gitea.com/gitea/actions-proto-go) 进行连接。
有关 gRPC 的更多信息，请访问[其官方网站](https://grpc.io/)。

## 网络架构

让我们来看一下整体的网络架构。
这将帮助您解决一些问题，并解释为什么使用回环地址注册Runner是个不好的主意。

![network](/images/usage/actions/network.png)

图片中标记了四个网络连接，并且箭头的方向表示建立连接的方向。

### 连接 1，act runner到Gitea实例

act runner 必须能够连接到Gitea以接收任务并发送执行结果回来。

### 连接 2，Job容器到Gitea实例

即使Job容器位于同一台机器上，它们的网络命名空间与Runner不同。
举个例子，如果工作流中包含 `actions/checkout@v3`，Job容器需要连接到Gitea来获取代码。
获取代码并不总是运行某些Job所必需的，但在大多数情况下是必需的。

如果您使用回环地址注册Runner，当Runner与Gitea在同一台机器上时，Runner可以连接到Gitea。
然而，如果Job容器尝试从本地主机获取代码，它将失败，因为Gitea不在同一个容器中。

### 连接 3，act runner到互联网

当您使用诸如 `actions/checkout@v3` 的一些Actions时，act runner下载的是脚本，而不是Job容器。
默认情况下，它从[gitea.com](http://gitea.com/)下载，因此需要访问互联网。
它还默认从Docker Hub下载一些Docker镜像，这也需要互联网访问。

然而，互联网访问并不是绝对必需的。
您可以配置您的Gitea实例从您的内部网络设施中获取 Actions 或镜像。

实际上，您的Gitea实例可以同时充当 Actions 市场和镜像注册表。
您可以将GitHub上的Actions仓库镜像到您的Gitea实例，并将其用作普通Actions。
而 [Gitea 容器注册表](https://docs.gitea.io/en-us/usage/packages/container/) 可用作Docker镜像注册表。

### 连接 4，Job容器到互联网

当使用诸如`actions/setup-go@v4`的Actions时，可能需要从互联网下载资源，以设置Job容器中的Go语言环境。
因此，成功完成这些Actions需要访问互联网。

然而，这也是可选的。
您可以使用自定义的Actions来避免依赖互联网访问，或者可以使用已安装所有依赖项的打包的Docker镜像来运行Job。

## 总结

使用Gitea Actions只需要确保Runner能够连接到Gitea实例。
互联网访问是可选的，但如果没有互联网访问，将需要额外的工作。
换句话说：当Runner能够自行查询互联网时，它的工作效果最好，但您不需要将其暴露给互联网（无论是单向还是双向）。

如果您在使用Gitea Actions时遇到任何网络问题，希望上面的图片能够帮助您进行故障排除。
