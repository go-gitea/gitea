---
date: "2016-11-08T16:00:00+02:00"
title: "文档"
slug: /
sidebar_position: 10
toc: false
draft: false
---

# 关于Gitea

Gitea 是一个无痛的自助式一体化软件托管平台服务，包括 Git 托管、代码审查、团队协作、软件包注册和 CI/CD。它与 GitHub、Bitbucket 和 GitLab 等比较类似。
Gitea 最初是从 [Gogs](http://gogs.io) 分支而来，几乎所有代码都已更改。对于我们Fork的原因可以看
[这里](https://blog.gitea.com/welcome-to-gitea/)。

## 目标

Gitea的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好
的自建 Git 服务。

采用Go作为后端语言，只需生成一个可执行程序即可。
支持 Linux, macOS 和 Windows等多平台，
 支持主流的x86，amd64、
 ARM 和 PowerPC等架构。

## 功能特性

- 用户仪表板
  - 切换控制面板用户(组织/当前用户)
  - 活动时间线
    - 提交
    - 工单
    - 合并请求
    - 仓库创建
  - 可搜索的仓库列表
  - 组织列表
  - 镜像仓库列表
- 工单管理仪表盘
  - 身份切换(组织和当前用户)
  - 筛选
    - 开启中
    - 已关闭
    - 用户仓库中
    - 指派
    - 被提及
    - 根据关联的仓库筛选
  - 排序规则
    - 最近更新
    - 最早更新
    - 最多评论
- 合并请求仪表盘
  - 与工单仪表盘类似
- 存储库类型
  - 镜像仓库
  - 普通仓库
  - 迁移仓库
- 提醒(邮件和网页)
  - 已读
  - 未读
  - 固定(Pin)
- 探索页
  - 用户
  - 仓库
  - 组织
  - 搜索
- 自定义模板
- 覆盖公共文件（图标、CSS样式等）
- CSRF 和 XSS 保护
- 支持 HTTPS
- 设置允许的上传大小和类型
- 日志
- 配置
  - 数据库
    - MySQL (>=5.7)
    - PostgreSQL (>=10)
    - SQLite3
    - MSSQL (>=2008R2 SP3)
    - TiDB (MySQL protocol)
  - 配置文件
    - [app.ini](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini)
  - 管理员面板
    - 数据统计
    - 动作
      - 删除所有未激活的帐户
      - 删除所有代码库的存档
      - 删除所有丢失 Git 文件的仓库
      - 对仓库进行垃圾回收
      - 使用 Gitea 的 SSH 密钥更新
      - 重新同步所有仓库的 pre-receive、update 和 post-receive 钩子
      - 重新初始化所有丢失的 Git 仓库存在的记录
    - 系统状态监控
      - 服务状态：运行时间、协程数量等
      - 内存状态：内存使用情况、内存占用量、分配、释放情况等
      - 协程数量
      - 等等
    - 用户管理
      - 搜索
      - 排序
      - 上次登录
      - 认证源
      - 最大存储库数量
      - 禁用账户
      - 管理员权限
      - 创建 Git hooks的权限
      - 创建组织的权限
      - 导入仓库的权限
    - 组织管理
      - 成员
      - 团队
      - 头像
      - 钩子
    - 仓库管理
      - 查看所有仓库信息并管理仓库
    - 认证源
      - OAuth
      - PAM
      - LDAP
      - SMTP
    - 配置查看器
      - 配置文件中的全部内容
    - 系统通知
      - 当意外发生时
    - 监测
      - 当前处理器
      - 定时任务
        - 更新镜像
        - 仓库健康检查
        - 检查仓库统计数据
        - 清理旧存档
  - 环境变量
  - 命令行选项
- 多语言支持 ([21种语言](https://github.com/go-gitea/gitea/tree/main/options/locale))
- [Mermaid](https://mermaidjs.github.io/) Markdown 图表
- Markdown 中的数学语法
- 邮件服务
  - 通知
  - 注册确认
  - 密码重置
- 支持反向代理
  - 包括子路径
- 用户
  - 简介
    - 名称
    - 用户名
    - 邮件
    - 网站
    - 注册时间
    - 粉丝和关注
    - 组织
    - 仓库
    - 活动
    - 点赞仓库
  - 设置
    - 与用户简介相同，更多信息见下文
    - 保持电子邮件的私密性
    - 头像
      - Gravatar
      - Libravatar
      - 自定义
    - 密码
    - 多个电子邮件地址
    - SSH Keys
    - 连接应用程序
    - 双因素认证
    - 已链接的 OAuth2 源
    - 删除帐户
- 仓库
  - 使用 SSH/HTTP/HTTPS协议克隆
  - Git LFS
  - 关注、点赞、派生
  - 查看关注、点赞、派生
  - 代码
    - 分支查看
    - 基于 Web 的文件上传和创建
    - 克隆网址
    - 下载
      - ZIP
      - TAR.GZ
    - 基于 Web 的编辑器
      - Markdown 编辑器
      - 纯文本编辑器
        - 语法高亮
      - Diff预览
      - 预览
      - 选择commit分支
    - 查看文件历史
    - 删除文件
    - 查看原始数据
  - 工单
    - 工单模板
    - 里程碑
    - 标签
    - 分配g工单
    - 跟踪时间
    - 响应
    - 过滤器
      - 开启中
      - 已经关闭
      - 指派
      - 创建者
      - 被提及
    - 排序
      - 最早创建
      - 最近更新
      - 评论数量
    - 查找
    - 评论
    - 附件
  - 合并请求
    - 和工单相同的功能
  - 提交
    - 提交图
    - 按分支查看
    - 查找
    - 在所有分支中搜索
    - 查看差异
    - 查看SHA
    - 查看作者
    - 浏览提交中的文件
  - 版本发布
    - 附件
    - 标题
    - 内容
    - 删除
    - 标记为预发布
    - 选择分支
  - 百科
    - 导入
    - Markdown编辑器
  - 设置
    - 选项
      - 名称
      - 描述
      - 私有/公开
      - 网站
      - 百科
        - 启用/禁用
        - 内部/外部
      - 工单
        - 启用/禁用
        - 内部/外部
        - 支持外部的url来帮助集成
      - 启用/禁用合并请求
      - 迁移仓库
      - 删除百科
      - 删除仓库
    - 协作
      - Read/write/admin
    - 分支
      - 默认分支
      - 分支保护
    - Webhooks
    - Git Hooks
    - 部署keys
- 软件包注册
  - Composer
  - Conan
  - Container
  - Generic
  - Helm
  - Maven
  - NPM
  - Nuget
  - PyPI
  - RubyGems

## 系统要求

- 树莓派Pi3功能强大，足以运行 Gitea 来处理小型工作负载。
- 对于小型团队/项目而言，2 个 CPU 内核和 1GB 内存通常就足够了。
- 在 UNIX 系统上，Gitea 应使用专用的非 root 系统账户运行。
  - 注意：Gitea 管理 `~/.ssh/authorized_keys` 文件。以普通用户身份运行 Gitea 可能会破坏该用户的登录能力。
- [Git](https://git-scm.com/) 需要 2.0.0 或更高版本。
  - [Git Large File Storage](https://git-lfs.github.com/) 如果启用，且 Git 版本大于等于 2.1.2，则该选项可用
  - 如果 Git 版本大于等于 2.18，将自动启用 Git 提交历史图形化展示功能

## 浏览器支持

- Last 2 versions of Chrome, Firefox, Safari and Edge
- Firefox ESR

## 技术栈

- Web框架： [Chi](http://github.com/go-chi/chi)
- ORM: [XORM](https://xorm.io)
- UI 框架：
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - 更多组件参见 package.json
- 编辑器：
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
- 数据库驱动：
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## 集成支持

请访问 [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/) 获得更多的第三方集成支持
