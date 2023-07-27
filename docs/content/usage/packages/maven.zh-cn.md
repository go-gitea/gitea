---
date: "2021-07-20T00:00:00+00:00"
title: "Maven 软件包注册表"
slug: "maven"
sidebar_position: 60
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Maven"
    sidebar_position: 60
    identifier: "maven"
---

# Maven 软件包注册表

为您的用户或组织发布 [Maven](https://maven.apache.org) 软件包。

## 要求

要使用 Maven 软件包注册表，您可以使用 [Maven](https://maven.apache.org/install.html) 或 [Gradle](https://gradle.org/install/)。
以下示例使用 `Maven` 和 `Gradle Groovy`。

## 配置软件包注册表

要注册软件包注册表，首先需要将访问令牌添加到 [`settings.xml`](https://maven.apache.org/settings.html) 文件中：

```xml
<settings>
  <servers>
    <server>
      <id>gitea</id>
      <configuration>
        <httpHeaders>
          <property>
            <name>Authorization</name>
            <value>token {access_token}</value>
          </property>
        </httpHeaders>
      </configuration>
    </server>
  </servers>
</settings>
```

然后在项目的 `pom.xml` 文件中添加以下部分：

```xml
<repositories>
  <repository>
    <id>gitea</id>
    <url>https://gitea.example.com/api/packages/{owner}/maven</url>
  </repository>
</repositories>
<distributionManagement>
  <repository>
    <id>gitea</id>
    <url>https://gitea.example.com/api/packages/{owner}/maven</url>
  </repository>
  <snapshotRepository>
    <id>gitea</id>
    <url>https://gitea.example.com/api/packages/{owner}/maven</url>
  </snapshotRepository>
</distributionManagement>
```

| 参数           | 描述                                                                                  |
| -------------- | ------------------------------------------------------------------------------------- |
| `access_token` | 您的[个人访问令牌](development/api-usage.md#通过-api-认证) |
| `owner`        | 软件包的所有者                                                                        |

### Gradle variant

如果您计划在项目中添加来自 Gitea 实例的一些软件包，请将其添加到 repositories 部分中：

```groovy
repositories {
    // other repositories
    maven { url "https://gitea.example.com/api/packages/{owner}/maven" }
}
```

在 Groovy gradle 中，您可以在发布部分中包含以下脚本：

```groovy
publishing {
    // 其他发布设置
    repositories {
        maven {
            name = "Gitea"
            url = uri("https://gitea.example.com/api/packages/{owner}/maven")

            credentials(HttpHeaderCredentials) {
                name = "Authorization"
                value = "token {access_token}"
            }

            authentication {
                header(HttpHeaderAuthentication)
            }
        }
    }
}
```

## 发布软件包

要发布软件包，只需运行以下命令：

```shell
mvn deploy
```

或者，如果您使用的是 Gradle，请使用 `gradle` 命令和 `publishAllPublicationsToGiteaRepository` 任务：

```groovy
./gradlew publishAllPublicationsToGiteaRepository
```

如果您想要将预构建的软件包发布到注册表中，可以使用 [`mvn deploy:deploy-file`](https://maven.apache.org/plugins/maven-deploy-plugin/deploy-file-mojo.html) 命令：

```shell
mvn deploy:deploy-file -Durl=https://gitea.example.com/api/packages/{owner}/maven -DrepositoryId=gitea -Dfile=/path/to/package.jar
```

| 参数    | 描述           |
| ------- | -------------- |
| `owner` | 软件包的所有者 |

如果存在相同名称和版本的软件包，您无法发布该软件包。您必须先删除现有的软件包。

## 安装软件包

要从软件包注册表中安装 Maven 软件包，请在项目的 `pom.xml` 文件中添加新的依赖项：

```xml
<dependency>
  <groupId>com.test.package</groupId>
  <artifactId>test_project</artifactId>
  <version>1.0.0</version>
</dependency>
```

在 `Gradle Groovy` 中类似的操作如下：

```groovy
implementation "com.test.package:test_project:1.0.0"
```

然后运行：

```shell
mvn install
```

## 支持的命令

```
mvn install
mvn deploy
mvn dependency:get:
```
