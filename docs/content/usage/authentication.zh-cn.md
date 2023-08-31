---
date: "2016-12-01T16:00:00+02:00"
title: "认证"
slug: "authentication"
sidebar_position: 10
toc: false
draft: false
aliases:
  - /zh-cn/authentication
menu:
  sidebar:
    parent: "usage"
    name: "认证"
    sidebar_position: 10
    identifier: "authentication"
---

# 认证

## 轻量级目录访问协议（Lightweight Directory Access Protocol，LDAP）

通过BindDN的LDAP和简单认证方式LDAP共享以下字段:

- 认证名称 **(必选)**

  - 分配给新授权方法的名称。

- 主机名 **(必选)**

  - LDAP 服务的主机地址.
  - 例如:`mydomain.com`

- 端口号 **(必选)**

  - LDAP 服务的端口号.
  - 例如: LDAP `389`/ LDAPs `636`

- 安全协议 (可选)
  - 连接LDAP服务器时是否使用TLS协议。

- 管理员过滤规则 (可选)
  - 一个LDAP过滤器，用于指定哪些用户应该被赋予管理员特权。如果用户帐户符合过滤器条件，则该用户将被授予管理员权限。
  - 示例:`(objectClass=adminAccount)`
  - 适用于Microsoft Active Directory（AD）的示例:`memberOf=CN=admin-group,OU=example,DC=example,DC=org`

- 用户名属性（可选）

  - 用户LDAP记录中包含用户名称的属性。在第一次成功登录后，将使用指定的属性值作为新的Gitea账户用户名。若留空，则使用登录表单上提供的用户名。
  - 当提供的登录名与多个属性匹配时，这一选项非常有用，但是只有一个特定属性应该用于Gitea账户名称，请参阅"用户过滤器"。
  - 示例:uid
  - 适用于Microsoft Active Directory（AD）的示例:`sAMAccountName`

- 名字属性（可选）

  - 用户LDAP记录中包含用户名字的属性。将用于填充他们的账户信息。
  - 示例:givenName
- 姓氏属性（可选）
  - 用户LDAP记录中包含用户姓氏的属性。将用于填充他们的账户信息。
  - 示例:`sn`

- 电子邮件属性 **(必选)**
  - 用户LDAP记录中包含用户电子邮件地址的属性。将用于填充他们的账户信息。
  - 示例:`mail`

### LDAP(via BindDN)

需要额外设置以下字段:

- 绑定DN (可选)

  - 搜索用户时绑定到LDAP服务器的DN。这可以留空以执行匿名搜索。
  - 示例: `cn=Search,dc=mydomain,dc=com`

- 绑定密码 (可选)

  - 上述指定的Bind DN（绑定区别名）的密码，如果有的话。注意：该密码在服务器上使用SECRET_KEY进行加密存储。仍然建议确保Bind DN具有尽可能少的权限。

- 用户搜索基准 **(必选)**

  - 这是用于搜索用户帐户的LDAP基础路径.
  - 示例: `ou=Users,dc=mydomain,dc=com`

- 用户过滤规则 **(必选)**
  - LDAP 过滤器声明如何查找试图进行身份验证的用户记录
    `%[1]s`匹配参数将替换为登录表单中给出的登录名
  - 示例: `(&(objectClass=posixAccount)(|(uid=%[1]s)(mail=%[1]s)))`
  - 示例 for Microsoft Active Directory (AD): `(&(objectCategory=Person)(memberOf=CN=user-group,OU=example,DC=example,DC=org)(sAMAccountName=%s)(!(UserAccountControl:1.2.840.113556.1.4.803:=2)))`
  - 如需多次替换，应使用 `%[1]s`，例如在将提供的登录名与多个属性（如用户标识符、电子邮件甚至电话号码）进行匹配时。
  - 示例: `(&(objectClass=Person)(|(uid=%[1]s)(mail=%[1]s)(mobile=%[1]s)))`
- 启用用户同步
  - 这个选项启用了一个周期性任务，用于将Gitea用户与LDAP服务器进行同步。默认的同步周期是每24小时，
    但您可以在app.ini文件中进行更改。
    有关此部分的详细说明，请参阅[sample
    app.ini](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini)
    的_cron.sync_external_users_ 部分的注释。前面提到的_User Search Base_和_User Filter_
    设置将限制哪些用户可以使用Gitea以及哪些用户将被同步。
    在初始运行任务时，将根据给定的设置创建所有与LDAP匹配的用户，因此在使用大型企业LDAP目录时需要小心。

### LDAP(simple auth)

需要额外设置以下字段:

- 用户DN **(必选)**

  - 用作用户 DN 的模板。匹配参数 `%s` 将替换为登录表单中的登录名。
  - 示例: `cn=%s,ou=Users,dc=mydomain,dc=com`
  - 示例: `uid=%s,ou=Users,dc=mydomain,dc=com`

- 用户搜索基准 (可选)

  - 用户搜索基准声明哪些用户帐户将被搜索.
  - 示例: `ou=Users,dc=mydomain,dc=com`

- 用户过滤规则 **(必选)**
  - LDAP 过滤器声明何时允许用户登录
    `%[1]s`匹配参数将替换为登录表单中给出的登录名。
  - 示例: `(&(objectClass=posixAccount)(|(cn=%[1]s)(mail=%[1]s)))`
  - 示例: `(&(objectClass=posixAccount)(|(uid=%[1]s)(mail=%[1]s)))`

### 使用LDAP验证分组成员

使用以下字段:

- 群组搜索基础DN(可选)
  - 组使用的 LDAP DN。
  - 示例: `ou=group,dc=mydomain,dc=com`

- 组名过滤器 (可选)

  - LDAP 过滤器，声明如何在上述 DN 中查找有效组。
  - 示例: `(|(cn=gitea_users)(cn=admins))`

- 组中的用户属性 (可选)

  - 组中列出了哪个用户的 LDAP 属性。
  - 示例: `uid`

- 用户组属性 (可选)
  - 哪个组的 LDAP 属性包含一个高于用户属性名称的数组。
  - 示例: `memberUid`

## 可插拔式认证模块(Pluggable Authentication Module,PAM)

这个过程启用了PAM（Pluggable Authentication Modules）认证。用户仍然可以通过用户管理手动添加到系统中。
PAM提供了一种机制，通过对用户进行PAM认证来自动将其添加到当前数据库中。为了与普通的Linux密码一起使用，
运行Gitea的用户还必须具有对`/etc/shadow`的读取权限，以便在使用公钥登录时检查账户的有效性。

**注意**:如果用户已将SSH公钥添加到Gitea中，使用这些密钥可能会绕过登录检查系统。因此，
如果您希望禁用使用PAM进行身份验证的用户，应该在Gitea中手动禁用该账户，使用内置的用户管理功能。

1. 配置和安装准备.
    - 建议您创建一个管理用户.
    - 建议取消自动注册.
1. 一旦数据库已初始化完成，使用新创建的管理员账户登录.
1. 导航至用户设置（右上角的图标），然后选择
`Site Administration` -> `Authentication Sources`, 并选择
`Add Authentication Source`.
1. 填写字段如下:
    - 认证类型:`PAM`。
    - 名称:任何有效的值都可以，如果您愿意，可以使用"System Authentication"。
    - PAM服务名称:从/etc/pam.d/目录下选择适用于所需认证的正确文件[^1]。
    - PAM电子邮件域:用户认证时要附加的电子邮件后缀。例如，如果登录系统期望一个名为gituse的用户，
  并且将此字段设置为mail.com，那么Gitea在验证一个GIT实例的用户时将期望user emai字段为gituser@mail.com[^2]。

**Note**: PAM 支持通过[build-time flags](installation/install-from-source.md#build)添加,
而官方提供的二进制文件通常不会默认启用此功能。PAM需要确保系统上有必要的libpam动态库，并且编译器可以访问必要的PAM开发头文件。

[^1]: 例如，在Debian "Bullseye"上使用标准Linux登录，可以使用`common-session-noninteractive`。这个值对于其他版本的Debian，
包括Ubuntu和Mint，可能也是有效的，请查阅您所使用发行版的文档以确认。

[^2]: **PAM的必选项** 请注意:在上面的示例中，用户将作为`gituser`而不是`gituser@mail.com`登录到Gitea的Web界面。

## 简单邮件传输协议(Simple Mail Transfer Protocol,SMTP)

此选项允许 Gitea 以 Gitea 用户身份登录 SMTP 主机。请设置以下字段:

- 身份验证名称 **(必选)**
  - 分配给新授权方法的名称

- SMTP 验证类型 **(必选)**
  - 用于连接 SMTP 主机的验证类型，plain 或 login

- 主机名 **(必选)**

  - SMTP 服务的主机地址
  - 例如:`smtp.mydomain.com`

- 端口号 **(必选)**

  - SMTP 服务的端口号
  - 例如: `587`

- 允许的域名

  - 如果使用公共 SMTP 主机或有多个域的 SMTP 主机，限制哪些域可以登录
    限制哪些域可以登录。
  - 示例: `gitea.io,mydomain.com,mydomain2.com`

- 强制使用 SMTPS
  - 默认情况下将使用SMTPS连接到端口465.如果您希望将smtp用于其他端口，自行设置
  - 否则，如果服务器提供' STARTTLS '扩展名，则将使用此扩展名
- 跳过 TLS 验证
  - 禁用 TLS 验证身份.
- 该认证源处于激活状态
  - 启用或禁用此身份验证源

## FreeIPA

- 要使用 FreeIPA 凭据登录 Gitea，需要为 Gitea 创建一个绑定帐户。
  创建一个绑定帐户:
- 在FreeIPA服务器上创建一个gitea.ldif文件，并将`dc=example,dc=com`替换为您的`dn`，然后提供一个适当安全的密码。

  ```sh
  dn: uid=gitea,cn=sysaccounts,cn=etc,dc=example,dc=com
  changetype: add
  objectclass: account
  objectclass: simplesecurityobject
  uid: gitea
  userPassword: secure password
  passwordExpirationTime: 20380119031407Z
  nsIdleTimeout: 0
  ```

- 导入LDIF文件（如果需要，请将localhost更改为IPA服务器）。系统会提示您输入Directory Manager的密码。:

  ```sh
  ldapmodify -h localhost -p 389 -x -D \
  "cn=Directory Manager" -W -f gitea.ldif
  ```

- 为`gitea_users`添加IPA组:

  ```sh
  ipa group-add --desc="Gitea Users" gitea_users
  ```

- **提示**:对于IPA凭证错误，运行' kinit admin '并提供域管理帐户密码.
- 以管理员身份登录Gitea，点击Admin Panel下的`Authentication`。然后单击`Add New Source`并填写详细信息，更改所有适当的地方。

## SPNEGO with SSPI (Kerberos/NTLM, for Windows only)

Gitea支持通过Windows内置的安全支持提供程序接口（Security Support Provider Interface，SSPI）实现SPNEGO单点登录认证（由RFC4559定义的方案），用于服务器的Web部分。SSPI仅在Windows环境中工作，即当服务器和客户端都在Windows操作系统上运行时。

在激活SSPI单点登录认证（SSO）之前，您需要准备您的环境:

- 在Active Directory中创建一个单独的用户账户，gitea.exe 进程将在该账户下运行（例如，在domain.local域下创建一个名为user的账户:
- 为运行gitea.exe的主机创建一个服务主体名称（Service Principal Name，SPN），其类别为HTTP:

  - 以特权域用户（例如域管理员）的身份启动“命令提示符”或“PowerShell”。
  - 运行下面的命令，将host.domain.local替换为Web应用程序将运行的服务器的完全限定域名（FQDN），将domain\user替换为在前一步中创建的账户名称:

  ```sh
  setspn -A HTTP/host.domain.local domain\user
  ```

在遵循上述步骤之前，请确保您按照以下流程进行操作:

1. 用之前创建的用户登录（如果已经登录，请先注销）。
2. 确保在`custom/conf/app.ini`文件的`[server]`部分中，`ROOT_URL`设置为Web应用程序将运行的服务器的完全限定域名（FQDN），与之前创建服务主体名称时使用的一致（例如，`host.domain.local`）。
3. 启动Web服务器（运行 `gitea.exe web`）。
4. 在 `Site Administration -> Authentication Sources` 中添加一个 `SPNEGO with SSPI` 认证源，以启用SSPI认证。
5. 在域中的客户端计算机上，使用任何域用户登录（与运行`gitea.exe`的服务器不同）。
6. 如果您使用Chrome或Edge浏览器，请将Web应用程序的URL添加到“本地站点”（`Internet选项 -> 安全 -> 本地站点 -> 站点`）。
7. 启动Chrome或Edge浏览器，导航到Gitea的FQDN URL（例如，`http://host.domain.local:3000`）。
8. 在控制面板中点击“Sign In”按钮，然后选择SSPI，将会自动使用当前登录到计算机的用户进行登录。
9. 如果无法正常工作，请确保:
   - 您不是在运行`gitea.exe`的同一台服务器上运行Web浏览器。应该在与服务器不同的域加入计算机（客户端）上运行Web浏览器。如果客户端和服务器都在同一台计算机上运行，则NTLM将优先于Kerberos。
   - 主机上只有一个`HTTP/...`的SPN。
   - SPN中只包含主机名，不包含端口号。
   - 将Web应用程序的URL添加到"本地站点"。
   - 服务器和客户端的时钟差异不超过5分钟（取决于组策略）。
   - 在Internet Explorer中启用了"集成Windows身份验证"（在"高级设置"下）。

遵循这些步骤，您应该能够成功启用和使用SSPI单点登录认证（SSO）。

## 反向代理认证

Gitea 支持通过读取反向代理传递的 HTTP 头中的登录名或者 email 地址来支持反向代理来认证。默认是不启用的，你可以用以下配置启用。

```ini
[service]
ENABLE_REVERSE_PROXY_AUTHENTICATION = true
```

默认的登录用户名的 HTTP 头是 `X-WEBAUTH-USER`，你可以通过修改 `REVERSE_PROXY_AUTHENTICATION_USER` 来变更它。如果用户不存在，可以自动创建用户，当然你需要修改 `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION=true` 来启用它。

默认的登录用户 Email 的 HTTP 头是 `X-WEBAUTH-EMAIL`，你可以通过修改 `REVERSE_PROXY_AUTHENTICATION_EMAIL` 来变更它。如果用户不存在，可以自动创建用户，当然你需要修改 `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION=true` 来启用它。你也可以通过修改 `ENABLE_REVERSE_PROXY_EMAIL` 来启用或停用这个 HTTP 头。

如果设置了 `ENABLE_REVERSE_PROXY_FULL_NAME=true`，则用户的全名会从 `X-WEBAUTH-FULLNAME` 读取，这样在自动创建用户时将使用这个字段作为用户全名，你也可以通过修改 `REVERSE_PROXY_AUTHENTICATION_FULL_NAME` 来变更 HTTP 头。

你也可以通过修改 `REVERSE_PROXY_TRUSTED_PROXIES` 来设置反向代理的IP地址范围，加强安全性，默认值是 `127.0.0.0/8,::1/128`。 通过 `REVERSE_PROXY_LIMIT`， 可以设置最多信任几级反向代理。

注意:反向代理认证不支持认证 API，API 仍旧需要用 access token 来进行认证。
