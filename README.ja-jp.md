# Gitea

[![](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml/badge.svg?branch=main)](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml?query=branch%3Amain "Release Nightly")
[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")
[![](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea "Go Report Card")
[![](https://pkg.go.dev/badge/code.gitea.io/gitea?status.svg)](https://pkg.go.dev/code.gitea.io/gitea "GoDoc")
[![](https://img.shields.io/github/release/go-gitea/gitea.svg)](https://github.com/go-gitea/gitea/releases/latest "GitHub release")
[![](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea "Help Contribute to Open Source")
[![](https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen)](https://opencollective.com/gitea "Become a backer/sponsor of gitea")
[![](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT "License: MIT")
[![Contribute with Gitpod](https://img.shields.io/badge/Contribute%20with-Gitpod-908a85?logo=gitpod&color=green)](https://gitpod.io/#https://github.com/go-gitea/gitea)
[![](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com "Crowdin")

[English](./README.md) | [繁體中文](./README.zh-tw.md) | [简体中文](./README.zh-cn.md)

## 目的

本プロジェクトは、セルフホスト型Gitサービスを、より簡単・高速・スムーズに構築できる環境を提供することを目的としています。

GiteaはGo言語で実装されており、Goがサポートする**あらゆる**プラットフォームとアーキテクチャ（Linux、macOS、Windowsのx86、amd64、ARM、PowerPCなど）で動作します。

オンラインデモは[demo.gitea.com](https://demo.gitea.com)でご覧いただけます。

無料のGiteaサービス(リポジトリ数に制限あり)にアクセスするには、[gitea.com](https://gitea.com/user/login)をご利用ください。

Gitea Cloudで専用のGiteaインスタンスを素早くデプロイするには、[cloud.gitea.com](https://cloud.gitea.com)で無料トライアルを開始できます。

## ドキュメント

公式の[ドキュメントサイト](https://docs.gitea.com/)にて、詳細なドキュメントをご確認いただけます。

インストール、管理、使用、開発、貢献ガイドなどが含まれており、すぐに始めてすべての機能を効果的に活用できるようサポートします。

提案や貢献をしたい場合は、[ドキュメントリポジトリ](https://gitea.com/gitea/docs)をご覧ください。

## ビルド

ソースツリーのルートから次のコマンドを実行します:

    TAGS="bindata" make build

SQLiteサポートが必要な場合:

    TAGS="bindata sqlite sqlite_unlock_notify" make build

`build`ターゲットは2つのサブターゲットに分かれています:

- `make backend` - [Go Stable](https://go.dev/dl/)が必要です。必要なバージョンは[go.mod](/go.mod)で定義されています。
- `make frontend` - [Node.js LTS](https://nodejs.org/en/download/)以上と[pnpm](https://pnpm.io/installation)が必要です。

goおよびnpmモジュールをダウンロードするにはインターネット接続が必要です。プリビルドされたフロントエンドファイルを含む公式ソースtarballからビルドする場合、`frontend`ターゲットはトリガーされず、Node.jsなしでビルドできます。

詳細: https://docs.gitea.com/installation/install-from-source

## 使用方法

ビルド後、デフォルトではソースツリーのルートに`gitea`という名前のバイナリファイルが生成されます。実行するには次のコマンドを使用します:

    ./gitea web

> [!NOTE]
> APIの使用に興味がある場合、実験的なサポートと[ドキュメント](https://docs.gitea.com/api)を提供しています。

## 貢献

期待されるワークフロー: Fork -> Patch -> Push -> Pull Request

> [!NOTE]
>
> 1. **プルリクエストの作業を開始する前に、[貢献者ガイド](CONTRIBUTING.md)を必ずお読みください。**
> 2. プロジェクトの脆弱性を発見した場合は、**security@gitea.io**に非公開でご連絡ください。ありがとうございます!

## 翻訳

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com)

翻訳は[Crowdin](https://translate.gitea.com)を通じて行われます。新しい言語に翻訳したい場合は、Crowdinプロジェクトのマネージャーに新しい言語の追加を依頼してください。

言語追加のためのissueを作成したり、Discordの#translationチャンネルで質問することもできます。コンテキストが必要な場合や翻訳の問題を見つけた場合は、文字列にコメントを残すか、Discordで質問できます。一般的な翻訳に関する質問については、ドキュメントにセクションがあります。現在は少し空白ですが、質問が出てくるにつれて充実させていく予定です。

詳細は[ドキュメント](https://docs.gitea.com/contributing/localization)をご覧ください。

## 公式および第三者プロジェクト

公式の[go-sdk](https://gitea.com/gitea/go-sdk)、[tea](https://gitea.com/gitea/tea)というCLIツール、そしてGitea Action用の[action runner](https://gitea.com/gitea/act_runner)を提供しています。

[gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea)でGitea関連プロジェクトのリストを管理しており、SDK、プラグイン、テーマなど、より多くの第三者プロジェクトを発見できます。

## コミュニケーション

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")

[ドキュメント](https://docs.gitea.com/)でカバーされていない質問がある場合は、[Discordサーバー](https://discord.gg/Gitea)で連絡を取るか、[discourseフォーラム](https://forum.gitea.com/)に投稿してください。

## 作者

- [メンテナー](https://github.com/orgs/go-gitea/people)
- [コントリビューター](https://github.com/go-gitea/gitea/graphs/contributors)
- [翻訳者](options/locale/TRANSLATORS)

## バッカー

すべてのバッカーの皆様に感謝します! 🙏 [[バッカーになる](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## スポンサー

スポンサーになってこのプロジェクトを支援してください。あなたのロゴがここに表示され、ウェブサイトへのリンクが付きます。[[スポンサーになる](https://opencollective.com/gitea#sponsor)]

<a href="https://opencollective.com/gitea/sponsor/0/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/1/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/2/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/3/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/4/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/5/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/6/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/7/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/8/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/9/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/9/avatar.svg"></a>

## FAQ

**Giteaの発音は?**

Giteaは[/ɡɪ'ti:/](https://youtu.be/EM71-2uDAoY)と発音され、「ギッティー」のように聞こえます。gは濁音です。

**なぜこれはGiteaインスタンスでホストされていないのですか?**

[取り組んでいます](https://github.com/go-gitea/gitea/issues/1029)。

**セキュリティパッチはどこで見つけられますか?**

[リリースログ](https://github.com/go-gitea/gitea/releases)または[変更ログ](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md)で、キーワード`SECURITY`を検索してセキュリティパッチを見つけてください。

## ライセンス

このプロジェクトはMITライセンスの下でライセンスされています。
完全なライセンステキストについては、[LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE)ファイルをご覧ください。

## 詳細情報

<details>
<summary>インターフェースの概要をお探しですか?こちらをご覧ください!</summary>

### ログイン/登録ページ

![Login](https://dl.gitea.com/screenshots/login.png)
![Register](https://dl.gitea.com/screenshots/register.png)

### ユーザーダッシュボード

![Home](https://dl.gitea.com/screenshots/home.png)
![Issues](https://dl.gitea.com/screenshots/issues.png)
![Pull Requests](https://dl.gitea.com/screenshots/pull_requests.png)
![Milestones](https://dl.gitea.com/screenshots/milestones.png)

### ユーザープロフィール

![Profile](https://dl.gitea.com/screenshots/user_profile.png)

### 探索

![Repos](https://dl.gitea.com/screenshots/explore_repos.png)
![Users](https://dl.gitea.com/screenshots/explore_users.png)
![Orgs](https://dl.gitea.com/screenshots/explore_orgs.png)

### リポジトリ

![Home](https://dl.gitea.com/screenshots/repo_home.png)
![Commits](https://dl.gitea.com/screenshots/repo_commits.png)
![Branches](https://dl.gitea.com/screenshots/repo_branches.png)
![Labels](https://dl.gitea.com/screenshots/repo_labels.png)
![Milestones](https://dl.gitea.com/screenshots/repo_milestones.png)
![Releases](https://dl.gitea.com/screenshots/repo_releases.png)
![Tags](https://dl.gitea.com/screenshots/repo_tags.png)

#### リポジトリのIssue

![List](https://dl.gitea.com/screenshots/repo_issues.png)
![Issue](https://dl.gitea.com/screenshots/repo_issue.png)

#### リポジトリのプルリクエスト

![List](https://dl.gitea.com/screenshots/repo_pull_requests.png)
![Pull Request](https://dl.gitea.com/screenshots/repo_pull_request.png)
![File](https://dl.gitea.com/screenshots/repo_pull_request_file.png)
![Commits](https://dl.gitea.com/screenshots/repo_pull_request_commits.png)

#### リポジトリのActions

![List](https://dl.gitea.com/screenshots/repo_actions.png)
![Details](https://dl.gitea.com/screenshots/repo_actions_run.png)

#### リポジトリのアクティビティ

![Activity](https://dl.gitea.com/screenshots/repo_activity.png)
![Contributors](https://dl.gitea.com/screenshots/repo_contributors.png)
![Code Frequency](https://dl.gitea.com/screenshots/repo_code_frequency.png)
![Recent Commits](https://dl.gitea.com/screenshots/repo_recent_commits.png)

### 組織

![Home](https://dl.gitea.com/screenshots/org_home.png)

</details>
