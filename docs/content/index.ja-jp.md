---
date: "2016-11-08T16:00:00+02:00"
title: "ドキュメント"
slug: /
sidebar_position: 10
toc: false
draft: false
---

# Giteaとは?

Gitea は軽量な自己ホスト型のオールインワンソフトウェア開発サービスです。Git ホスティング、コードレビュー、チームコラボレーション、パッケージレジストリ、CI/CD などの機能が含まれています。GitHub、Bitbucket、GitLab に似ています。
Giteaは元々 [Gogs](http://gogs.io) からフォークされたもので、ほぼすべてのコードが変更されました。フォークの理由については、[こちら](https://blog.gitea.com/welcome-to-gitea/)のブログ投稿を参照してください。

## 目的

このプロジェクトの目標は、最も簡単、最も速い、そして最も苦労しない設定方法を提供することです。
セルフホスト型 Git サービスをを提供することです。
The goal of this project is to provide the easiest, fastest, and most painless way of setting
up a self-hosted Git service.

Go を使用することで、プラットフォームに依存せず、**すべてのプラットフォーム** で実行できます。
Go は x86、amd64、ARM、および PowerPC アーキテクチャ上の Linux、macOS、Windows がサポートしています。
[オンラインデモ](https://try.gitea.io/)で試すことができます。

## 機能

- コードホスティング：Gitea はリポジトリの作成と管理、コミット履歴とコードファイルの参照、コードのレビューとマージ、コラボレーターの管理、ブランチの管理などをサポートしています。また、タグ、チェリーピック、フック、統合コラボレーションツールなど、一般的な Git 機能もサポートしています。

- 軽量と速い：Gitea の設計目標の1つは軽量と速い応答速度です。一部大規模なコードホスティングプラットフォームとは異なり、優れたパフォーマンスを発揮し、リソースが限られたサーバー環境に適しています。Gitea は軽量設計のため、リソース消費が比較的低く、リソースに制約のある環境でも良好なパフォーマンスを発揮できます。

- 簡単な構築とメンテナンス: 複雑な構成や依存関係を必要とせずに、さまざまなサーバーに簡単に構築できます。これにより、個人開発者や小規模チームが独自の Git サービスをセットアップして管理することが容易になります。

- セキュリティ：Gitea はセキュリティを重視しており、コードとデータのセキュリティを確保するためのユーザー権限管理、アクセス制御などの機能を提供しています。

- コードレビュー：コードレビューはプルリクエストワークフローと AGit ワークフローの両方をサポートしています。レビュー担当者はオンラインでコードを参照し、レビューコメントやフィードバックを書くことができます。提出者はオンラインでレビューコメントを受けたり、返信したり、コードを変更したりできます。コードレビューは個人や組織がコードの品質向上に役立ちます。

- CI/CD: Gitea Actions は GitHub Actions と互換性がある CI/CD 機能です。ユーザーは使い慣れた YAML 形式のワークフローを作成し、既存のさまざまなアクションプラグインを再利用できます。アクションプラグインは任意のGit ウェブサイトからダウンロードできます。

- プロジェクト管理：Gitea はカンバンやイシューを通じてプロジェクトの要件、機能、バグを追跡できます。イシューはブランチ、タグ、マイルストーン、割り当て、時間追跡、期日、依存関係などの機能をサポートしています。

- アーティファクトリポジトリ: Gitea は Cargo、Chef、Composer、Conan、Conda、Container、Helm、Maven、npm、NuGet、Pub、PyPI、RubyGems、Vagrant などを含む 20 種類を超えるパブリックまたはプライベートのソフトウェアパッケージ管理をサポートしています。

- オープンソースコミュニティサポート: Gitea は MIT ライセンスに基づくオープンソースプロジェクトです。プラットフォームの開発と改善を継続的に行う活発なコミュニティがあります。このプロジェクトはコミュニティの貢献も積極的に歓迎しており、更新と革新を保証します。

- 多言語サポート：Gitea は複数言語のインターフェースを提供し、世界中のユーザーに対応し、国際化とローカリゼーションを促進しています。

その他の機能：詳細については、以下を参照してください：
https://docs.gitea.com/installation/comparison#general-features

## 動作環境

- 小規模なワークロードなら Raspberry Pi 3 でも十分です。
- 通常 2 コア CPU と 1GB メモリは小規模なチーム/プロジェクトには十分です。
- Gitea は UNIX タイプのシステム上で専用の非 root システムアカウントを使用して実行する必要があります。
  - 注: Gitea は `~/.ssh/authorized_keys` ファイルを管理しています。 Gitea を通常のユーザーとして実行すると、そのユーザーがログインできなくなる可能性があります。
- バージョン 2.0.0 以降の [Git](https://git-scm.com/) が必要です。
  - [Git Large File Storage](https://git-lfs.github.com/) が有効になっており、Git バージョンが 2.1.2 以上の場合、利用可能になります。
  - Git バージョンが 2.18 以上の場合、Git コミットグラフレンダリングが自動的に有効になります。

## ブラウザのサポート

- 最新の 2 つのバージョンのChrome、Firefox、SafariとEdge
- Firefox ESR

## コンポーネント

- ウェブサーバーフレームワーク： [Chi](http://github.com/go-chi/chi)
- ORM: [XORM](https://xorm.io)
- UI フレームワーク:
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - その他 (package.jsonを参照)
- エディタ:
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
- データベースドライバ:
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## サードパーティプロジェクト

 [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/)でGitea関連のサードパーティプロジェクトが掲載しています。
