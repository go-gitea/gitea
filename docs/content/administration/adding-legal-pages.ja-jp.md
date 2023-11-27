---
date: "2019-12-28"
title: "法的ページの追加"
slug: adding-legal-pages
sidebar_position: 110
toc: false
draft: false
aliases:
  - /ja-jp/adding-legal-pages
menu:
  sidebar:
    parent: "administration"
    name: "法的ページの追加"
    identifier: "adding-legal-pages"
    sidebar_position: 110
---

一部の法域 (EU など) では、特定の法的ページ (プライバシーポリシーなど) をウェブサイトに追加する必要があります。それらを Gitea インスタンスに追加するには、次の手順に従ってください。

## ページの取得

Gitea のソースコードにはサンプルページが付属しており、`contrib/legal` ディレクトリで入手できます。それらを `custom/public/` にコピーします。たとえば、プライバシーポリシーを追加するには:

```
wget -O /path/to/custom/public/privacy.html https://raw.githubusercontent.com/go-gitea/gitea/main/contrib/legal/privacy.html.sample
```

次に、要件を満たすようにページを編集する必要があります。特に、メールアドレス、ウェブサイトのURL、および「Your Gitea Instance」に関する引用を状況に合わせて変更する必要があります。

Gitea プロジェクトがこのサーバーに対して責任を負っていることを示唆する一般的な ToS またはプライバシーに関する声明を絶対に配置してはなりません。

## 見えるようにする

`/path/to/custom/templates/custom/extra_links_footer.tmpl`に下記の内容を作成または追加します:

```go
<a class="item" href="{{AppSubUrl}}/assets/privacy.html">プライベートポリシー</a>
```

Gtiea サーバーを再起動して、変更を確認します。
