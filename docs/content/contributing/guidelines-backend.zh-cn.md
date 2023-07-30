---
date: "2023-05-25T23:41:00+08:00"
title: "后端开发指南"
slug: "guidelines-backend"
sidebar_position: 20
toc: false
draft: false
aliases:
  - /zh-cn/guidelines-backend
menu:
  sidebar:
    parent: "contributing"
    name: "后端开发指南"
    sidebar_position: 20
    identifier: "guidelines-backend"
---

# 后端开发指南

## 背景

Gitea使用Golang作为后端编程语言。它使用了许多第三方包，并且自己也编写了一些包。
例如，Gitea使用[Chi](https://github.com/go-chi/chi)作为基本的Web框架。[Xorm](https://xorm.io)是一个用于与数据库交互的ORM框架。
因此，管理这些包非常重要。在开始编写后端代码之前，请参考以下准则。

## 包设计准则

### 包列表

为了保持易于理解的代码并避免循环依赖，拥有良好的代码结构是很重要的。Gitea后端分为以下几个部分：

- `build`：帮助构建Gitea的脚本。
- `cmd`：包含所有Gitea的实际子命令，包括web、doctor、serv、hooks、admin等。`web`将启动Web服务。`serv`和`hooks`将被Git或OpenSSH调用。其他子命令可以帮助维护Gitea。
- `tests`：常用的测试函数
- `tests/integration`：集成测试，用于测试后端回归。
- `tests/e2e`：端到端测试，用于测试前端和后端的兼容性和视觉回归。
- `models`：包含由xorm用于构建数据库表的数据结构。它还包含查询和更新数据库的函数。应避免与其他Gitea代码的依赖关系。在某些情况下，比如日志记录时可以例外。
  - `models/db`：基本的数据库操作。所有其他`models/xxx`包都应依赖于此包。`GetEngine`函数只能从models/中调用。
  - `models/fixtures`：单元测试和集成测试中使用的示例数据。一个`yml`文件表示一个将在测试开始时加载到数据库中的表。
  - `models/migrations`：存储不同版本之间的数据库迁移。修改数据库结构的PR**必须**包含一个迁移步骤。
- `modules`：在Gitea中处理特定功能的不同模块。工作正在进行中：其中一些模块应该移到`services`中，特别是那些依赖于models的模块，因为它们依赖于数据库。
  - `modules/setting`：存储从ini文件中读取的所有系统配置，并在各处引用。但是在可能的情况下，应将其作为函数参数使用。
  - `modules/git`：用于与`Git`命令行或Gogit包交互的包。
- `public`：编译后的前端文件（JavaScript、图像、CSS等）
- `routers`：处理服务器请求。由于它使用其他Gitea包来处理请求，因此其他包（models、modules或services）不能依赖于routers。
  - `routers/api`：包含`/api/v1`相关路由，用于处理RESTful API请求。
  - `routers/install`：只能在系统处于安装模式（INSTALL_LOCK=false）时响应。
  - `routers/private`：仅由内部子命令调用，特别是`serv`和`hooks`。
  - `routers/web`：处理来自Web浏览器或Git SMART HTTP协议的HTTP请求。
- `services`：用于常见路由操作或命令执行的支持函数。使用`models`和`modules`来处理请求。
- `templates`：用于生成HTML输出的Golang模板。

### 包依赖关系

由于Golang不支持导入循环，我们必须仔细决定包之间的依赖关系。这些包之间有一些级别。以下是理想的包依赖关系方向。

`cmd` -> `routers` -> `services` -> `models` -> `modules`

从左到右，左侧的包可以依赖于右侧的包，但右侧的包不能依赖于左侧的包。在同一级别的子包中，可以根据该级别的规则进行依赖。

**注意事项**

为什么我们需要在`models`之外使用数据库事务？以及如何使用？
某些操作在数据库记录插入/更新/删除失败时应该允许回滚。
因此，服务必须能够创建数据库事务。以下是一些示例：

```go
// services/repository/repository.go
func CreateXXXX() error {
    return db.WithTx(func(ctx context.Context) error {
        // do something, if err is returned, it will rollback automatically
        if err := issues.UpdateIssue(ctx, repoID); err != nil {
            // ...
            return err
        }
        // ...
        return nil
    })
}
```

在`services`中**不应该**直接使用`db.GetEngine(ctx)`，而是应该在`models/`下编写一个函数。
如果该函数将在事务中使用，请将`context.Context`作为函数的第一个参数。

```go
// models/issues/issue.go
func UpdateIssue(ctx context.Context, repoID int64) error {
    e := db.GetEngine(ctx)

    // ...
}
```

### 包名称

对于顶层包，请使用复数作为包名，例如`services`、`models`，对于子包，请使用单数，例如`services/user`、`models/repository`。

### 导入别名

由于有一些使用相同包名的包，例如`modules/user`、`models/user`和`services/user`，当这些包在一个Go文件中被导入时，很难知道我们使用的是哪个包以及它是变量名还是导入名。因此，我们始终建议使用导入别名。为了与常见的驼峰命名法的包变量区分开，建议使用**snake_case**作为导入别名的命名规则。
例如：`import user_service "code.gitea.io/gitea/services/user"`

### 重要注意事项

- 永远不要写成`x.Update(exemplar)`，而没有明确的`WHERE`子句：
  - 这将导致表中的所有行都被使用exemplar的非零值进行更新，包括ID。
  - 通常应该写成`x.ID(id).Update(exemplar)`。
- 如果在迁移过程中使用`x.Insert(exemplar)`向表中插入记录，而ID是预设的：
  - 对于MSSQL变体，你将需要执行``SET IDENTITY_INSERT `table` ON``（否则迁移将失败）
  - 对于PostgreSQL，你还需要更新ID序列，否则迁移将悄无声息地通过，但后续的插入将失败：
    ``SELECT setval('table_name_id_seq', COALESCE((SELECT MAX(id)+1 FROM `table_name`), 1), false)``

### 未来的任务

目前，我们正在进行一些重构，以完成以下任务：

- 纠正不符合规则的代码。
- `models`中的文件太多了，所以我们正在将其中的一些移动到子包`models/xxx`中。
- 由于它们依赖于`models`，因此应将某些`modules`子包移动到`services`中。
