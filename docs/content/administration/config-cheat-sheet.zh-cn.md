---
date: "2016-12-26T16:00:00+02:00"
title: "配置说明"
slug: "config-cheat-sheet"
sidebar_position: 30
toc: false
draft: false
aliases:
  - /zh-cn/config-cheat-sheet
menu:
  sidebar:
    parent: "administration"
    name: "配置说明"
    sidebar_position: 30
    identifier: "config-cheat-sheet"
---

# 配置说明

这是针对Gitea配置文件的说明，
你可以了解Gitea的强大配置。

需要说明的是，你的所有改变请修改 `custom/conf/app.ini` 文件而不是源文件。
如果是从发行版本完成的安装，
配置文件的路径为`/etc/gitea/conf/app.ini`。

所有默认值可以通过 [app.example.ini](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini) 查看到。
如果你发现 `%(X)s` 这样的内容，请查看
[ini](https://github.com/go-ini/ini/#recursive-values) 这里的说明。
标注了 :exclamation: 的配置项表明除非你真的理解这个配置项的意义，否则最好使用默认值。

在下面的默认值中,`$XYZ`代表环境变量`XYZ`的值(详见:`environment-to-ini`)。 _`XxYyZz`_是指默认配置的一部分列出的值。这些在 app.ini 文件中不起作用，仅在此处列出作为文档说明。

包含`#`或者`;`的变量必须使用引号(`` ` ``或者`""""`)包裹，否则会被解析为注释。

**注意:** 修改完配置文件后，需要重启 Gitea 服务才能生效。

## 默认配置 (非`app.ini`配置文件)

这些值取决于环境，但构成了许多值的基础。当运行 `gitea help`或启动时，它们将
作为默认配置的一部分进行报告。它们在那里发出的顺序略有不同，但我们将按照设置的顺序在这里列出。

- _`AppPath`_: Gitea二进制可执行文件的绝对路径
- _`AppWorkPath`_: Gitea可执行文件的工作目录。 该配置可以通过以下几种方式设置，优先级依次递减:
  - `app.ini`中的`WORK_PATH`配置项
  - 启动Gitea时的`--work-path`命令行参数
  - `$GITEA_WORK_DIR`环境变量
  - 在编译时设置的内置值（参见从源代码编译）
  - 默认为 _`AppPath`_ 的目录
  - 如果上述任何路径为相对路径，将自动解析为相对于 _`AppPath`_ 目录的绝对路径
- _`CustomPath`_: 这是用于自定义模板和其他选项的基础目录。
它是通过使用以下层次结构中的第一个设置的内容来确定的：
  - 通过传递给二进制文件的`--custom-path`标志
  - 环境变量 `$GITEA_CUSTOM`
  - 在构建时设置的内置值（参见从源代码构建）
  - 否则，默认为 _`AppWorkPath`_`/custom`
  - 如果上述任何路径是相对路径，则会相对于 _`AppWorkPath`_ 目录进行处理，
  使其变为绝对路径。
- _`CustomConf`_: 这是指向`app.ini`文件的路径。
  - 这是指向`app.ini`文件的路径。
  - 在构建时设置的内置值（参见从源代码构建）
  - 否则，默认为 _`CustomPath`_`/conf/app.ini`
  - 如果上述任何路径是相对路径，则会相对于_`CustomPath`_目录进行处理。

此外，还有_`StaticRootPath`_，可以在构建时设置为内置值，否则将默认为 _`AppWorkPath`_。

## Overall (`DEFAULT`)

- `APP_NAME`: **Gitea: Git with a cup of tea** 应用名称，在网页的标题中显示。
- `RUN_USER`: **_current OS username_/`$USER`/`$USERNAME` e.g. git**: 运行Gitea的用户，
  应当是一个专用的系统账户(非用户使用，推荐创建一个专用的`git`用户). 如果在你自己的个人电脑使用改成你自己的用户名。
  该配置如果设置不正确，Gitea可能崩溃。
- `RUN_MODE`: **prod**: 应用的运行模式，对运行性能和问题排除有影响: `dev` 或者 `prod`,默认为 `prod`。 `dev`模式有助于开发和问题排查, 除设置为`dev` 外，均被视为 `prod`.
- `WORK_PATH`: **_the-work-path_**: 工作目录, 前文有提及.

## 仓库 (`repository`)

- `ROOT`: **%(APP_DATA_PATH)s/gitea-repositories**: 存放git工程的根目录，建议填绝对路径。
  相对路径将被解析为**_`AppWorkPath`_/%(ROOT)s**.
- `SCRIPT_TYPE`: **bash**: 服务器支持的Shell类型，通常是`bash`，
  但有些服务器也有可能是`sh`。
- `DETECTED_CHARSETS_ORDER`: **UTF-8, UTF-16BE, UTF-16LE, UTF-32BE, UTF-32LE, ISO-8859, windows-1252, ISO-8859, windows-1250, ISO-8859, ISO-8859, ISO-8859, windows-1253, ISO-8859, windows-1255, ISO-8859, windows-1251, windows-1256, KOI8-R, ISO-8859, windows-1254, Shift_JIS, GB18030, EUC-JP, EUC-KR, Big5, ISO-2022, ISO-2022, ISO-2022, IBM424_rtl, IBM424_ltr, IBM420_rtl, IBM420_ltr**: 检测到的字符集的决定性顺序 - 如果检测到的字符集具有相等的置信度，则优先选择列表中较早出现的字符集，而不是较晚出现的字符集。添加“defaults”将会将未命名的字符集放置在该点。
- `ANSI_CHARSET`: **_empty_**: 默认的ANSI字符集，用于覆盖非UTF-8字符集。
- `FORCE_PRIVATE`: **false**: 强制使每个新仓库变为私有。
- `DEFAULT_PRIVATE`: **last**: 创建新仓库时默认为私有。
   \[last, private, public\]
- `DEFAULT_PUSH_CREATE_PRIVATE`: **true**: 使用推送创建新仓库时默认为私有。
- `MAX_CREATION_LIMIT`: **-1**: 每个用户的全局仓库创建上限,
   `-1` 代表无限制.
- `PREFERRED_LICENSES`: **Apache License 2.0,MIT License**: 要放置在列表顶部的指定许可证。
   名称必须与 options/license 或 custom/options/license 中的文件名匹配。
- `DISABLE_HTTP_GIT`: **false**: 禁用HTTP协议与仓库进行
      交互的能力。
- `USE_COMPAT_SSH_URI`: **false**: 当使用默认的SSH端口时，强制使用ssh://克隆URL，
  而不是scp-style uri。
- `GO_GET_CLONE_URL_PROTOCOL`: **https**: 用于 "go get" 请求的值，返回仓库的URL作为https或ssh，
    默认为https。
- `ACCESS_CONTROL_ALLOW_ORIGIN`: **_empty_**:用于 Access-Control-Allow-Origin 标头的值，
默认不提供。
警告：如果您不提供正确的值，这可能对您的网站造成危害。
- `DEFAULT_CLOSE_ISSUES_VIA_COMMITS_IN_ANY_BRANCH`:  **false**: 如果非默认分支上的提交将问题标记为已关闭，则关闭该问题。
- `ENABLE_PUSH_CREATE_USER`:  **false**: 允许用户将本地存储库推送到Gitea，并为用户自动创建它们。
- `ENABLE_PUSH_CREATE_ORG`:  **false**:  允许用户将本地存储库推送到Gitea，并为组织自动创建它们。
- `DISABLED_REPO_UNITS`: **_empty_**: 逗号分隔的全局禁用的仓库单元列表。允许的值是：: \[repo.issues, repo.ext_issues, repo.pulls, repo.wiki, repo.ext_wiki, repo.projects, repo.packages, repo.actions\]
- `DEFAULT_REPO_UNITS`: **repo.code,repo.releases,repo.issues,repo.pulls,repo.wiki,repo.projects,repo.packages,repo.actions**: 逗号分隔的默认新仓库单元列表。允许的值是：: \[repo.code, repo.releases, repo.issues, repo.pulls, repo.wiki, repo.projects, repo.packages, repo.actions\]. 注意：目前无法停用代码和发布。如果您指定了默认的仓库单元，您仍应将它们列出以保持未来的兼容性。外部wiki和问题跟踪器不能默认启用，因为它需要额外的设置。禁用的仓库单元将不会添加到新的仓库中，无论它是否在默认列表中。
- `DEFAULT_FORK_REPO_UNITS`: **repo.code,repo.pulls**: 逗号分隔的默认分叉仓库单元列表。允许的值和规则与`DEFAULT_REPO_UNITS`相同。
- `PREFIX_ARCHIVE_FILES`: **true**: 通过将存档文件放置在以仓库命名的目录中来添加前缀。
- `DISABLE_MIGRATIONS`: **false**: 禁用迁移功能。
- `DISABLE_STARS`: **false**: 禁用点赞功能。
- `DEFAULT_BRANCH`: **main**: 所有仓库的默认分支名称。
- `ALLOW_ADOPTION_OF_UNADOPTED_REPOSITORIES`: **false**: 允许非管理员用户认领未被认领的仓库。
- `ALLOW_DELETION_OF_UNADOPTED_REPOSITORIES`: **false**: 允许非管理员用户删除未被认领的仓库。
- `DISABLE_DOWNLOAD_SOURCE_ARCHIVES`: **false**: 不允许从用户界面下载源代码存档文件。
- `ALLOW_FORK_WITHOUT_MAXIMUM_LIMIT`: **true**: 允许无限制得派生仓库。

### 仓库 - 编辑器 (`repository.editor`)

- `LINE_WRAP_EXTENSIONS`: **.txt,.md,.markdown,.mdown,.mkd,.livemd,**: 在 Monaco 编辑器中应该换行的文件扩展名列表。用逗号分隔扩展名。要对没有扩展名的文件进行换行，只需放置一个逗号。
- `PREVIEWABLE_FILE_MODES`: **markdown**: 具有预览API的有效文件模式，例如 `api/v1/markdown`。用逗号分隔各个值。如果文件扩展名不匹配，编辑模式下的预览选项卡将不会显示。

### 仓库 - 合并请求 (`repository.pull-request`)

- `WORK_IN_PROGRESS_PREFIXES`: **WIP:,\[WIP\]**: 在拉取请求标题中用于标记工作正在进行中的前缀列表。
这些前缀在不区分大小写的情况下进行匹配。
- `CLOSE_KEYWORDS`: **close**, **closes**, **closed**, **fix**, **fixes**, **fixed**, **resolve**, **resolves**, **resolved**: 在拉取请求评论中用于自动关闭相关问题的关键词列表。
- `REOPEN_KEYWORDS`: **reopen**, **reopens**, **reopened**: 在拉取请求评论中用于自动重新打开相关问题的
关键词列表。
- `DEFAULT_MERGE_STYLE`: **merge**: 设置创建仓库的默认合并方式，可选: `merge`, `rebase`, `rebase-merge`, `squash`, `fast-forward-only`
- `DEFAULT_MERGE_MESSAGE_COMMITS_LIMIT`: **50**: 在默认合并消息中，对于`squash`提交，最多包括此数量的提交。设置为 -1 以包括所有提交。
- `DEFAULT_MERGE_MESSAGE_SIZE`: **5120**: 在默认的合并消息中，对于`squash`提交，限制提交消息的大小。设置为 `-1`以取消限制。仅在`POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES`为`true`时使用。
- `DEFAULT_MERGE_MESSAGE_ALL_AUTHORS`: **false**: 在默认合并消息中，对于`squash`提交，遍历所有提交以包括所有作者的`Co-authored-by`，否则仅使用限定列表中的作者。
- `DEFAULT_MERGE_MESSAGE_MAX_APPROVERS`: **10**:在默认合并消息中，限制列出的审批者数量为`Reviewed-by`:。设置为 `-1` 以包括所有审批者。
- `DEFAULT_MERGE_MESSAGE_OFFICIAL_APPROVERS_ONLY`: **true**: 在默认合并消息中，仅包括官方允许审查的审批者。
- `POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES`: **false**: 在默认的 squash 合并消息中，包括构成拉取请求的所有提交的提交消息。
- `ADD_CO_COMMITTER_TRAILERS`: **true**: 如果提交者与作者不匹配，在合并提交消息中添加`co-authored-by`和`co-committed-by`标记。
- `TEST_CONFLICTING_PATCHES_WITH_GIT_APPLY`:使用三方合并方法测试`PR Patch`以发现是否存在冲突。如果此设置`true`，将使用`git apply`重新测试冲突的`PR Pathch` - 这是1.18（和之前版本）中的先前行为，但效率相对较低。如果发现需要此设置，请报告。

### 仓库 - 工单 (`repository.issue`)

- `LOCK_REASONS`: **Too heated,Off-topic,Resolved,Spam**: 合并请求或工单被锁定的原因列表。
- `MAX_PINNED`: **3**: 每个仓库的最大可固定工单数量。设置为0禁用固定工单。

### 仓库 - 文件上传 (`repository.upload`)

- `ENABLED`: **true**: 是否启用仓库文件上传。
- `TEMP_PATH`: **data/tmp/uploads**: 文件上传的临时保存路径(在Gitea重启的时候该目录会被清空)。
- `ALLOWED_TYPES`: **_empty_**: 以逗号分割的列表，代表支持上传的文件类型。(`.zip`), mime类型 (`text/plain`) or 通配符类型 (`image/*`, `audio/*`, `video/*`). 为空或者 `*/*`代表允许所有类型文件。
- `FILE_MAX_SIZE`: **50**: 每个文件的最大大小(MB)。
- `MAX_FILES`: **5**: 每次上传的最大文件数。

### 仓库 - 版本发布 (`repository.release`)

- `ALLOWED_TYPES`: **_empty_**: 允许发布的文件类型列表，用逗号分隔 。如压缩包类型(`.zip`), mime 类型 (`text/plain`) ，也支持通配符 (`image/*`, `audio/*`, `video/*`)。 空值或者 `*/*` 代表允许所有类型。
- `DEFAULT_PAGING_NUM`: **10**: 默认的发布版本页面分页大小
- 关于版本发布相关的附件设置，详见`附件`部分。

### 仓库 - Signing (`repository.signing`)

- `SIGNING_KEY`: **default**: \[none, KEYID, default \]: 用于签名的密钥
- `SIGNING_NAME` &amp; `SIGNING_EMAIL`: 如果`SIGNING_KEY`提供了一个 KEYID，将使用这些作为签名者的姓名和电子邮件地址。这些应与密钥的公开姓名和电子邮件地址相匹配。
- `INITIAL_COMMIT`: **always**: \[never, pubkey, twofa, always\]: 签名初始提交。
  - `never`: 永不签名
  - `pubkey`: 仅在用户具有公钥时签名
  - `twofa`: 仅在用户使用双因素身份验证登录时签名
  - `always`: 始终签名
  - 除了 never 和 always 之外的选项可以组合为逗号分隔的列表。
- `DEFAULT_TRUST_MODEL`: **collaborator**: \[collaborator, committer, collaboratorcommitter\]: 用于验证提交的默认信任模型。
  - `collaborator`: 信任协作者密钥签名的签名。
  - `committer`: 信任与提交者匹配的签名（这与GitHub匹配，并会强制Gitea签名的提交具有Gitea作为提交者）。
  - `collaboratorcommitter`: 信任与提交者匹配的协作者密钥签名的签名。
- `WIKI`: **never**: \[never, pubkey, twofa, always, parentsigned\]: 对wiki提交进行签名。
- `CRUD_ACTIONS`: **pubkey, twofa, parentsigned**: \[never, pubkey, twofa, parentsigned, always\]: 对CRUD操作进行签名。
  - 与上面相同的选项，增加了：
  - `parentsigned`: 仅在父提交进行了签名时才进行签名。
- `MERGES`: **pubkey, twofa, basesigned, commitssigned**: \[never, pubkey, twofa, approved, basesigned, commitssigned, always\]: 对合并操作进行签名。
  - `approved`: 仅对已批准的合并操作进行签名，适用于受保护的分支。
  - `basesigned`: 仅在基础仓库的父提交进行了签名时才进行签名。
  - `headsigned`: 仅在头分支的头提交进行了签名时才进行签名。
  - `commitssigned`: 仅在头分支中的所有提交到合并点都进行了签名时才进行签名。

### 仓库 - Local (`repository.local`)

- `LOCAL_COPY_PATH`: **tmp/local-repo**:临时本地仓库副本的路径。默认为 tmp/local-repo（内容在 Gitea 重新启动时被删除）

### 仓库 -  MIME type mapping (`repository.mimetype_mapping`)

配置用于根据可下载文件的文件扩展名设置预期的 MIME 类型。配置以键值对的形式呈现，文件扩展名以`.`开头。

以下配置在下载具有`.apk`文件扩展名的文件时设置`Content-Type: application/vnd.android.package-archive`头部。

```ini
.apk=application/vnd.android.package-archive
```

## 跨域 (`cors`)

- `ENABLED`: **false**: 启用 CORS 头部（默认禁用）
- `ALLOW_DOMAIN`: **\***: 允许请求的域名列表
- `METHODS`: **GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS**: 允许发起的请求方式列表
- `MAX_AGE`: **10m**: 缓存响应的最大时间
- `ALLOW_CREDENTIALS`: **false**: 允许带有凭据的请求
- `HEADERS`: **Content-Type,User-Agent**: 允许请求携带的头部
- `X_FRAME_OPTIONS`: **SAMEORIGIN**: 详见 `X-Frame-Options`HTTP头部.

## 界面 (`ui`)

- `EXPLORE_PAGING_NUM`: **20**: 探索页面每页显示的仓库数量。
- `ISSUE_PAGING_NUM`: **20**: 工单页面每页显示的工单数量。
- `MEMBERS_PAGING_NUM`: **20**: 组织成员页面每页显示的成员数量。
- `FEED_MAX_COMMIT_NUM`: **5**: 活动流页面显示的最大提交数量。
- `FEED_PAGING_NUM`: **20**: 活动流页面显示的最大活动数量。
- `SITEMAP_PAGING_NUM`: **20**: 在单个子SiteMap中显示的项数。
- `GRAPH_MAX_COMMIT_NUM`: **100**: 提交图中显示的最大commit数量。
- `CODE_COMMENT_LINES`: **4**: 在代码评论中能够显示的最大代码行数。
- `DEFAULT_THEME`: **gitea-auto**: 在Gitea安装时候设置的默认主题，自定义的主题可以通过 `{CustomPath}/public/assets/css/theme-*.css` 提供。
- `SHOW_USER_EMAIL`: **true**: 用户的电子邮件是否应该显示在`Explore Users`页面中。
- `THEMES`:  **_empty_**: 所有可用的主题（由 `{CustomPath}/public/assets/css/theme-*.css` 提供）。允许用户选择个性化的主题，
- `MAX_DISPLAY_FILE_SIZE`: **8388608**: 能够显示文件的最大大小（默认为8MiB）。
- `REACTIONS`: 用户可以在问题（Issue）、Pull Request（PR）以及评论中选择的所有可选的反应。
    这些值可以是表情符号别名（例如：:smile:）或Unicode表情符号。
    对于自定义的反应，在 public/assets/img/emoji/ 目录下添加一个紧密裁剪的正方形图像，文件名为 reaction_name.png。
- `CUSTOM_EMOJIS`: **gitea, codeberg, gitlab, git, github, gogs**: 不在utf8标准中定义的额外表情符号。
    默认情况下，我们支持 Gitea 表情符号（:gitea:）。要添加更多表情符号，请将它们复制到 public/assets/img/emoji/ 目录下，
    并将其添加到此配置中。
- `DEFAULT_SHOW_FULL_NAME`: **false**: 是否在可能的情况下显示用户的全名。如果没有设置全名，则将使用用户名。
- `SEARCH_REPO_DESCRIPTION`: **true**: 是否在探索页面上的仓库搜索中搜索描述。
- `ONLY_SHOW_RELEVANT_REPOS`: **false** 在没有指定关键字并使用默认排序时，是否仅在探索页面上显示相关的仓库。
    如果一个仓库是分叉或者没有元数据（没有描述、图标、主题），则被视为不相关的仓库。

### 界面 - 管理员 (`ui.admin`)

- `USER_PAGING_NUM`: **50**: 单页显示的用户数量。
- `REPO_PAGING_NUM`: **50**: 单页显示的仓库数量。
- `NOTICE_PAGING_NUM`: **25**: 单页显示的通知数量。
- `ORG_PAGING_NUM`: **50**: 单页显示的组织数量。

### 界面 - 用户 (`ui.user`)

- `REPO_PAGING_NUM`: **15**: 单页显示的仓库数量。

### 界面 - 元信息 (`ui.meta`)

- `AUTHOR`: **Gitea - Git with a cup of tea**: 主页的作者元标签。
- `DESCRIPTION`: **Gitea (Git with a cup of tea) is a painless self-hosted Git service written in Go**: 主页的描述元标签。
- `KEYWORDS`: **go,git,self-hosted,gitea**: 首页关键词元标签。

### 界面 - 通知 (`ui.notification`)

- `MIN_TIMEOUT`: **10s**: 这些选项控制通知端点定期轮询以更新通知计数。在页面加载后，通知计数将在` MIN_TIMEOUT`之后进行检查。如果通知计数未更改，超时时间将按照`TIMEOUT_STEP`增加到`MAX_TIMEOUT`。将 `MIN_TIMEOUT`设置为 -1 以关闭该功能。
- `MAX_TIMEOUT`: **60s**.
- `TIMEOUT_STEP`: **10s**.
- `EVENT_SOURCE_UPDATE_TIME`: **10s**: 该设置确定了查询数据库以更新通知计数的频率。如果浏览器客户端支持`EventSource`和`SharedWorker`，则优先使用`SharedWorker`而不是轮询通知端点。将其设置为 -1 可以禁用 `EventSource`。

### 界面 - SVG Images (`ui.svg`)

- `ENABLE_RENDER`: **true**: 是否将SVG文件呈现为图像。如果禁用了SVG渲染，SVG文件将显示为文本，无法作为图像嵌入到Markdown文件中。

### 界面 - CSV Files (`ui.csv`)

- `MAX_FILE_SIZE`: **524288** (512kb): 以字节为单位允许将CSV文件呈现为表格的最大文件大小（将其设置为0表示没有限制）。

## Markdown (`markdown`)

- `ENABLE_HARD_LINE_BREAK_IN_COMMENTS`: **true**: 在评论中将软换行符呈现为硬换行符，
  这意味着段落之间的单个换行符将导致换行，
  并且不需要在段落后添加尾随空格来强制换行。
- `ENABLE_HARD_LINE_BREAK_IN_DOCUMENTS`: **false**: 在文档中将软换行符呈现为硬换行符，
  这意味着段落之间的单个换行符将导致换行，
  并且不需要在段落后添加尾随空格来强制换行。
- `CUSTOM_URL_SCHEMES`: 使用逗号分隔的列表（ftp、git、svn）来指示要在Markdown中呈现的附加URL超链接。
  以http和https开头的URL始终显示。
  如果此条目为空，则允许所有URL方案。
- `FILE_EXTENSIONS`: **.md,.markdown,.mdown,.mkd,.livemd**: 应呈现/编辑为Markdown的文件扩展名列表。使用逗号分隔扩展名。要将没有任何扩展名的文件呈现为Markdown，请只需放置一个逗号。
- `ENABLE_MATH`: **true**: 启用对`\(...\)`, `\[...\]`, `$...$`和`$$...$$` 作为数学块的检测。

## 服务器 (`server`)

- `APP_DATA_PATH`: **_`AppWorkPath`_/data**: 这是存储数据的默认根路径。
- `PROTOCOL`: **http**: \[http, https, fcgi, http+unix, fcgi+unix\]
- `USE_PROXY_PROTOCOL`: **false**: 在连接中预期`PROXY`协议头。
- `PROXY_PROTOCOL_TLS_BRIDGING`: **false**: 协议为 https 时，在`TLS`协商后预期`PROXY`协议头。
- `PROXY_PROTOCOL_HEADER_TIMEOUT`: **5s**: 等待`PROXY`协议头的超时时间（设置为`0`表示没有超时）。
- `PROXY_PROTOCOL_ACCEPT_UNKNOWN`: **false**:接受带有未知类型的`PROXY`协议头。
- `DOMAIN`: **localhost**: 此服务器的域名。
- `ROOT_URL`: **%(PROTOCOL)s://%(DOMAIN)s:%(HTTP\_PORT)s/**:
   覆盖自动生成的公共URL。
   如果内部URL和外部URL不匹配（例如在Docker中），这很有用。
- `STATIC_URL_PREFIX`: **_empty_**:
    覆盖此选项以从不同的URL请求静态资源。
    这包括CSS文件、图片、JS文件和Web字体。
    头像图片是动态资源，仍由Gitea提供。
   选项可以是不同的路径，例如`/static`, 也可以是另一个域，例如`https://cdn.example.com`.
   请求会变成 `%(ROOT_URL)s/static/assets/css/index.css` 或 `https://cdn.example.com/assets/css/index.css`
   静态文件位于Gitea源代码仓库的`public/`目录中。
   您可以将`STATIC_URL_PREFIX`请求代理到 Gitea 服务器以提供静态资源，或者将手动构建的 Gitea 资源从 `$GITEA_BUILD/public`复制到静态位置，例如`/var/www/assets`。确保`$STATIC_URL_PREFIX/assets/css/index.css`指向`/var/www/assets/css/index.css`。

- `HTTP_ADDR`: **0.0.0.0**：HTTP 监听地址。
  - 如果 `PROTOCOL` 设置为 `fcgi`，Gitea 将在由
  `HTTP_ADDR` 和 `HTTP_PORT` 配置设置定义的 TCP 套接字上监听 FastCGI 请求。
  - 如果 `PROTOCOL` 设置为 `http+unix` 或 `fcgi+unix`，则应该是要使用的 Unix 套接字文件的名称。相对路径将相对于 _`AppWorkPath`_ 被转换为绝对路径。
- `HTTP_PORT`: **3000**：HTTP 监听端口。
  - 如果 `PROTOCOL` 设置为 `fcgi`，Gitea 将在由 `HTTP_ADDR` 和 `HTTP_PORT`
  配置设置定义的 TCP 套接字上监听 FastCGI 请求。
- `UNIX_SOCKET_PERMISSION`: **666**：Unix 套接字的权限。
- `LOCAL_ROOT_URL`: **%(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/**：
  用于访问网络服务的 Gitea 工作器（例如 SSH 更新）的本地（DMZ）URL。
  在大多数情况下，您不需要更改默认值。
  仅在您的 SSH 服务器节点与 HTTP 节点不同的情况下才修改它。对于不同的协议，默认值不同。如果 `PROTOCOL`
  是 `http+unix`，则默认值为 `http://unix/`。如果 `PROTOCOL` 是 `fcgi` 或 `fcgi+unix`，则默认值为
  `%(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/`。如果监听在 `0.0.0.0`，则默认值为
  `%(PROTOCOL)s://localhost:%(HTTP_PORT)s/`，
  否则默认值为 `%(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/`。
- `LOCAL_USE_PROXY_PROTOCOL`: **%(USE_PROXY_PROTOCOL)s**：在进行本地连接时传递 PROXY 协议头。
   如果本地连接将经过代理，请将其设置为 false。
- `PER_WRITE_TIMEOUT`: **30s**：连接的任何写操作的超时时间。（将其设置为 -1
  以禁用所有超时。）
- `PER_WRITE_PER_KB_TIMEOUT`: **10s**：连接每写入 1 KB 的超时时间。
- `DISABLE_SSH`: **false**：当SSH不可用时禁用SSH功能。
- `START_SSH_SERVER`: **false**：启用时，使用内置的SSH服务器。
- `SSH_SERVER_USE_PROXY_PROTOCOL`: **false**：在与内置SSH服务器建立连接时，期望PROXY协议头。
- `BUILTIN_SSH_SERVER_USER`: **%(RUN_USER)s**：用于内置SSH服务器的用户名。
- `SSH_USER`: **%(BUILTIN_SSH_SERVER_USER)s**：在克隆URL中显示的SSH用户名。这仅适用于自行配置SSH服务器的人；在大多数情况下，您希望将其留空并修改`BUILTIN_SSH_SERVER_USER`。
- `SSH_DOMAIN`: **%(DOMAIN)s**：此服务器的域名，用于显示的克隆 URL。
- `SSH_PORT`: **22**：显示在克隆 URL 中的 SSH 端口。
- `SSH_LISTEN_HOST`: **0.0.0.0**：内置 SSH 服务器的监听地址。
- `SSH_LISTEN_PORT`: **%(SSH_PORT)s**：内置 SSH 服务器的端口。
- `SSH_ROOT_PATH`: **~/.ssh**：SSH 目录的根路径。
- `SSH_CREATE_AUTHORIZED_KEYS_FILE`: **true**：当 Gitea 不使用内置 SSH 服务器时，默认情况下 Gitea 会创建一个 authorized_keys 文件。如果您打算使用 AuthorizedKeysCommand 功能，您应该关闭此选项。
- `SSH_AUTHORIZED_KEYS_BACKUP`: **false**：在重写所有密钥时启用 SSH 授权密钥备份，默认值为 false。
- `SSH_TRUSTED_USER_CA_KEYS`: **_empty_**：指定信任的证书颁发机构的公钥，用于对用户证书进行身份验证。多个密钥应以逗号分隔。例如 `ssh-<algorithm> <key>` 或 `ssh-<algorithm> <key1>, ssh-<algorithm> <key2>`。有关详细信息，请参阅 `sshd` 配置手册中的 `TrustedUserCAKeys` 部分。当为空时，不会创建文件，并且 `SSH_AUTHORIZED_PRINCIPALS_ALLOW` 默认为 `off`。
- `SSH_TRUSTED_USER_CA_KEYS_FILENAME`: **`RUN_USER`/.ssh/gitea-trusted-user-ca-keys.pem**：Gitea 将管理的 `TrustedUserCaKeys` 文件的绝对路径。如果您正在运行自己的 SSH 服务器，并且想要使用 Gitea 管理的文件，您还需要修改您的 `sshd_config` 来指向此文件。官方的 Docker 映像将自动工作，无需进一步配置。
- `SSH_AUTHORIZED_PRINCIPALS_ALLOW`: **off** 或 **username, email**：\[off, username, email, anything\]：指定允许用户用作 principal 的值。当设置为 `anything` 时，对 principal 字符串不执行任何检查。当设置为 `off` 时，不允许设置授权的 principal。
- `SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE`: **false/true**：当 Gitea 不使用内置 SSH 服务器且 `SSH_AUTHORIZED_PRINCIPALS_ALLOW` 不为 `off` 时，默认情况下 Gitea 会创建一个 authorized_principals 文件。
- `SSH_AUTHORIZED_PRINCIPALS_BACKUP`: **false/true**：在重写所有密钥时启用 SSH 授权 principal 备份，默认值为 true（如果 `SSH_AUTHORIZED_PRINCIPALS_ALLOW` 不为 `off`）。
- `SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE`: **`{{.AppPath}} --config={{.CustomConf}} serv key-{{.Key.ID}}`**：设置用于传递授权密钥的命令模板。可能的密钥是：AppPath、AppWorkPath、CustomConf、CustomPath、Key，其中 Key 是 `models/asymkey.PublicKey`，其他是 shellquoted 字符串。
- `SSH_SERVER_CIPHERS`: **chacha20-poly1305@openssh.com, aes128-ctr, aes192-ctr, aes256-ctr, aes128-gcm@openssh.com, aes256-gcm@openssh.com**：对于内置的 SSH 服务器，选择支持的 SSH 连接的加密方法，对于系统 SSH，此设置无效。
- `SSH_SERVER_KEY_EXCHANGES`: **curve25519-sha256, ecdh-sha2-nistp256, ecdh-sha2-nistp384, ecdh-sha2-nistp521, diffie-hellman-group14-sha256, diffie-hellman-group14-sha1**：对于内置 SSH 服务器，选择支持的 SSH 连接的密钥交换算法，对于系统 SSH，此设置无效。
- `SSH_SERVER_MACS`: **hmac-sha2-256-etm@openssh.com, hmac-sha2-256, hmac-sha1**：对于内置 SSH 服务器，选择支持的 SSH 连接的 MAC 算法，对于系统 SSH，此设置无效。
- `SSH_SERVER_HOST_KEYS`: **ssh/gitea.rsa, ssh/gogs.rsa**：对于内置 SSH 服务器，选择要提供为主机密钥的密钥对。私钥应在 `SSH_SERVER_HOST_KEY` 中，公钥在 `SSH_SERVER_HOST_KEY.pub` 中。相对路径会相对于 `APP_DATA_PATH` 转为绝对路径。如果不存在密钥，将为您创建一个 4096 位的 RSA 密钥。
- `SSH_KEY_TEST_PATH`: **/tmp**：在使用 `ssh-keygen` 测试公共 SSH 密钥时要在其中创建临时文件的目录，默认为系统临时目录。
- `SSH_KEYGEN_PATH`: **_empty_**：使用 `ssh-keygen` 解析公共 SSH 密钥。该值将传递给 shell。默认情况下，Gitea 会自行进行解析。
- `SSH_EXPOSE_ANONYMOUS`: **false**：启用将 SSH 克隆 URL 暴露给匿名访问者，默认为 false。
- `SSH_PER_WRITE_TIMEOUT`: **30s**：对 SSH 连接的任何写入设置超时。（将其设置为 -1 可以禁用所有超时。）
- `SSH_PER_WRITE_PER_KB_TIMEOUT`: **10s**：对写入 SSH 连接的每 KB 设置超时。
- `MINIMUM_KEY_SIZE_CHECK`: **true**：指示是否检查最小密钥大小与相应类型。
- `OFFLINE_MODE`: **true**：禁用 CDN 用于静态文件和 Gravatar 用于个人资料图片。
- `CERT_FILE`: **https/cert.pem**：用于 HTTPS 的证书文件路径。在链接时，服务器证书必须首先出现，然后是中间 CA 证书（如果有）。如果 `ENABLE_ACME=true`，则此设置会被忽略。路径相对于 `CUSTOM_PATH`。
- `KEY_FILE`: **https/key.pem**：用于 HTTPS 的密钥文件路径。如果 `ENABLE_ACME=true`，则此设置会被忽略。路径相对于 `CUSTOM_PATH`。
- `STATIC_ROOT_PATH`: **_`StaticRootPath`_**：模板和静态文件路径的上一级。
- `APP_DATA_PATH`: **data**（在 Docker 上为 **/data/gitea**）：应用程序数据的默认路径。相对路径会相对于 _`AppWorkPath`_ 转为绝对路径。
- `STATIC_CACHE_TIME`: **6h**：对 `custom/`、`public/` 和所有上传的头像的静态资源的 Web 浏览器缓存时间。请注意，在 `RUN_MODE` 为 "dev" 时，此缓存会被禁用。
- `ENABLE_GZIP`: **false**：为运行时生成的内容启用 gzip 压缩，静态资源除外。
- `ENABLE_PPROF`: **false**：应用程序分析（内存和 CPU）。对于 "web" 命令，它会在 `localhost:6060` 上监听。对于 "serv" 命令，它会将数据转储到磁盘上的 `PPROF_DATA_PATH` 中，文件名为 `(cpuprofile|memprofile)_<username>_<temporary id>`。
- `PPROF_DATA_PATH`: **_`AppWorkPath`_/data/tmp/pprof**：`PPROF_DATA_PATH`，当您将 Gitea 作为服务启动时，请使用绝对路径。
- `LANDING_PAGE`: **home**：未经身份验证用户的登录页面 \[home, explore, organizations, login, **custom**]。其中 custom 可以是任何 URL，例如 "/org/repo" 或甚至是 `https://anotherwebsite.com`。
- `LFS_START_SERVER`: **false**：启用 Git LFS 支持。
- `LFS_CONTENT_PATH`: **%(APP_DATA_PATH)s/lfs**：默认的 LFS 内容路径（如果它在本地存储中）。**已弃用**，请使用 `[lfs]` 中的设置。
- `LFS_JWT_SECRET`: **_empty_**：LFS 身份验证密钥，将其更改为唯一的字符串。
- `LFS_JWT_SECRET_URI`: **_empty_**：代替在配置中定义 LFS_JWT_SECRET，可以使用此配置选项为 Gitea 提供包含密钥的文件的路径（示例值：`file:/etc/gitea/lfs_jwt_secret`）。
- `LFS_HTTP_AUTH_EXPIRY`: **24h**：LFS 身份验证的有效期，以 time.Duration 表示，超过此期限的推送可能会失败。
- `LFS_MAX_FILE_SIZE`: **0**：允许的最大 LFS 文件大小（以字节为单位，设置为 0 为无限制）。
- `LFS_LOCKS_PAGING_NUM`: **50**：每页返回的最大 LFS 锁定数。
- `REDIRECT_OTHER_PORT`: **false**：如果为 true 并且 `PROTOCOL` 为 https，则允许将 http 请求重定向到 Gitea 监听的 https 端口的 `PORT_TO_REDIRECT`。
- `REDIRECTOR_USE_PROXY_PROTOCOL`: **%(USE_PROXY_PROTOCOL)s**：在连接到 https 重定向器时，需要 PROXY 协议头。
- `PORT_TO_REDIRECT`: **80**：http 重定向服务监听的端口。当 `REDIRECT_OTHER_PORT` 为 true 时使用。
- `SSL_MIN_VERSION`: **TLSv1.2**：设置最低支持的 SSL 版本。
- `SSL_MAX_VERSION`: **_empty_**：设置最大支持的 SSL 版本。
- `SSL_CURVE_PREFERENCES`: **X25519,P256**：设置首选的曲线。
- `SSL_CIPHER_SUITES`: **ecdhe_ecdsa_with_aes_256_gcm_sha384,ecdhe_rsa_with_aes_256_gcm_sha384,ecdhe_ecdsa_with_aes_128_gcm_sha256,ecdhe_rsa_with_aes_128_gcm_sha256,ecdhe_ecdsa_with_chacha20_poly1305,ecdhe_rsa_with_chacha20_poly1305**：设置首选的密码套件。
  - 如果没有对 AES 套件的硬件支持，默认情况下，ChaCha 套件将优先于 AES 套件。
  - 根据 Go 1.18 的支持的套件有：
    - TLS 1.0 - 1.2 套件
      - "rsa_with_rc4_128_sha"
      - "rsa_with_3des_ede_cbc_sha"
      - "rsa_with_aes_128_cbc_sha"
      - "rsa_with_aes_256_cbc_sha"
      - "rsa_with_aes_128_cbc_sha256"
      - "rsa_with_aes_128_gcm_sha256"
      - "rsa_with_aes_256_gcm_sha384"
      - "ecdhe_ecdsa_with_rc4_128_sha"
      - "ecdhe_ecdsa_with_aes_128_cbc_sha"
      - "ecdhe_ecdsa_with_aes_256_cbc_sha"
      - "ecdhe_rsa_with_rc4_128_sha"
      - "ecdhe_rsa_with_3des_ede_cbc_sha"
      - "ecdhe_rsa_with_aes_128_cbc_sha"
      - "ecdhe_rsa_with_aes_256_cbc_sha"
      - "ecdhe_ecdsa_with_aes_128_cbc_sha256"
      - "ecdhe_rsa_with_aes_128_cbc_sha256"
      - "ecdhe_rsa_with_aes_128_gcm_sha256"
      - "ecdhe_ecdsa_with_aes_128_gcm_sha256"
      - "ecdhe_rsa_with_aes_256_gcm_sha384"
      - "ecdhe_ecdsa_with_aes_256_gcm_sha384"
      - "ecdhe_rsa_with_chacha20_poly1305_sha256"
      - "ecdhe_ecdsa_with_chacha20_poly1305_sha256"
    - TLS 1.3 套件
      - "aes_128_gcm_sha256"
      - "aes_256_gcm_sha384"
      - "chacha20_poly1305_sha256"
    - 别名
      - "ecdhe_rsa_with_chacha20_poly1305" 是 "ecdhe_rsa_with_chacha20_poly1305_sha256" 的别名
      - "ecdhe_ecdsa_with_chacha20_poly1305" 是 "ecdhe_ecdsa_with_chacha20_poly1305_sha256" 的别名
- `ENABLE_ACME`: **false**：通过 ACME 能力的证书颁发机构（CA）服务器（默认为 Let's Encrypt）启用自动证书管理的标志。如果启用，将忽略 `CERT_FILE` 和 `KEY_FILE`，并且 CA 必须将 `DOMAIN` 解析为此 Gitea 服务器。确保设置了 DNS 记录，并且端口 `80` 或端口 `443` 可以被 CA 服务器访问（默认情况下是公共互联网），并重定向到相应的端口 `PORT_TO_REDIRECT` 或 `HTTP_PORT`。
- `ACME_URL`: **_empty_**：CA 的 ACME 目录 URL，例如自托管的 [smallstep CA 服务器](https://github.com/smallstep/certificates)，它可以是 `https://ca.example.com/acme/acme/directory`。如果留空，默认使用 Let's Encrypt 的生产 CA（还要检查 `LETSENCRYPT_ACCEPTTOS`）。
- `ACME_ACCEPTTOS`: **false**：这是一个明确的检查，您是否接受 ACME 提供者的服务条款。默认为 Let's Encrypt 的 [服务条款](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf)。
- `ACME_DIRECTORY`: **https**：证书管理器用于缓存证书和私钥等信息的目录。
- `ACME_EMAIL`: **_empty_**：用于 ACME 注册的电子邮件。通常用于通知有关已颁发的证书的问题。
- `ACME_CA_ROOT`: **_empty_**：CA 的根证书。如果留空，默认使用系统的信任链。
- `ALLOW_GRACEFUL_RESTARTS`: **true**：在 SIGHUP 时执行优雅重启。
- `GRACEFUL_HAMMER_TIME`: **60s**：在重新启动后，父进程将停止接受新连接，并允许请求在停止之前完成。如果耗时超过此时间，则会强制关闭关闭。
- `STARTUP_TIMEOUT`: **0**：如果启动超过提供的时间，将关闭服务器。在 Windows 上设置这将向 SVC 主机发送一个等待提示，告诉 SVC 主机启动可能需要一些时间。请注意，启动由监听器（HTTP/HTTPS/SSH）的打开来确定。索引程序可能需要更长时间启动，可能具有自己的超时时间。

## 数据库 (`database`)

- `DB_TYPE`: **mysql**：数据库类型 \[mysql, postgres, mssql, sqlite3\]。
- `HOST`: **127.0.0.1:3306**：数据库主机地址和端口或unix套接字的绝对路径 \[mysql, postgres\]（例如：/var/run/mysqld/mysqld.sock）。
- `NAME`: **gitea**：数据库名称。
- `USER`: **root**：数据库用户名。
- `PASSWD`: **_empty_**：数据库密码。如果密码包含特殊字符，请使用 \`your password\` 或 """your password"""。
- `SCHEMA`: **_empty_**：对于 PostgreSQL，如果与 "public" 不同的模式。模式必须事先存在，用户必须对其具有创建特权，并且用户搜索路径必须设置为首先查找模式（例如 `ALTER USER user SET SEARCH_PATH = schema_name,"$user",public;`）。
- `SSL_MODE`: **disable**：MySQL 或 PostgreSQL 数据库是否启用 SSL 模式，仅适用于 MySQL 和 PostgreSQL。
  - MySQL 的有效值：
    - `true`：启用 TLS，并针对数据库服务器证书根证书进行验证。选择此选项时，请确保用于验证数据库服务器证书的根证书（例如 CA 证书）位于数据库服务器和 Gitea 服务器的系统证书存储中。有关如何将 CA 证书添加到证书存储的说明，请参阅系统文档。
    - `false`：禁用 TLS。
    - `disable`：`false` 的别名，与 PostgreSQL 兼容。
    - `skip-verify`：启用 TLS，但不进行数据库服务器证书验证。如果数据库服务器上有自签名或无效证书，请使用此选项。
    - `prefer`：启用 TLS，并回退到非 TLS 连接。
  - PostgreSQL 的有效值：
    - `disable`：禁用 TLS。
    - `require`：启用 TLS，但不进行任何验证。
    - `verify-ca`：启用 TLS，并对数据库服务器证书进行根证书验证。
    - `verify-full`：启用 TLS，并验证数据库服务器名称是否与给定的证书的 "Common Name" 或 "Subject Alternative Name" 字段匹配。
- `SQLITE_TIMEOUT`：**500**：仅适用于 SQLite3 的查询超时。
- `SQLITE_JOURNAL_MODE`：**""**：更改 SQlite3 的日志模式。可以用于在高负载导致写入拥塞时启用 [WAL 模式](https://www.sqlite.org/wal.html)。有关可能的值，请参阅 [SQlite3 文档](https://www.sqlite.org/pragma.html#pragma_journal_mode)。默认为数据库文件的默认值，通常为 DELETE。
- `ITERATE_BUFFER_SIZE`：**50**：用于迭代的内部缓冲区大小。
- `PATH`：**data/gitea.db**：仅适用于 SQLite3 的数据库文件路径。
- `LOG_SQL`：**false**：记录已执行的 SQL。
- `DB_RETRIES`：**10**：允许多少次 ORM 初始化 / DB 连接尝试。
- `DB_RETRY_BACKOFF`：**3s**：如果发生故障，等待另一个 ORM 初始化 / DB 连接尝试的 time.Duration。
- `MAX_OPEN_CONNS`：**0**：数据库最大打开连接数 - 默认为 0，表示没有限制。
- `MAX_IDLE_CONNS`：**2**：连接池上的最大空闲数据库连接数，默认为 2 - 这将限制为 `MAX_OPEN_CONNS`。
- `CONN_MAX_LIFETIME`：**0 或 3s**：设置 DB 连接可以重用的最长时间 - 默认为 0，表示没有限制（除了 MySQL，其中为 3s - 请参见 #6804 和 #7071）。
- `AUTO_MIGRATION`：**true**：是否自动执行数据库模型迁移。

请参见 #8540 和 #8273 以获取有关 `MAX_OPEN_CONNS`、`MAX_IDLE_CONNS` 和 `CONN_MAX_LIFETIME` 的适当值及其与端口耗尽的关系的进一步讨论。

## 索引 (`indexer`)

- `ISSUE_INDEXER_TYPE`: **bleve**：工单索引类型，当前支持：`bleve`、`db`、`elasticsearch` 或 `meilisearch`。
- `ISSUE_INDEXER_CONN_STR`：****：工单索引连接字符串，仅适用于 elasticsearch 和 meilisearch（例如：http://elastic:password@localhost:9200）或者（例如：http://:apikey@localhost:7700）。
- `ISSUE_INDEXER_NAME`：**gitea_issues**：工单索引器名称，在 ISSUE_INDEXER_TYPE 为 elasticsearch 或 meilisearch 时可用。
- `ISSUE_INDEXER_PATH`：**indexers/issues.bleve**：用于工单搜索的索引文件；在 ISSUE_INDEXER_TYPE 为 bleve 和 elasticsearch 时可用。相对路径将相对于 _`AppWorkPath`_ 进行绝对路径化。

- `REPO_INDEXER_ENABLED`：**false**：启用代码搜索（占用大量磁盘空间，约为存储库大小的 6 倍）。
- `REPO_INDEXER_REPO_TYPES`：**sources,forks,mirrors,templates**：存储库索引器单元。要索引的项目可以是 `sources`、`forks`、`mirrors`、`templates` 或它们的任何组合，用逗号分隔。如果为空，则默认为仅 `sources`，如果要完全禁用，请参见 `REPO_INDEXER_ENABLED`。
- `REPO_INDEXER_TYPE`：**bleve**：代码搜索引擎类型，可以为 `bleve` 或者 `elasticsearch`。
- `REPO_INDEXER_PATH`：**indexers/repos.bleve**：用于代码搜索的索引文件。
- `REPO_INDEXER_CONN_STR`：****：代码索引器连接字符串，在 `REPO_INDEXER_TYPE` 为 elasticsearch 时可用。例如：http://elastic:password@localhost:9200
- `REPO_INDEXER_NAME`：**gitea_codes**：代码索引器名称，在 `REPO_INDEXER_TYPE` 为 elasticsearch 时可用。

- `REPO_INDEXER_INCLUDE`：**empty**：逗号分隔的 glob 模式列表（参见 https://github.com/gobwas/glob）以用于**包括**在索引中。使用 `**.txt` 匹配任何具有 .txt 扩展名的文件。空列表表示包括所有文件。
- `REPO_INDEXER_EXCLUDE`：**empty**：逗号分隔的 glob 模式列表（参见 https://github.com/gobwas/glob）以用于**排除**在索引中。即使在 `REPO_INDEXER_INCLUDE` 中匹配，也不会索引与此列表匹配的文件。
- `REPO_INDEXER_EXCLUDE_VENDORED`：**true**：从索引中排除 vendored 文件。
- `MAX_FILE_SIZE`：**1048576**：要索引的文件的最大字节数。
- `STARTUP_TIMEOUT`：**30s**：如果索引器启动时间超过此超时时间 - 则失败。（此超时时间将添加到上面的锤子时间中，用于子进程 - 因为 bleve 不会在上一个父进程关闭之前启动）。设置为 -1 表示永不超时。

## 队列 (`queue` and `queue.*`)

[queue] 配置在 `[queue.*]` 下为各个队列设置默认值，并允许为各个队列设置单独的配置覆盖。（不过请参见下文。）

- `TYPE`：**level**：通用队列类型，当前支持：`level`（在内部使用 LevelDB）、`channel`、`redis`、`dummy`。无效的类型将视为 `level`。
- `DATADIR`：**queues/common**：用于存储 level 队列的基本 DataDir。单独的队列的 `DATADIR` 可以在 `queue.name` 部分进行设置。相对路径将根据 `%(APP_DATA_PATH)s` 变为绝对路径。
- `LENGTH`：**100000**：通道队列阻塞之前的最大队列大小
- `BATCH_LENGTH`：**20**：在传递给处理程序之前批处理数据
- `CONN_STR`：**redis://127.0.0.1:6379/0**：redis 队列类型的连接字符串。对于 `redis-cluster`，使用 `redis+cluster://127.0.0.1:6379/0`。可以使用查询参数来设置选项。类似地，LevelDB 选项也可以使用：**leveldb://relative/path?option=value** 或 **leveldb:///absolute/path?option=value** 进行设置，并将覆盖 `DATADIR`。
- `QUEUE_NAME`：**_queue**：默认的 redis 和磁盘队列名称的后缀。单独的队列将默认为 **`name`**`QUEUE_NAME`，但可以在特定的 `queue.name` 部分中进行覆盖。
- `SET_NAME`：**_unique**：将添加到默认的 redis 和磁盘队列 `set` 名称中以用于唯一队列的后缀。单独的队列将默认为 **`name`**`QUEUE_NAME`_`SET_NAME`_，但可以在特定的 `queue.name` 部分中进行覆盖。
- `MAX_WORKERS`：**(dynamic)**：队列的最大工作协程数。默认值为 "CpuNum/2"，限制在 1 到 10 之间。

Gitea 创建以下非唯一队列：

- `code_indexer`
- `issue_indexer`
- `notification-service`
- `task`
- `mail`
- `push_update`

以及以下唯一队列：

- `repo_stats_update`
- `repo-archive`
- `mirror`
- `pr_patch_checker`

## Admin (`admin`)

- `DEFAULT_EMAIL_NOTIFICATIONS`: **enabled**：用户电子邮件通知的默认配置（用户可配置）。选项：enabled、onmention、disabled
- `DISABLE_REGULAR_ORG_CREATION`: **false**：禁止普通（非管理员）用户创建组织。
- `USER_DISABLED_FEATURES`:**_empty_** 禁用的用户特性，当前允许为空或者 `deletion`，`manage_ssh_keys`， `manage_gpg_keys` 未来可以增加更多设置。
  - `deletion`: 用户不能通过界面或者API删除他自己。
  - `manage_ssh_keys`: 用户不能通过界面或者API配置SSH Keys。
  - `manage_gpg_keys`: 用户不能配置 GPG 密钥。

## 安全性 (`security`)

- `INSTALL_LOCK`: **false**：控制是否能够访问安装向导页面，设置为 `true` 则禁止访问安装向导页面。
- `SECRET_KEY`: **\<每次安装时随机生成\>**：全局服务器安全密钥。这个密钥非常重要，如果丢失将无法解密加密的数据（例如 2FA）。
- `SECRET_KEY_URI`: **_empty_**：与定义 `SECRET_KEY` 不同，此选项可用于使用存储在文件中的密钥（示例值：`file:/etc/gitea/secret_key`）。它不应该像 `SECRET_KEY` 一样容易丢失。
- `LOGIN_REMEMBER_DAYS`: **31**：在要求重新登录之前，记住用户的登录状态多长时间（以天为单位）。
- `COOKIE_REMEMBER_NAME`: **gitea\_incredible**：保存自动登录信息的 Cookie 名称。
- `REVERSE_PROXY_AUTHENTICATION_USER`: **X-WEBAUTH-USER**：反向代理认证的 HTTP 头部名称，用于提供用户信息。
- `REVERSE_PROXY_AUTHENTICATION_EMAIL`: **X-WEBAUTH-EMAIL**：反向代理认证的 HTTP 头部名称，用于提供邮箱信息。
- `REVERSE_PROXY_AUTHENTICATION_FULL_NAME`: **X-WEBAUTH-FULLNAME**：反向代理认证的 HTTP 头部名称，用于提供全名信息。
- `REVERSE_PROXY_LIMIT`: **1**：解释 X-Forwarded-For 标头或 X-Real-IP 标头，并将其设置为请求的远程 IP。
   可信代理计数。设置为零以不使用这些标头。
- `REVERSE_PROXY_TRUSTED_PROXIES`: **127.0.0.0/8,::1/128**：逗号分隔的受信任代理服务器的 IP 地址和网络列表。使用 `*` 来信任全部。
- `DISABLE_GIT_HOOKS`: **true**：设置为 `false` 以允许具有 Git 钩子权限的用户创建自定义 Git 钩子。
   警告：自定义 Git 钩子可用于在主机操作系统上执行任意代码。这允许用户访问和修改此配置文件和 Gitea 数据库，并中断 Gitea 服务。
   通过修改 Gitea 数据库，用户可以获得 Gitea 管理员权限。
   它还使他们可以访问正在运行 Gitea 实例的操作系统上用户可用的其他资源，并以 Gitea 操作系统用户的名义执行任意操作。
   这可能对您的网站或操作系统造成危害。
   在必要之前，请在更改现有 git 存储库中的钩子之前进行调整。
- `DISABLE_WEBHOOKS`: **false**：设置为 `true` 以禁用 Webhooks 功能。
- `ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET`: **true**：设置为 `false` 以允许本地用户在未设置 Gitea 环境的情况下推送到 Gitea 存储库。不建议这样做，如果您希望本地用户推送到 Gitea 存储库，应该适当地设置环境。
- `IMPORT_LOCAL_PATHS`: **false**：设置为 `false` 以防止所有用户（包括管理员）从服务器上导入本地路径。
- `INTERNAL_TOKEN`: **\<每次安装时随机生成，如果未设置 URI\>**：用于验证 Gitea 二进制文件内部通信的密钥。
- `INTERNAL_TOKEN_URI`: **_empty_**：与在配置中定义 `INTERNAL_TOKEN` 不同，此配置选项可用于将包含内部令牌的文件的路径提供给 Gitea（示例值：`file:/etc/gitea/internal_token`）。
- `PASSWORD_HASH_ALGO`: **pbkdf2**：要使用的哈希算法 \[argon2、pbkdf2、pbkdf2_v1、pbkdf2_hi、scrypt、bcrypt\]，argon2 和 scrypt 将消耗大量内存。
  - 注意：`pbkdf2` 哈希的默认参数已更改 - 先前的设置可作为 `pbkdf2_v1` 使用，但不建议使用。
  - 可以通过在算法后使用 `$` 进行调整：
    - `argon2$<time>$<memory>$<threads>$<key-length>`
    - `bcrypt$<cost>`
    - `pbkdf2$<iterations>$<key-length>`
    - `scrypt$<n>$<r>$<p>$<key-length>`
  - 默认值为：
    - `argon2`：`argon2$2$65536$8$50`
    - `bcrypt`：`bcrypt$10`
    - `pbkdf2`：`pbkdf2$50000$50`
    - `pbkdf2_v1`：`pbkdf2$10000$50`
    - `pbkdf2_v2`：`pbkdf2$50000$50`
    - `pbkdf2_hi`：`pbkdf2$320000$50`
    - `scrypt`：`scrypt$65536$16$2$50`
  - 使用此功能调整算法参数存在一定风险。
- `CSRF_COOKIE_HTTP_ONLY`: **true**：设置为 false 以允许 JavaScript 读取 CSRF cookie。
- `MIN_PASSWORD_LENGTH`: **6**：新用户的最小密码长度。
- `PASSWORD_COMPLEXITY`: **off**：要求通过最小复杂性的字符类别的逗号分隔列表。如果留空或没有指定有效值，则禁用检查（off）：
  - lower - 使用一个或多个小写拉丁字符
  - upper - 使用一个或多个大写拉丁字符
  - digit - 使用一个或多个数字
  - spec - 使用一个或多个特殊字符，如 ``!"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~``
  - off - 不检查密码复杂性
- `PASSWORD_CHECK_PWN`: **false**：检查密码是否在 [HaveIBeenPwned](https://haveibeenpwned.com/Passwords) 中曝光。
- `SUCCESSFUL_TOKENS_CACHE_SIZE`: **20**：缓存成功的令牌哈希。API 令牌在数据库中存储为 pbkdf2 哈希，但这意味着在存在多个 API 操作时可能会有显着的哈希负载。此缓存将在 LRU 缓存中存储成功的哈希令牌，以在性能和安全性之间保持平衡。

## Camo (`camo`)

- `ENABLED`: **false**：启用媒体代理，目前仅支持图像。
- `SERVER_URL`: **_empty_**：Camo服务器的URL，如果启用camo，则**必填**。
- `HMAC_KEY`: **_empty_**：为URL编码提供HMAC密钥，如果启用camo，则**必填**。
- `ALLWAYS`: **false**：设置为true以在HTTP和HTTPS内容上使用camo，否则仅代理非HTTPS URL。

## OpenID (`openid`)

- `ENABLE_OPENID_SIGNIN`: **true**：允许通过OpenID进行身份验证。
- `ENABLE_OPENID_SIGNUP`: **! DISABLE\_REGISTRATION**：允许通过OpenID进行注册。
- `WHITELISTED_URIS`: **_empty_**：如果非空，是一组匹配OpenID URI的POSIX正则表达式模式，用于允许访问。
- `BLACKLISTED_URIS`: **_empty_**：如果非空，是一组匹配OpenID URI的POSIX正则表达式模式，用于阻止访问。

## OAuth2 Client (`oauth2_client`)

- `REGISTER_EMAIL_CONFIRM`: _[service]_ **REGISTER\_EMAIL\_CONFIRM**：设置此项以启用或禁用OAuth2自动注册的电子邮件确认。（覆盖`[service]`部分的`REGISTER\_EMAIL\_CONFIRM`设置）
- `OPENID_CONNECT_SCOPES`: **_empty_**：附加的OpenID连接范围的列表。（`openid`已隐式添加）
- `ENABLE_AUTO_REGISTRATION`: **false**：为新的OAuth2用户自动创建用户帐户。
- `USERNAME`: **nickname**：新OAuth2帐户的用户名来源：
  - userid - 使用userid / sub属性
  - nickname - 使用nickname属性
  - email - 使用email属性的用户名部分
- `UPDATE_AVATAR`: **false**：如果OAuth2提供程序中有可用的头像，则进行头像更新。更新将在每次登录时执行。
- `ACCOUNT_LINKING`: **login**：如果帐户/电子邮件已存在，如何处理：
  - disabled - 显示错误
  - login - 显示帐户链接登录
  - auto - 自动与帐户链接（请注意，这将因为提供相同的用户名或电子邮件而自动授予现有帐户的访问权限。您必须确保这不会导致身份验证提供程序出现问题。）

## Service (`service`)

- `ACTIVE_CODE_LIVE_MINUTES`: **180**：确认帐户/电子邮件注册的时间限制（分钟）。
- `RESET_PASSWD_CODE_LIVE_MINUTES`: **180**：确认忘记密码重置流程的时间限制（分钟）。
- `REGISTER_EMAIL_CONFIRM`: **false**：启用此项以要求通过邮件确认注册。需要启用`Mailer`。
- `REGISTER_MANUAL_CONFIRM`: **false**：启用此项以手动确认新的注册。需要禁用`REGISTER_EMAIL_CONFIRM`。
- `DISABLE_REGISTRATION`: **false**：禁用注册，之后只有管理员可以为用户创建帐户。
- `REQUIRE_EXTERNAL_REGISTRATION_PASSWORD`: **false**：启用此项以强制通过外部方式创建的帐户（通过GitHub、OpenID Connect等）创建密码。警告：启用此项将降低安全性，因此只有在您知道自己在做什么时才应启用它。
- `REQUIRE_SIGNIN_VIEW`: **false**：启用此项以强制用户登录以查看任何页面或使用API。
- `ENABLE_NOTIFY_MAIL`: **false**：启用此项以在发生某些情况（如创建问题）时向存储库的观察者发送电子邮件。需要启用`Mailer`。
- `ENABLE_BASIC_AUTHENTICATION`: **true**：禁用此项以禁止使用HTTP BASIC和用户的密码进行身份验证。请注意，如果禁用此项，您将无法使用密码访问令牌API端点。此外，这仅会禁用使用密码的BASIC身份验证，而不会禁用令牌或OAuth Basic。
- `ENABLE_REVERSE_PROXY_AUTHENTICATION`: **false**：启用此项以允许反向代理身份验证。
- `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION`: **false**：启用此项以允许反向身份验证的自动注册。
- `ENABLE_REVERSE_PROXY_EMAIL`: **false**：启用此项以允许使用提供的电子邮件而不是生成的电子邮件进行自动注册。
- `ENABLE_REVERSE_PROXY_FULL_NAME`: **false**：启用此项以允许使用提供的全名进行自动注册。
- `ENABLE_CAPTCHA`: **false**：启用此项以对注册使用验证码验证。
- `REQUIRE_CAPTCHA_FOR_LOGIN`: **false**：启用此项以要求登录使用验证码验证。您还必须启用`ENABLE_CAPTCHA`。
- `REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA`: **false**：启用此项以强制对外部帐户（即GitHub、OpenID Connect等）使用验证码验证。您还必须启用`ENABLE_CAPTCHA`。
- `CAPTCHA_TYPE`: **image**：\[image、recaptcha、hcaptcha、mcaptcha、cfturnstile\]
- `RECAPTCHA_SECRET`: **""**：访问https://www.google.com/recaptcha/admin以获取recaptcha的密钥。
- `RECAPTCHA_SITEKEY`: **""**：访问https://www.google.com/recaptcha/admin以获取recaptcha的站点密钥。
- `RECAPTCHA_URL`: **https://www.google.com/recaptcha/**：设置recaptcha网址，允许使用recaptcha net。
- `HCAPTCHA_SECRET`: **""**：注册https://www.hcaptcha.com/以获取hcaptcha的密钥。
- `HCAPTCHA_SITEKEY`: **""**：注册https://www.hcaptcha.com/以获取hcaptcha的站点密钥。
- `MCAPTCHA_SECRET`: **""**：访问您的mCaptcha实例以获取mCaptcha的密钥。
- `MCAPTCHA_SITEKEY`: **""**：访问您的mCaptcha实例以获取mCaptcha的站点密钥。
- `MCAPTCHA_URL` **https://demo.mcaptcha.org/**：设置mCaptcha的URL。
- `CF_TURNSTILE_SECRET` **""**：访问https://dash.cloudflare.com/?to=/:account/turnstile以获取cloudflare turnstile的密钥。
- `CF_TURNSTILE_SITEKEY` **""**：访问https://dash.cloudflare.com/?to=/:account/turnstile以获取cloudflare turnstile的站点密钥。
- `DEFAULT_KEEP_EMAIL_PRIVATE`: **false**：默认情况下，将用户设置为保持其电子邮件地址私有。
- `DEFAULT_ALLOW_CREATE_ORGANIZATION`: **true**：默认情况下，允许新用户创建组织。
- `DEFAULT_USER_IS_RESTRICTED`: **false**：默认情况下，为新用户分配受限权限。
- `DEFAULT_ENABLE_DEPENDENCIES`: **true**：启用此项以默认启用依赖项。
- `ALLOW_CROSS_REPOSITORY_DEPENDENCIES` : **true** 启用此项以允许从用户被授予访问权限的任何存储库上进行依赖项操作。
- `USER_LOCATION_MAP_URL`: **""**：一个显示用户在地图上位置的地图服务URL。位置将作为转义的查询参数附加到URL中。
- `ENABLE_USER_HEATMAP`: **true**：启用此项以在用户个人资料上显示热图。
- `ENABLE_TIMETRACKING`: **true**：启用时间跟踪功能。
- `DEFAULT_ENABLE_TIMETRACKING`: **true**：默认情况下，允许存储库默认使用时间跟踪。
- `DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME`: **true**：仅允许具有写权限的用户跟踪时间。
- `EMAIL_DOMAIN_ALLOWLIST`: **_empty_**：如果非空，逗号分隔的域名列表，只能用于在此实例上注册，支持通配符。
- `EMAIL_DOMAIN_BLOCKLIST`: **_empty_**：如果非空，逗号分隔的域名列表，不能用于在此实例上注册，支持通配符。
- `SHOW_REGISTRATION_BUTTON`: **! DISABLE\_REGISTRATION**：显示注册按钮
- `SHOW_MILESTONES_DASHBOARD_PAGE`: **true** 启用此项以显示里程碑仪表板页面 - 查看所有用户的里程碑
- `AUTO_WATCH_NEW_REPOS`: **true** 启用此项以在创建新存储库时让所有组织用户观看新存储库
- `AUTO_WATCH_ON_CHANGES`: **false** 启用此项以在首次提交后使用户观看存储库
- `DEFAULT_USER_VISIBILITY`: **public**：为用户设置默认的可见性模式，可以是"public"、"limited"或"private"。
- `ALLOWED_USER_VISIBILITY_MODES`: **public,limited,private**：设置用户可以具有的可见性模式
- `DEFAULT_ORG_VISIBILITY`: **public**：为组织设置默认的可见性模式，可以是"public"、"limited"或"private"。
- `DEFAULT_ORG_MEMBER_VISIBLE`: **false**：如果添加到组织时将用户的成员身份可见，设置为True。
- `ALLOW_ONLY_INTERNAL_REGISTRATION`: **false**：设置为True以强制仅通过Gitea进行注册。
- `ALLOW_ONLY_EXTERNAL_REGISTRATION`: **false**：设置为True以强制仅使用第三方服务进行注册。
- `NO_REPLY_ADDRESS`: **noreply.DOMAIN**：如果用户将KeepEmailPrivate设置为True，则在Git日志中的用户电子邮件地址的域部分的值。DOMAIN解析为server.DOMAIN中的值。
  用户的电子邮件将被替换为小写的用户名、"@"和NO_REPLY_ADDRESS的连接。
- `USER_DELETE_WITH_COMMENTS_MAX_TIME`: **0**：用户删除后，评论将保留的最短时间。
- `VALID_SITE_URL_SCHEMES`: **http, https**：用户个人资料的有效站点URL方案

### Service - Explore (`service.explore`)

- `REQUIRE_SIGNIN_VIEW`: **false**：仅允许已登录的用户查看探索页面。
- `DISABLE_USERS_PAGE`: **false**：禁用用户探索页面。

## SSH Minimum Key Sizes (`ssh.minimum_key_sizes`)

定义允许的算法及其最小密钥长度（使用-1来禁用某个类型）：

- `ED25519`：**256**
- `ECDSA`：**256**
- `RSA`：**3071**：我们在这里设置为2047，因为一个其他方面有效的3072 RSA密钥可能被报告为3071长度。
- `DSA`：**-1**：默认情况下禁用DSA。设置为**1024**以重新启用，但请注意可能需要重新配置您的SSHD提供者

## Webhook (`webhook`)

- `QUEUE_LENGTH`: **1000**：钩子任务队列长度。编辑此值时要小心。
- `DELIVER_TIMEOUT`: **5**：发送 Webhook 的交付超时时间（秒）。
- `ALLOWED_HOST_LIST`: **external**：出于安全原因，Webhook 仅能调用允许的主机。以逗号分隔的列表。
  - 内置网络：
    - `loopback`：IPv4 的 127.0.0.0/8 和 IPv6 的 ::1/128，包括 localhost。
    - `private`：RFC 1918（10.0.0.0/8，172.16.0.0/12，192.168.0.0/16）和 RFC 4193（FC00::/7）。也称为 LAN/Intranet。
    - `external`：一个有效的非私有单播 IP，您可以访问公共互联网上的所有主机。
    - `*`：允许所有主机。
  - CIDR 列表：IPv4 的 `1.2.3.0/8` 和 IPv6 的 `2001:db8::/32`
  - 通配符主机：`*.mydomain.com`，`192.168.100.*`
- `SKIP_TLS_VERIFY`: **false**：允许不安全的证书。
- `PAGING_NUM`: **10**：一页中显示的 Webhook 历史事件数量。
- `PROXY_URL`: **_empty_**：代理服务器 URL，支持 http://、https://、socks://，留空将遵循环境的 http_proxy/https_proxy 设置。如果未提供，将使用全局代理设置。
- `PROXY_HOSTS`: **_empty_**：需要代理的主机名的逗号分隔列表。支持通配符模式 (*)；使用 ** 来匹配所有主机。如果未提供，将使用全局代理设置。

## 邮件 (`mailer`)

⚠️ 此部分适用于 Gitea 1.18 及更高版本。如果您使用的是 Gitea 1.17 或更早版本，请阅读以下链接获取更多信息：
[Gitea 1.17 app.ini 示例](https://github.com/go-gitea/gitea/blob/release/v1.17/custom/conf/app.example.ini)
和
[Gitea 1.17 配置文档](https://github.com/go-gitea/gitea/blob/release/v1.17/docs/content/doc/advanced/config-cheat-sheet.en-us.md)

- `ENABLED`: **false**：是否启用邮件服务。
- `PROTOCOL`: **_empty_**：邮件服务协议，可选择 "smtp"、"smtps"、"smtp+starttls"、"smtp+unix"、"sendmail"、"dummy"。在 Gitea 1.18 之前，邮件服务协议由 `MAILER_TYPE` 和 `IS_TLS_ENABLED` 两个配置共同决定。
  - SMTP 类族，如果您的提供者没有明确说明使用的是哪个协议，但提供了一个端口，您可以设置 SMTP_PORT，它将被推断出来。
  - **sendmail** 使用操作系统的 `sendmail` 命令，而不是 SMTP。这在 Linux 系统上很常见。
  - **dummy** 将邮件消息发送到日志，作为测试阶段。
  - 请注意，启用 sendmail 将忽略所有其他 `mailer` 设置，除了 `ENABLED`、`FROM`、`SUBJECT_PREFIX` 和 `SENDMAIL_PATH`。
  - 启用 dummy 将忽略所有设置，除了 `ENABLED`、`SUBJECT_PREFIX` 和 `FROM`。
- `SMTP_ADDR`: **_empty_**：邮件服务器地址，例如 smtp.gmail.com。对于 smtp+unix，这应该是一个到 unix socket 的路径。在 1.18 之前，此设置与 `SMTP_PORT` 合并，名称为 `HOST`。
- `SMTP_PORT`: **_empty_**：邮件服务器端口。如果未指定协议，将通过此设置进行推断。常用端口如下。在 1.18 之前，此设置与 `SMTP_ADDR` 合并，名称为 `HOST`。
  - 25：不安全的简单邮件传输协议（insecure SMTP）
  - 465：安全的简单邮件传输协议（SMTP Secure）
  - 587：StartTLS
- `USE_CLIENT_CERT`: **false**：使用客户端证书进行 TLS/SSL 加密。
- `CLIENT_CERT_FILE`: **custom/mailer/cert.pem**：客户端证书文件。
- `CLIENT_KEY_FILE`: **custom/mailer/key.pem**：客户端密钥文件。
- `FORCE_TRUST_SERVER_CERT`: **false**：如果设置为 `true`，将完全忽略服务器证书验证错误。此选项不安全。考虑将证书添加到系统信任存储中。
- `USER`: **_empty_**：邮件用户的用户名（通常是发件人的电子邮件地址）。
- `PASSWD`: **_empty_**：邮件用户的密码。如果密码中使用了特殊字符，请使用 \`your password\` 进行引用。
  - 请注意：只有在 SMTP 服务器通信通过 TLS 加密（可以通过 `STARTTLS` 实现）或 SMTP 主机是 localhost 时，才支持身份验证。有关更多信息，请参阅 [邮件设置](administration/email-setup.md)。
- `ENABLE_HELO`: **true**：启用 HELO 操作。
- `HELO_HOSTNAME`: **（从系统检索）**：HELO 主机名。
- `FROM`: **_empty_**：邮件的发件人地址，符合 RFC 5322。这可以是一个电子邮件地址，也可以是 "Name" \<email@example.com\> 格式。
- `ENVELOPE_FROM`: **_empty_**：在 SMTP 邮件信封上设置的地址作为发件地址。设置为 `<>` 以发送一个空地址。
- `SUBJECT_PREFIX`: **_empty_**：放置在电子邮件主题行之前的前缀。
- `SENDMAIL_PATH`: **sendmail**：操作系统上 `sendmail` 的位置（可以是命令或完整路径）。
- `SENDMAIL_ARGS`: **_empty_**：指定任何额外的 sendmail 参数。（注意：您应该知道电子邮件地址可能看起来像选项 - 如果您的 `sendmail` 命令带有选项，您必须设置选项终止符 `--`）
- `SENDMAIL_TIMEOUT`: **5m**：通过 sendmail 发送电子邮件的默认超时时间。
- `SENDMAIL_CONVERT_CRLF`: **true**：大多数版本的 sendmail 偏好使用 LF 换行符，而不是 CRLF 换行符。如果您的 sendmail 版本需要 CRLF 换行符，请将此设置为 false。
- `SEND_BUFFER_LEN`: **100**：邮件队列的缓冲区长度。**已弃用**，请在 `[queue.mailer]` 中使用 `LENGTH`。
- `SEND_AS_PLAIN_TEXT`: **false**：仅以纯文本形式发送邮件，不包括 HTML 备选方案。

## 入站邮件 (`email.incoming`)

- `ENABLED`: **false**：启用处理入站邮件。
- `REPLY_TO_ADDRESS`: **_empty_**：包括 `%{token}` 占位符的电子邮件地址，该占位符将根据用户/操作进行替换。示例：`incoming+%{token}@example.com`。占位符必须出现在地址的用户部分（在 `@` 之前）。
- `HOST`: **_empty_**：IMAP 服务器主机。
- `PORT`: **_empty_**：IMAP 服务器端口。
- `USERNAME`: **_empty_**：接收帐户的用户名。
- `PASSWORD`: **_empty_**：接收帐户的密码。
- `USE_TLS`: **false**：IMAP 服务器是否使用 TLS。
- `SKIP_TLS_VERIFY`: **false**：如果设置为 `true`，将完全忽略服务器证书验证错误。此选项不安全。
- `MAILBOX`: **INBOX**：入站邮件将到达的邮箱名称。
- `DELETE_HANDLED_MESSAGE`: **true**：是否应从邮箱中删除已处理的消息。
- `MAXIMUM_MESSAGE_SIZE`: **10485760**：要处理的消息的最大大小。忽略更大的消息。将其设置为 0 以允许每种大小。

## 缓存 (`cache`)

- `ADAPTER`: **memory**: 缓存引擎，可以为 `memory`, `redis`, `redis-cluster`, `twoqueue` 和 `memcache`. (`twoqueue` 代表缓冲区固定的LRU缓存)
- `INTERVAL`: **60**: 垃圾回收间隔(秒)，只对`memory`和`towqueue`有效。
- `HOST`: **_empty_**: 缓存配置。`redis`, `redis-cluster`，`memcache`配置连接字符串;`twoqueue` 设置队列参数
  - Redis: `redis://:macaron@127.0.0.1:6379/0?pool_size=100&idle_timeout=180s`
  - Redis-cluster `redis+cluster://:macaron@127.0.0.1:6379/0?pool_size=100&idle_timeout=180s`
  - Memcache: `127.0.0.1:9090;127.0.0.1:9091`
  - TwoQueue LRU cache: `{"size":50000,"recent_ratio":0.25,"ghost_ratio":0.5}` 或者 `50000`，代表缓冲区的缓存对象容量
- `ITEM_TTL`: **16h**: 缓存项目失效时间，设置为 -1 则禁用缓存

### 缓存 - 最后提交缓存设置 (`cache.last_commit`)

- `ITEM_TTL`: **8760h**：如果未使用，保持缓存中的项目的时间，将其设置为 -1 会禁用缓存。
- `COMMITS_COUNT`: **1000**：仅在存储库的提交计数大于时启用缓存。

## 会话 (`session`)

- `PROVIDER`: **memory**：会话存储引擎 \[memory, file, redis, redis-cluster, db, mysql, couchbase, memcache, postgres\]。设置为 `db` 将会重用 `[database]` 的配置信息。
- `PROVIDER_CONFIG`: **data/sessions**：对于文件，为根路径；对于 db，为空（将使用数据库配置）；对于其他引擎，为连接字符串。相对路径将根据 _`AppWorkPath`_ 绝对化。
- `COOKIE_SECURE`: **_empty_**：`true` 或 `false`。启用此选项以强制在所有会话访问中使用 HTTPS。如果没有设置，当 ROOT_URL 是 https 链接的时候默认设置为 true。
- `COOKIE_NAME`: **i\_like\_gitea**：用于会话 ID 的 cookie 名称。
- `GC_INTERVAL_TIME`: **86400**：GC 间隔时间，以秒为单位。
- `SESSION_LIFE_TIME`: **86400**：会话生命周期，以秒为单位，默认为 86400（1 天）。
- `DOMAIN`: **_empty_**：设置 cookie 的域。
- `SAME_SITE`: **lax** \[strict, lax, none\]：为 cookie 设置 SameSite 属性。

## 图像 (`picture`)

- `GRAVATAR_SOURCE`: **gravatar**：头像来源，可以是 gravatar、duoshuo 或类似 http://cn.gravatar.com/avatar/ 的来源。
   `http://cn.gravatar.com/avatar/`。
- `DISABLE_GRAVATAR`: **false**：启用后，只使用内部头像。**已弃用 [v1.18+]** 该配置已迁移到数据库中保存，通过管理员面板进行配置。
- `ENABLE_FEDERATED_AVATAR`: **false**：启用头像联盟支持（参见
   [http://www.libravatar.org](http://www.libravatar.org)）。**已弃用 [v1.18+]** 该配置已迁移到数据库中保存，通过管理员面板进行配置。

- `AVATAR_STORAGE_TYPE`: **default**：在 `[storage.xxx]` 中定义的存储类型。默认为 `default`，如果没有 `[storage]` 部分，则将读取 `[storage]`，如果没有则将是 `local` 类型。
- `AVATAR_UPLOAD_PATH`: **data/avatars**：存储用户头像图像文件的路径。
- `AVATAR_MAX_WIDTH`: **4096**：头像的最大宽度，以像素为单位。
- `AVATAR_MAX_HEIGHT`: **4096**：头像的最大高度，以像素为单位。
- `AVATAR_MAX_FILE_SIZE`: **1048576**（1MiB）：头像的最大大小。
- `AVATAR_MAX_ORIGIN_SIZE`: **262144**（256KiB）：如果上传的文件不大于此字节大小，则图像将原样使用，无需调整大小/转换。
- `AVATAR_RENDERED_SIZE_FACTOR`: **2**：渲染的头像图像的乘法因子。较大的值在 HiDPI 设备上会产生更细腻的渲染。

- `REPOSITORY_AVATAR_STORAGE_TYPE`: **default**：在 `[storage.xxx]` 中定义的存储类型。默认为 `default`，如果没有 `[storage]` 部分，则将读取 `[storage]`，如果没有则将是 `local` 类型。
- `REPOSITORY_AVATAR_UPLOAD_PATH`: **data/repo-avatars**：存储仓库头像图像文件的路径。
- `REPOSITORY_AVATAR_FALLBACK`: **none**：Gitea 处理缺少仓库头像的方式
  - none = 不显示任何头像
  - random = 生成随机头像
  - image = 使用默认图像（在 `REPOSITORY_AVATAR_FALLBACK_IMAGE` 中设置），如果设置为 image 并且未上传任何图像。
- `REPOSITORY_AVATAR_FALLBACK_IMAGE`: **/img/repo_default.png**：作为默认仓库头像的图像（如果将 `REPOSITORY_AVATAR_FALLBACK` 设置为 image 并且没有上传图像）。

## 项目 (`project`)

默认项目看板的模板：

- `PROJECT_BOARD_BASIC_KANBAN_TYPE`: **待办，进行中，已完成**
- `PROJECT_BOARD_BUG_TRIAGE_TYPE`: **待分析，高优先级，低优先级，已关闭**

## 工单和合并请求的附件 (`attachment`)

- `ENABLED`: **true**: 是否允许用户上传附件。
- `ALLOWED_TYPES`: **.cpuprofile,.csv,.dmp,.docx,.fodg,.fodp,.fods,.fodt,.gif,.gz,.jpeg,.jpg,.json,.jsonc,.log,.md,.mov,.mp4,.odf,.odg,.odp,.ods,.odt,.patch,.pdf,.png,.pptx,.svg,.tgz,.txt,.webm,.xls,.xlsx,.zip**: 允许的文件扩展名（`.zip`）、mime 类型（`text/plain`）或通配符类型（`image/*`、`audio/*`、`video/*`）的逗号分隔列表。空值或 `*/*` 允许所有类型。
- `MAX_SIZE`: **2048**: 附件的最大限制（MB）。
- `MAX_FILES`: **5**: 一次最多上传的附件数量。
- `STORAGE_TYPE`: **local**: 附件的存储类型，`local` 表示本地磁盘，`minio` 表示兼容 S3 的对象存储服务，如果未设置将使用默认值 `local` 或其他在 `[storage.xxx]` 中定义的名称。
- `SERVE_DIRECT`: **false**: 允许存储驱动器重定向到经过身份验证的 URL 以直接提供文件。目前，只支持 Minio/S3 通过签名 URL 提供支持，local 不会执行任何操作。
- `PATH`: **attachments**: 存储附件的路径，仅当 STORAGE_TYPE 为 `local` 时可用。如果是相对路径，将会被解析为 `${AppDataPath}/${attachment.PATH}`.
- `MINIO_ENDPOINT`: **localhost:9000**: Minio 端点以连接，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID 以连接，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey 以连接，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_BUCKET`: **gitea**: Minio 存储附件的存储桶，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_LOCATION`: **us-east-1**: Minio 存储桶的位置以创建，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_BASE_PATH`: **attachments/**: Minio 存储桶上的基本路径，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_USE_SSL`: **false**: Minio 启用 SSL，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_INSECURE_SKIP_VERIFY`: **false**: Minio 跳过 SSL 验证，仅当 STORAGE_TYPE 为 `minio` 时可用。
- `MINIO_CHECKSUM_ALGORITHM`: **default**: Minio 校验算法：`default`（适用于 MinIO 或 AWS S3）或 `md5`（适用于 Cloudflare 或 Backblaze）
- `MINIO_BUCKET_LOOKUP_TYPE`: **auto**: Minio的bucket查找方式默认为`auto`模式，可将其设置为`dns`（虚拟托管样式）或`path`（路径样式），仅当`STORAGE_TYPE`为`minio`时可用。

## 日志 (`log`)

- `ROOT_PATH`: **_empty_**: 日志文件的根目录。
- `MODE`: **console**: 日志模式。对于多个模式，请使用逗号分隔。您可以在每个模式的日志子部分中配置每个模式。 `\[log.writer-mode-name\]`.
- `LEVEL`: **Info**: 日志级别。可选值：\[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `STACKTRACE_LEVEL`: **None**: 记录创建堆栈跟踪的默认日志级别（很少有用，不要设置它）。可选值：\[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `ENABLE_SSH_LOG`: **false**: 将 SSH 日志保存到日志文件中。
- `logger.access.MODE`: **_empty_**: "access" 记录器
- `logger.router.MODE`: **,**: "router" 记录器，单个逗号表示它将使用上述默认 MODE
- `logger.xorm.MODE`: **,**: "xorm" 记录器

### 访问日志 (`log`)

- `ACCESS_LOG_TEMPLATE`: **`{{.Ctx.RemoteHost}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}" "{{.Ctx.Req.UserAgent}}"`**: 设置用于创建访问日志的模板。
  - 可用以下变量：
  - `Ctx`: 请求的 `context.Context`。
  - `Identity`: 登录的 SignedUserName 或 `"-"`（如果未登录）。
  - `Start`: 请求的开始时间。
  - `ResponseWriter`: 请求的 responseWriter。
  - `RequestID`: 与 REQUEST_ID_HEADERS 相匹配的值（默认：如果不匹配则为 `-`）。
  - 您必须非常小心，确保此模板不会引发错误或 panic，因为此模板在 panic/recovery 脚本之外运行。
- `REQUEST_ID_HEADERS`: **_empty_**: 您可以在这里配置由逗号分隔的多个值。它将按照配置的顺序进行匹配，最终将在访问日志中打印第一个匹配的值。
  - 例如：
  - 在请求头中：X-Request-ID: **test-id-123**
  - 在 app.ini 中的配置：REQUEST_ID_HEADERS = X-Request-ID
  - 在日志中打印：127.0.0.1:58384 - - [14/Feb/2023:16:33:51 +0800]  "**test-id-123**" ...

### 日志子部分 (`log.<writer-mode-name>`)

- `MODE`: **name**: 设置此日志记录器的模式 - 默认为提供的子部分名称。这允许您在不同级别上具有两个不同的文件日志记录器。
- `LEVEL`: **log.LEVEL**: 设置此日志记录器的日志级别。默认为全局 `[log]` 部分中设置的 `LEVEL`。
- `STACKTRACE_LEVEL`: **log.STACKTRACE_LEVEL**: 设置记录堆栈跟踪的日志级别。
- `EXPRESSION`: **""**: 用于匹配函数名称、文件或消息的正则表达式。默认为空。只有匹配表达式的日志消息才会保存在记录器中。
- `FLAGS`: **stdflags**: 逗号分隔的字符串，表示日志标志。默认为 `stdflags`，表示前缀：`2009/01/23 01:23:23 ...a/b/c/d.go:23:runtime.Caller() [I]: message`。`none` 表示不要在日志行前缀中添加任何内容。有关更多信息，请参见 `modules/log/flags.go`。
- `PREFIX`: **""**: 该记录器中每个日志行的附加前缀。默认为空。
- `COLORIZE`: **false**: 是否为日志行添加颜色

### 控制台日志模式 (`log.console` 或 `MODE=console`)

- 对于控制台记录器，如果不在 Windows 上或终端被确定为能够着色，则 `COLORIZE` 默认为 `true`。
- `STDERR`: **false**: 使用 Stderr 而不是 Stdout。

### 文件日志模式 (`log.file` 或 `MODE=file`)

- `FILE_NAME`: 设置此记录器的文件名。默认为 `gitea.log`（例外：访问日志默认为 `access.log`）。如果是相对路径，将相对于 `ROOT_PATH`。
- `LOG_ROTATE`: **true**: 旋转日志文件。
- `MAX_SIZE_SHIFT`: **28**: 单个文件的最大大小移位，28 表示 256Mb。
- `DAILY_ROTATE`: **true**: 每天旋转日志。
- `MAX_DAYS`: **7**: 在 n 天后删除日志文件
- `COMPRESS`: **true**: 默认使用 gzip 压缩旧的日志文件
- `COMPRESSION_LEVEL`: **-1**: 压缩级别

### 连接日志模式 (`log.conn` 或 `MODE=conn`)

- `RECONNECT_ON_MSG`: **false**: 对每个单独的消息重新连接主机。
- `RECONNECT`: **false**: 当连接丢失时尝试重新连接。
- `PROTOCOL`: **tcp**: 设置协议，可以是 "tcp"、"unix" 或 "udp"。
- `ADDR`: **:7020**: 设置要连接到的地址。

## 定时任务 (`cron`)

- `ENABLED`: **false**:  是否在后台运行定期任务。
- `RUN_AT_START`: **false**: 在应用程序启动时运行定时任务。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true时，任务成功完成时将进行通知。

- `SCHEDULE` 接受的格式:
  - 完整的crontab语法规范, e.g. `* * * * * ?`
  - 描述符e.g. `@midnight`, `@every 1h30m` ...
  - 更多详见: [cron documentation](https://pkg.go.dev/github.com/gogs/cron@v0.0.0-20171120032916-9f6c956d3e14)

### 基本定时任务 - 默认开启

#### 定时任务 - 删除旧的仓库存档 (`cron.archive_cleanup`)

- `ENABLED`: **true**: 是否启用该定时任务。
- `RUN_AT_START`: **true**: 设置在服务启动时运行。
- `SCHEDULE`: **@midnight**: 使用Cron语法的定时任务触发配置，例如 `@every 1h`。
- `OLDER_THAN`: **24h**: 超过`OLDER_THAN`时间的存档将被删除，例如 `12h`。

#### 定时任务 - 更新镜像仓库 (`cron.update_mirrors`)

- `SCHEDULE`: **@every 10m**: 使用Cron语法的定时任务触发配置，例如 `@every 3h`。
- `PULL_LIMIT`: **50**: 将要添加到队列的镜像数量限制为此数字（负值表示无限制，0将导致不会将镜像加入队列，从而有效地禁用镜像更新）。
- `PUSH_LIMIT`: **50**: 将要添加到队列的镜像数量限制为此数字（负值表示无限制，0将导致不会将镜像加入队列，从而有效地禁用镜像更新）。

#### 定时任务 - 健康检查所有仓库 (`cron.repo_health_check`)

- `SCHEDULE`: **@midnight**: Cron语法，用于安排仓库健康检查。
- `TIMEOUT`: **60s**: 用于健康检查执行超时的时间持续语法。
- `ARGS`: **_empty_**: `git fsck` 命令的参数，例如 `--unreachable --tags`。在 http://git-scm.com/docs/git-fsck 上了解更多。

#### 定时任务 - 检查所有仓库统计 (`cron.check_repo_stats`)

- `RUN_AT_START`: **true**: 在启动时运行仓库统计检查。
- `SCHEDULE`: **@midnight**: Cron语法，用于安排仓库统计检查。

#### 定时任务 - 清理 hook_task 表 (`cron.cleanup_hook_task_table`)

- `ENABLED`: **true**: 启用清理 hook_task 任务。
- `RUN_AT_START`: **false**: 在启动时运行清理 hook_task（如果启用）。
- `SCHEDULE`: **@midnight**: Cron语法，用于清理 hook_task 表。
- `CLEANUP_TYPE` **OlderThan** OlderThan 或 PerWebhook 方法来清理 hook_task，可以按年龄（即 hook_task 记录传递多久）或按每个 Webhook 保留的数量（即每个 Webhook 保留最新的 x 个传递）来清理。
- `OLDER_THAN`: **168h**: 如果 CLEANUP_TYPE 设置为 OlderThan，则早于此表达式的任何传递的 hook_task 记录将被删除。
- `NUMBER_TO_KEEP`: **10**: 如果 CLEANUP_TYPE 设置为 PerWebhook，则 Webhook 的此数量 hook_task 记录将被保留（即保留最新的 x 个传递）。

#### Cron - 清理过期的包 (`cron.cleanup_packages`)

- `ENABLED`: **true**: 启用清理过期包任务。
- `RUN_AT_START`: **true**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 每次运行此任务时都会通知。
- `SCHEDULE`: **@midnight**: Cron语法，用于任务。
- `OLDER_THAN`: **24h**: 未引用的包数据创建超过 OLDER_THAN 时间的包将被删除。

#### Cron - 更新迁移海报 ID (`cron.update_migration_poster_id`)

- `SCHEDULE`: **@midnight** : 同步之间的间隔作为持续时间，每次实例启动时都会尝试同步。

#### Cron - 同步外部用户 (`cron.sync_external_users`)

- `SCHEDULE`: **@midnight** : 同步之间的间隔作为持续时间，每次实例启动时都会尝试同步。
- `UPDATE_EXISTING`: **true**: 创建新用户，更新现有用户数据，并禁用不再在外部源中的用户（默认设置）或仅在 UPDATE_EXISTING 设置为 false 时创建新用户。

### 扩展的定时任务（默认未启用）

#### Cron - 垃圾收集所有仓库 (`cron.git_gc_repos`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。
- `TIMEOUT`: **60s**: 用于垃圾收集执行超时的时间持续语法。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `ARGS`: **_empty_**: `git gc` 命令的参数，例如 `--aggressive --auto`。默认值与 [git] -> GC_ARGS 相同。

#### Cron - 使用 Gitea SSH 密钥更新 '.ssh/authorized_keys' 文件 (`cron.resync_all_sshkeys`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。

#### Cron - 重新同步所有仓库的 pre-receive、update 和 post-receive 钩子 (`cron.resync_all_hooks`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。

#### Cron - 重新初始化所有缺失的 Git 仓库，但记录已存在 (`cron.reinit_missing_repos`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。

#### Cron - 删除所有缺少 Git 文件的仓库 (`cron.delete_missing_repos`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。

#### Cron - 删除生成的仓库头像 (`cron.delete_generated_repository_avatars`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 72h**: Cron语法，用于安排仓库存档清理，例如 `@every 1h`。

#### Cron - 从数据库中删除所有旧的操作 (`cron.delete_old_actions`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NOTICE_ON_SUCCESS`: **false**: 设置为 true 以打开成功通知。
- `SCHEDULE`: **@every 168h**: Cron语法，用于设置多长时间进行检查。
- `OLDER_THAN`: **8760h**: 早于此表达式的任何操作都将从数据库中删除，建议使用 `8760h`（1年），因为这是热力图的最大长度。

#### Cron - 从数据库中删除所有旧的系统通知 (`cron.delete_old_system_notices`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `NO_SUCCESS_NOTICE`: **false**: 设置为 true 以关闭成功通知。
- `SCHEDULE`: **@every 168h**: Cron语法，用于设置多长时间进行检查。
- `OLDER_THAN`: **8760h**: 早于此表达式的任何系统通知都将从数据库中删除。

#### Cron - 在仓库中回收 LFS 指针 (`cron.gc_lfs`)

- `ENABLED`: **false**: 启用服务。
- `RUN_AT_START`: **false**: 在启动时运行任务（如果启用）。
- `SCHEDULE`: **@every 24h**: Cron语法，用于设置多长时间进行检查。
- `OLDER_THAN`: **168h**: 只会尝试回收早于此时间（默认7天）的 LFSMetaObject。
- `LAST_UPDATED_MORE_THAN_AGO`: **72h**: 只会尝试回收超过此时间（默认3天）没有尝试过回收的 LFSMetaObject。
- `NUMBER_TO_CHECK_PER_REPO`: **100**: 每个仓库要检查的过期 LFSMetaObject 的最小数量。设置为 `0` 以始终检查所有。

## Git (`git`)

- `PATH`: **""**: Git可执行文件的路径。如果为空，Gitea将在PATH环境中搜索。
- `HOME_PATH`: **%(APP_DATA_PATH)s/home**: Git的HOME目录。
   此目录将用于包含Gitea的git调用将使用的`.gitconfig`和可能的`.gnupg`目录。如果您可以确认Gitea是在此环境中唯一运行的应用程序，您可以将其设置为Gitea用户的正常主目录。
- `DISABLE_DIFF_HIGHLIGHT`: **false**: 禁用已添加和已删除更改的高亮显示。
- `MAX_GIT_DIFF_LINES`: **1000**: 在diff视图中允许单个文件的最大行数。
- `MAX_GIT_DIFF_LINE_CHARACTERS`: **5000**: 在diff视图中每行的最大字符数。
- `MAX_GIT_DIFF_FILES`: **100**: 在diff视图中显示的最大文件数。
- `COMMITS_RANGE_SIZE`: **50**: 设置默认的提交范围大小
- `BRANCHES_RANGE_SIZE`: **20**: 设置默认的分支范围大小
- `GC_ARGS`: **_empty_**: 命令`git gc`的参数，例如`--aggressive --auto`。更多信息请参见http://git-scm.com/docs/git-gc/
- `ENABLE_AUTO_GIT_WIRE_PROTOCOL`: **true**: 如果使用Git版本 >= 2.18时使用Git wire协议版本2，默认为true，当您始终希望使用Git wire协议版本1时设置为false。
  要在使用OpenSSH服务器的情况下为通过SSH的Git启用此功能，请将`AcceptEnv GIT_PROTOCOL`添加到您的sshd_config文件中。
- `PULL_REQUEST_PUSH_MESSAGE`: **true**: 对于推送到非默认分支的响应，使用URL创建拉取请求（如果启用了该存储库的拉取请求）
- `VERBOSE_PUSH`: **true**: 在处理推送时打印有关推送状态的信息。
- `VERBOSE_PUSH_DELAY`: **5s**: 仅在推送时间超过此延迟时才打印详细信息。
- `LARGE_OBJECT_THRESHOLD`: **1048576**: （仅限于Go-Git），不要在内存中缓存大于此大小的对象。（设置为0以禁用。）
- `DISABLE_CORE_PROTECT_NTFS`: **false** 将`core.protectNTFS`强制设置为false。
- `DISABLE_PARTIAL_CLONE`: **false** 禁用使用部分克隆进行git。

### Git - 超时设置 (`git.timeout`)

- `DEFAULT`: **360**: Git操作的默认超时时间，单位秒
- `MIGRATE`: **600**: 在迁移外部存储库时的超时时间，单位秒
- `MIRROR`: **300**: 在镜像外部存储库时的超时时间，单位秒
- `CLONE`: **300**: 在存储库之间进行内部克隆的超时时间，单位秒
- `PULL`: **300**: 在存储库之间进行内部拉取的超时时间，单位秒
- `GC`: **60**: git存储库GC的超时时间，单位秒

### Git - 配置选项 (`git.config`)

此部分中的键/值对将用作git配置。
此部分仅执行“设置”配置，从此部分中删除的配置键不会自动从git配置中删除。格式为`some.configKey = value`。

- `diff.algorithm`: **histogram**
- `core.logAllRefUpdates`: **true**
- `gc.reflogExpire`: **90**

## 指标 (`metrics`)

- `ENABLED`: **false**: 启用/prometheus的metrics端点。
- `ENABLED_ISSUE_BY_LABEL`: **false**: 启用按标签统计问题，格式为`gitea_issues_by_label{label="bug"} 2`。
- `ENABLED_ISSUE_BY_REPOSITORY`: **false**: 启用按存储库统计问题，格式为`gitea_issues_by_repository{repository="org/repo"} 5`。
- `TOKEN`: **_empty_**: 如果要在授权中包含指标，则需要指定令牌。相同的令牌需要在prometheus参数`bearer_token`或`bearer_token_file`中使用。

## API (`api`)

- `ENABLE_SWAGGER`: **true**: 启用API文档接口 (`/api/swagger`, `/api/v1/swagger`, …). True or false。
- `MAX_RESPONSE_ITEMS`: **50**: API分页的最大单页项目数。
- `DEFAULT_PAGING_NUM`: **30**: API分页的默认分页数。
- `DEFAULT_GIT_TREES_PER_PAGE`: **1000**: Git trees API的默认单页项目数。
- `DEFAULT_MAX_BLOB_SIZE`: **10485760** (10MiB): blobs API的默认最大文件大小。

## OAuth2 (`oauth2`)

- `ENABLED`: **true**：启用OAuth2提供者。
- `ACCESS_TOKEN_EXPIRATION_TIME`：**3600**：OAuth2访问令牌的生命周期，以秒为单位。
- `REFRESH_TOKEN_EXPIRATION_TIME`：**730**：OAuth2刷新令牌的生命周期，以小时为单位。
- `INVALIDATE_REFRESH_TOKENS`：**false**：检查刷新令牌是否已被使用。
- `JWT_SIGNING_ALGORITHM`：**RS256**：用于签署OAuth2令牌的算法。有效值：[`HS256`，`HS384`，`HS512`，`RS256`，`RS384`，`RS512`，`ES256`，`ES384`，`ES512`]。
- `JWT_SECRET`：**_empty_**：OAuth2访问和刷新令牌的身份验证密钥，请将其更改为唯一的字符串。仅当`JWT_SIGNING_ALGORITHM`设置为`HS256`，`HS384`或`HS512`时才需要此设置。
- `JWT_SECRET_URI`：**_empty_**：可以使用此配置选项，而不是在配置中定义`JWT_SECRET`，以向Gitea提供包含密钥的文件的路径（示例值：`file:/etc/gitea/oauth2_jwt_secret`）。
- `JWT_SIGNING_PRIVATE_KEY_FILE`：**jwt/private.pem**：用于签署OAuth2令牌的私钥文件路径。路径相对于`APP_DATA_PATH`。仅当`JWT_SIGNING_ALGORITHM`设置为`RS256`，`RS384`，`RS512`，`ES256`，`ES384`或`ES512`时才需要此设置。文件必须包含PKCS8格式的RSA或ECDSA私钥。如果不存在密钥，则将为您创建一个4096位密钥。
- `MAX_TOKEN_LENGTH`：**32767**：从OAuth2提供者接受的令牌/cookie的最大长度。
- `DEFAULT_APPLICATIONS`：**git-credential-oauth，git-credential-manager, tea**：在启动时预注册用于某些服务的OAuth应用程序。有关可用选项列表，请参阅[OAuth2文档](/development/oauth2-provider.md)。

## i18n (`i18n`)

- `LANGS`: **en-US,zh-CN,zh-HK,zh-TW,de-DE,fr-FR,nl-NL,lv-LV,ru-RU,uk-UA,ja-JP,es-ES,pt-BR,pt-PT,pl-PL,bg-BG,it-IT,fi-FI,tr-TR,cs-CZ,sv-SE,ko-KR,el-GR,fa-IR,hu-HU,id-ID,ml-IN**：
    在语言选择器中显示的区域设置列表。如果用户浏览器的语言与列表中的任何区域设置不匹配，则将使用第一个区域设置作为默认值。

- `NAMES`：**English,简体中文,繁體中文（香港）,繁體中文（台灣）,Deutsch,Français,Nederlands,Latviešu,Русский,Українська,日本語,Español,Português do Brasil,Português de Portugal,Polski,Български,Italiano,Suomi,Türkçe,Čeština,Српски,Svenska,한국어,Ελληνικά,فارسی,Magyar nyelv,Bahasa Indonesia,മലയാളം**：
    对应于各区域设置的可见名称。

## Markup (`markup`)

- `MERMAID_MAX_SOURCE_CHARACTERS`: **5000**: 设置Mermaid源的最大大小。(设为-1代表禁止)

gitea支持外部渲染工具，你可以配置你熟悉的文档渲染工具. 比如一下将新增一个名字为 asciidoc 的渲染工具。

```ini
[markup.asciidoc]
ENABLED = true
NEED_POSTPROCESS = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoctor --embedded --safe-mode=secure --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED：**false** 设置是否启动渲染器
- NEED_POSTPROCESS：**true** 设置为**true**以替换链接/SHA1等。
- FILE_EXTENSIONS：**_empty_** 要由外部命令渲染的文件扩展名列表。多个扩展名需要用逗号分隔。
- RENDER_COMMAND：用于渲染所有匹配的扩展名的外部命令。
- IS_INPUT_FILE：**false** 输入不是标准输入，而是一个在`RENDER_COMMAND`之后带有文件参数的文件。
- RENDER_CONTENT_MODE：**sanitized** 内容将如何呈现。
  - sanitized：对内容进行清理，并在当前页面内呈现，默认仅允许一些HTML标签和属性。可以在`[markup.sanitizer.*]`中定义自定义的清理规则。
  - no-sanitizer：禁用清理程序，在当前页面内呈现内容。这是**不安全**的，如果内容包含恶意代码，可能会导致XSS攻击。
  - iframe：在单独的独立页面中呈现内容，并通过iframe嵌入到当前页面中。iframe处于禁用同源策略的沙箱模式，并且JS代码与父页面安全隔离。

两个特殊的环境变量会传递给渲染命令：

- `GITEA_PREFIX_SRC`，其中包含`src`路径树中的当前URL前缀。用作链接的前缀。
- `GITEA_PREFIX_RAW`，其中包含`raw`路径树中的当前URL前缀。用作图像路径的前缀。

如果`RENDER_CONTENT_MODE`为`sanitized`，Gitea支持自定义用于呈现的HTML的清理策略。下面的示例将支持来自pandoc的KaTeX输出。

```ini
[markup.sanitizer.TeX]
; Pandoc renders TeX segments as <span>s with the "math" class, optionally
; with "inline" or "display" classes depending on context.
ELEMENT = span
ALLOW_ATTR = class
REGEXP = ^\s*((math(\s+|$)|inline(\s+|$)|display(\s+|$)))+
ALLOW_DATA_URI_IMAGES = true
```

- `ELEMENT`：此策略适用于的元素。必须非空。
- `ALLOW_ATTR`：此策略允许的属性。必须非空。
- `REGEXP`：用于匹配属性内容的正则表达式。必须存在，但可以为空，以无条件允许此属性的白名单。
- `ALLOW_DATA_URI_IMAGES`：**false** 允许数据URI图像（`<img src="data:image/png;base64,..."/>`）。

可以通过添加唯一的子节来定义多个清理规则，例如`[markup.sanitizer.TeX-2]`。
要仅为指定的外部渲染器应用清理规则，它们必须使用渲染器名称，例如`[markup.sanitizer.asciidoc.rule-1]`。
如果规则在渲染器ini节之上定义，或者名称与渲染器不匹配，则应用于每个渲染器。

## 代码高亮映射 (`highlight.mapping`)

- `file_extension 比如 .toml`: **language 比如 ini**。文件扩展名到语言的映射覆盖。

- Gitea 将使用 `.gitattributes` 文件中的 `linguist-language` 或 `gitlab-language` 属性来对文件进行高亮显示，如果可用。
如果未设置此属性或语言不可用，则将查找文件扩展名在此映射中或使用启发式方法来确定文件类型。

## 时间 (`time`)

- `DEFAULT_UI_LOCATION`：在 UI 上的默认时间位置，以便我们可以在 UI 上显示正确的用户时间。例如：Asia/Shanghai

## 迁移 (`migrations`)

- `MAX_ATTEMPTS`：**3**：每次 http/https 请求的最大尝试次数（用于迁移）。
- `RETRY_BACKOFF`：**3**：每次 http/https 请求重试的退避时间（秒）。
- `ALLOWED_DOMAINS`：**_empty_**：允许迁移仓库的域名允许列表，默认为空。这意味着允许一切。多个域名可以用逗号分隔。支持通配符：`github.com, *.github.com`。
- `BLOCKED_DOMAINS`：**_empty_**：阻止迁移仓库的域名阻止列表，默认为空。多个域名可以用逗号分隔。当 `ALLOWED_DOMAINS` 不为空时，此选项优先级较高，用于拒绝域名。支持通配符。
- `ALLOW_LOCALNETWORKS`：**false**：允许 RFC 1918、RFC 1122、RFC 4632 和 RFC 4291 中定义的私有地址。如果域名被 `ALLOWED_DOMAINS` 允许，此选项将被忽略。
- `SKIP_TLS_VERIFY`：**false**：允许跳过 TLS 验证。

## 联邦（`federation`）

- `ENABLED`：**false**：启用/禁用联邦功能。
- `SHARE_USER_STATISTICS`：**true**：如果启用联邦，则启用/禁用节点信息的用户统计信息。
- `MAX_SIZE`：**4**：联邦请求和响应的最大大小（MB）。

警告：更改以下设置可能会破坏联邦功能。

- `ALGORITHMS`：**rsa-sha256, rsa-sha512, ed25519**：HTTP 签名算法。
- `DIGEST_ALGORITHM`：**SHA-256**：HTTP 签名摘要算法。
- `GET_HEADERS`：**(request-target), Date**：用于联邦请求的 GET 头部。
- `POST_HEADERS`：**(request-target), Date, Digest**：用于联邦请求的 POST 头部。

## 包（`packages`）

- `ENABLED`：**true**：启用/禁用包注册表功能。
- `CHUNKED_UPLOAD_PATH`：**tmp/package-upload**：分块上传的路径。默认为 `APP_DATA_PATH` + `tmp/package-upload`。
- `LIMIT_TOTAL_OWNER_COUNT`：**-1**：单个所有者可以拥有的包版本的最大数量（`-1` 表示无限制）。
- `LIMIT_TOTAL_OWNER_SIZE`：**-1**：单个所有者可以使用的包的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_ALPINE`：**-1**：Alpine 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CARGO`：**-1**：Cargo 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CHEF`：**-1**：Chef 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_COMPOSER`：**-1**：Composer 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CONAN`：**-1**：Conan 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CONDA`：**-1**：Conda 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CONTAINER`：**-1**：Container 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_CRAN`：**-1**：CRAN 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_DEBIAN`：**-1**：Debian 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_GENERIC`：**-1**：通用上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_GO`：**-1**：Go 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_HELM`：**-1**：Helm 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_MAVEN`：**-1**：Maven 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_NPM`：**-1**：npm 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_NUGET`：**-1**：NuGet 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_PUB`：**-1**：Pub 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_PYPI`：**-1**：PyPI 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_RPM`：**-1**：RPM 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_RUBYGEMS`：**-1**：RubyGems 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_SWIFT`：**-1**：Swift 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。
- `LIMIT_SIZE_VAGRANT`：**-1**：Vagrant 上传的最大大小（`-1` 表示无限制，格式为 `1000`、`1 MB`、`1 GiB`）。

## 镜像（`mirror`）

- `ENABLED`：**true**：启用镜像功能。设置为 **false** 以禁用所有镜像。预先存在的镜像保持有效，但不会更新；可以转换为常规仓库。
- `DISABLE_NEW_PULL`：**false**：禁用创建**新的**拉取镜像。预先存在的镜像保持有效。如果 `mirror.ENABLED` 为 `false`，将被忽略。
- `DISABLE_NEW_PUSH`：**false**：禁用创建**新的**推送镜像。预先存在的镜像保持有效。如果 `mirror.ENABLED` 为 `false`，将被忽略。
- `DEFAULT_INTERVAL`：**8h**：每次检查之间的默认间隔。
- `MIN_INTERVAL`：**10m**：检查的最小间隔。（必须大于 1 分钟）。

## LFS (`lfs`)

用于 lfs 数据的存储配置。当将 `STORAGE_TYPE` 设置为 `xxx` 时，它将从默认的 `[storage]` 或 `[storage.xxx]` 派生。
当派生时，`PATH` 的默认值是 `data/lfs`，`MINIO_BASE_PATH` 的默认值是 `lfs/`。

- `STORAGE_TYPE`：**local**：lfs 的存储类型，`local` 表示本地磁盘，`minio` 表示 S3 兼容对象存储服务，或者使用 `[storage.xxx]` 中定义的其他名称。
- `SERVE_DIRECT`：**false**：允许存储驱动程序重定向到经过身份验证的 URL 以直接提供文件。目前，仅支持通过签名的 URL 提供 Minio/S3，本地不执行任何操作。
- `PATH`：**./data/lfs**：存储 LFS 文件的位置，仅在 `STORAGE_TYPE` 为 `local` 时可用。如果未设置，则回退到 `[server]` 部分中已弃用的 `LFS_CONTENT_PATH` 值。
- `MINIO_ENDPOINT`：**localhost:9000**：连接的 Minio 终端点，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_ACCESS_KEY_ID`：Minio 的 accessKeyID，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_SECRET_ACCESS_KEY`：Minio 的 secretAccessKey，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_BUCKET`：**gitea**：用于存储 lfs 的 Minio 桶，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_LOCATION`：**us-east-1**：创建桶的 Minio 位置，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_BASE_PATH`：**lfs/**：桶上的 Minio 基本路径，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_USE_SSL`：**false**：Minio 启用 ssl，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_INSECURE_SKIP_VERIFY`：**false**：Minio 跳过 SSL 验证，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_BUCKET_LOOKUP_TYPE`: **auto**: Minio的bucket查找方式默认为`auto`模式，可将其设置为`dns`（虚拟托管样式）或`path`（路径样式），仅当`STORAGE_TYPE`为`minio`时可用。

## 存储 (`storage`)

默认的附件、lfs、头像、仓库头像、仓库归档、软件包、操作日志、操作艺术品的存储配置。

- `STORAGE_TYPE`：**local**：存储类型，`local` 表示本地磁盘，`minio` 表示 S3，`azureblob` 表示 azure 对象存储。
- `SERVE_DIRECT`：**false**：允许存储驱动程序重定向到经过身份验证的 URL 以直接提供文件。目前，仅支持通过签名的 URL 提供 Minio/S3，本地不执行任何操作。
- `MINIO_ENDPOINT`：**localhost:9000**：连接的 Minio 终端点，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_ACCESS_KEY_ID`：Minio 的 accessKeyID，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_SECRET_ACCESS_KEY`：Minio 的 secretAccessKey，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_BUCKET`：**gitea**：用于存储数据的 Minio 桶，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_LOCATION`：**us-east-1**：创建桶的 Minio 位置，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_USE_SSL`：**false**：Minio 启用 ssl，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_INSECURE_SKIP_VERIFY`：**false**：Minio 跳过 SSL 验证，仅在 `STORAGE_TYPE` 为 `minio` 时可用。
- `MINIO_BUCKET_LOOKUP_TYPE`: **auto**: Minio的bucket查找方式默认为`auto`模式，可将其设置为`dns`（虚拟托管样式）或`path`（路径样式），仅当`STORAGE_TYPE`为`minio`时可用。

- `AZURE_BLOB_ENDPOINT`: **_empty_**: Azure Blob 终端点，仅在 `STORAGE_TYPE` 为 `azureblob` 时可用。例如：https://accountname.blob.core.windows.net 或 http://127.0.0.1:10000/devstoreaccount1
- `AZURE_BLOB_ACCOUNT_NAME`: **_empty_**: Azure Blob 账号名，仅在 `STORAGE_TYPE` 为 `azureblob` 时可用。
- `AZURE_BLOB_ACCOUNT_KEY`: **_empty_**: Azure Blob 访问密钥，仅在 `STORAGE_TYPE` 为 `azureblob` 时可用。
- `AZURE_BLOB_CONTAINER`: **gitea**: 用于存储数据的 Azure Blob 容器名，仅在 `STORAGE_TYPE` 为 `azureblob` 时可用。

建议的 minio 存储配置如下：

```ini
[storage]
STORAGE_TYPE = minio
; Minio endpoint to connect only available when STORAGE_TYPE is `minio`
MINIO_ENDPOINT = localhost:9000
; Minio accessKeyID to connect only available when STORAGE_TYPE is `minio`
MINIO_ACCESS_KEY_ID =
; Minio secretAccessKey to connect only available when STORAGE_TYPE is `minio`
MINIO_SECRET_ACCESS_KEY =
; Minio bucket to store the attachments only available when STORAGE_TYPE is `minio`
MINIO_BUCKET = gitea
; Minio location to create bucket only available when STORAGE_TYPE is `minio`
MINIO_LOCATION = us-east-1
; Minio enabled ssl only available when STORAGE_TYPE is `minio`
MINIO_USE_SSL = false
; Minio skip SSL verification available when STORAGE_TYPE is `minio`
MINIO_INSECURE_SKIP_VERIFY = false
SERVE_DIRECT = true
; Minio bucket lookup method defaults to auto mode; set it to `dns` for virtual host style or `path` for path style, only available when STORAGE_TYPE is `minio`
MINIO_BUCKET_LOOKUP_TYPE = auto
```

默认情况下，每个存储都有其默认的基本路径，如下所示：

| storage           | default base path  |
| ----------------- | ------------------ |
| attachments       | attachments/       |
| lfs               | lfs/               |
| avatars           | avatars/           |
| repo-avatars      | repo-avatars/      |
| repo-archive      | repo-archive/      |
| packages          | packages/          |
| actions_log       | actions_log/       |
| actions_artifacts | actions_artifacts/ |

并且桶（bucket）、基本路径或`SERVE_DIRECT`可以是特殊的或被覆盖的，如果您想要使用不同的设置，您可以：

```ini
[storage.actions_log]
MINIO_BUCKET = gitea_actions_log
SERVE_DIRECT = true
MINIO_BASE_PATH = my_actions_log/ ; default is actions_log/ if blank
```

如果您想为' lfs '自定义一个不同的存储，如果上面定义了默认存储

```ini
[lfs]
STORAGE_TYPE = my_minio

[storage.my_minio]
STORAGE_TYPE = minio
; Minio endpoint to connect only available when STORAGE_TYPE is `minio`
MINIO_ENDPOINT = localhost:9000
; Minio accessKeyID to connect only available when STORAGE_TYPE is `minio`
MINIO_ACCESS_KEY_ID =
; Minio secretAccessKey to connect only available when STORAGE_TYPE is `minio`
MINIO_SECRET_ACCESS_KEY =
; Minio bucket to store the attachments only available when STORAGE_TYPE is `minio`
MINIO_BUCKET = gitea
; Minio location to create bucket only available when STORAGE_TYPE is `minio`
MINIO_LOCATION = us-east-1
; Minio enabled ssl only available when STORAGE_TYPE is `minio`
MINIO_USE_SSL = false
; Minio skip SSL verification available when STORAGE_TYPE is `minio`
MINIO_INSECURE_SKIP_VERIFY = false
; Minio bucket lookup method defaults to auto mode; set it to `dns` for virtual host style or `path` for path style, only available when STORAGE_TYPE is `minio`
MINIO_BUCKET_LOOKUP_TYPE = auto
```

### 存储库归档存储 (`storage.repo-archive`)

存储库归档存储的配置。当将`STORAGE_TYPE`设置为`xxx`时，它将继承默认的 `[storage]` 或 `[storage.xxx]` 配置。`PATH`的默认值是`data/repo-archive`，`MINIO_BASE_PATH`的默认值是`repo-archive/`。

- `STORAGE_TYPE`: **local**：存储类型，`local`表示本地磁盘，`minio`表示与S3兼容的对象存储服务，或者使用定义为`[storage.xxx]`的其他名称。
- `SERVE_DIRECT`: **false**：允许存储驱动程序重定向到经过身份验证的URL以直接提供文件。目前，只有Minio/S3支持通过签名URL，本地不执行任何操作。
- `PATH`: **./data/repo-archive**：用于存储归档文件的位置，仅在`STORAGE_TYPE`为`local`时可用。
- `MINIO_ENDPOINT`: **localhost:9000**：Minio端点，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_ACCESS_KEY_ID`: Minio的accessKeyID，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_SECRET_ACCESS_KEY`: Minio的secretAccessKey，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_BUCKET`: **gitea**：用于存储归档的Minio存储桶，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_LOCATION`: **us-east-1**：用于创建存储桶的Minio位置，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_BASE_PATH`: **repo-archive/**：存储桶上的Minio基本路径，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_USE_SSL`: **false**：启用Minio的SSL，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_INSECURE_SKIP_VERIFY`: **false**：跳过Minio的SSL验证，仅在`STORAGE_TYPE`为`minio`时可用。
- `MINIO_BUCKET_LOOKUP_TYPE`: **auto**: Minio的bucket查找方式默认为`auto`模式，可将其设置为`dns`（虚拟托管样式）或`path`（路径样式），仅当`STORAGE_TYPE`为`minio`时可用。

### 存储库归档 (`repo-archive`)

- `STORAGE_TYPE`: **local**：存储类型，用于操作日志，`local`表示本地磁盘，`minio`表示与S3兼容的对象存储服务，默认为`local`，或者使用定义为`[storage.xxx]`的其他名称。
- `MINIO_BASE_PATH`: **repo-archive/**：Minio存储桶上的基本路径，仅在`STORAGE_TYPE`为`minio`时可用。

## 代理 (`proxy`)

- `PROXY_ENABLED`: **false**: 启用代理，如果为true，所有通过HTTP向外部的请求都将受到影响，如果为false，即使环境设置了http_proxy/https_proxy也不会使用
- `PROXY_URL`: **_empty_**: 代理服务器地址，支持 http://, https//, socks://，为空则不启用代理而使用环境变量中的 http_proxy/https_proxy
- `PROXY_HOSTS`: **_empty_**: 逗号分隔的多个需要代理的网址，支持 * 号匹配符号， ** 表示匹配所有网站

i.e.

```ini
PROXY_ENABLED = true
PROXY_URL = socks://127.0.0.1:1080
PROXY_HOSTS = *.github.com
```

## Actions (`actions`)

- `ENABLED`: **true**：启用/禁用操作功能
- `DEFAULT_ACTIONS_URL`: **github**：获取操作插件的默认平台，`github`表示`https://github.com`，`self`表示当前的 Gitea 实例。
- `STORAGE_TYPE`: **local**：用于操作日志的存储类型，`local`表示本地磁盘，`minio`表示与S3兼容的对象存储服务，默认为`local`，或者使用定义为`[storage.xxx]`的其他名称。
- `MINIO_BASE_PATH`: **actions_log/**：Minio存储桶上的基本路径，仅在`STORAGE_TYPE`为`minio`时可用。

`DEFAULT_ACTIONS_URL` 指示 Gitea 操作运行程序应该在哪里找到带有相对路径的操作。
例如，`uses: actions/checkout@v4` 表示 `https://github.com/actions/checkout@v4`，因为 `DEFAULT_ACTIONS_URL` 的值为 `github`。
它可以更改为 `self`，以使其成为 `root_url_of_your_gitea/actions/checkout@v4`。

请注意，对于大多数情况，不建议使用 `self`，因为它可能使名称在全局范围内产生歧义。
此外，它要求您将所有所需的操作镜像到您的 Gitea 实例，这可能不值得。
因此，请仅在您了解自己在做什么的情况下使用 `self`。

在早期版本（`<= 1.19`）中，`DEFAULT_ACTIONS_URL` 可以设置为任何自定义 URL，例如 `https://gitea.com` 或 `http://your-git-server,https://gitea.com`，默认值为 `https://gitea.com`。
然而，后来的更新删除了这些选项，现在唯一的选项是 `github` 和 `self`，默认值为 `github`。
但是，如果您想要使用其他 Git 服务器中的操作，您可以在 `uses` 字段中使用完整的 URL，Gitea 支持此功能（GitHub 不支持）。
例如 `uses: https://gitea.com/actions/checkout@v4` 或 `uses: http://your-git-server/actions/checkout@v4`。

## 其他 (`other`)

- `SHOW_FOOTER_VERSION`: **true**: 在页面底部显示Gitea的版本。
- `SHOW_FOOTER_TEMPLATE_LOAD_TIME`: **true**: 在页脚显示模板执行的时间。
- `SHOW_FOOTER_POWERED_BY`: **true**: 在页脚显示“由...提供动力”的文本。
- `ENABLE_SITEMAP`: **true**: 生成sitemap.
- `ENABLE_FEED`: **true**: 是否启用RSS/Atom
