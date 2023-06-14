---
date: "2023-05-25T16:00:00+02:00"
title: "前端开发指南"
slug: "guidelines-frontend"
weight: 20
toc: false
draft: false
aliases:
  - /zh-cn/guidelines-frontend
menu:
  sidebar:
    parent: "contributing"
    name: "前端开发指南"
    weight: 20
    identifier: "guidelines-frontend"
---

# 前端开发指南

**目录**

{{< toc >}}

## 背景

Gitea 在其前端中使用[Fomantic-UI](https://fomantic-ui.com/introduction/getting-started.html)（基于[jQuery](https://api.jquery.com)）和 [Vue3](https://vuejs.org/)。

HTML 页面由[Go HTML Template](https://pkg.go.dev/html/template)渲染。

源文件可以在以下目录中找到：

* **CSS 样式**： `web_src/css/`
* **JavaScript 文件**： `web_src/js/`
* **Vue 组件**： `web_src/js/components/`
* **Go HTML 模板**： `templates/`

## 通用准则

我们推荐使用[Google HTML/CSS Style Guide](https://google.github.io/styleguide/htmlcssguide.html)和[Google JavaScript Style Guide](https://google.github.io/styleguide/jsguide.html)。

## Gitea 特定准则：

1. 每个功能（Fomantic-UI/jQuery 模块）应放在单独的文件/目录中。
2. HTML 的 id 和 class 应使用 kebab-case，最好包含2-3个与功能相关的关键词。
3. 在 JavaScript 中使用的 HTML 的 id 和 class 应在整个项目中是唯一的，并且应包含2-3个与功能相关的关键词。建议在仅在 JavaScript 中使用的 class 中使用 `js-` 前缀。
4. 不应覆盖框架提供的 class 的 CSS 样式。始终使用具有2-3个与功能相关的关键词的新 class 名称来覆盖框架样式。Gitea 中的帮助 CSS 类在 `helpers.less` 中。
5. 后端可以通过使用`ctx.PageData["myModuleData"] = map[]{}`将复杂数据传递给前端，但不要将整个模型暴露给前端，以避免泄露敏感数据。
6. 简单页面和与 SEO 相关的页面使用 Go HTML 模板渲染生成静态的 Fomantic-UI HTML 输出。复杂页面可以使用 Vue3。
7. 明确变量类型，优先使用`elem.disabled = true`而不是`elem.setAttribute('disabled', 'anything')`，优先使用`$el.prop('checked', var === 'yes')`而不是`$el.prop('checked', var)`。
8. 使用语义化元素，优先使用`<button class="ui button">`而不是`<div class="ui button">`。
9. 避免在 CSS 中使用不必要的`!important`，如果无法避免，添加注释解释为什么需要它。
10. 避免在一个事件监听器中混合不同的事件，优先为每个事件使用独立的事件监听器。
11. 推荐使用自定义事件名称前缀`ce-`。
12. Gitea 的 tailwind-style CSS 类使用`gt-`前缀（`gt-relative`），而 Gitea 自身的私有框架级 CSS 类使用`g-`前缀（`g-modal-confirm`）。

### 可访问性 / ARIA

在历史上，Gitea大量使用了可访问性不友好的框架 Fomantic UI。
Gitea使用一些补丁使Fomantic UI更具可访问性（参见`aria.js`和`aria.md`），
但仍然存在许多问题需要大量的工作和时间来修复。

### 框架使用

不建议混合使用不同的框架，这会使代码难以维护。
一个 JavaScript 模块应遵循一个主要框架，并遵循该框架的最佳实践。

推荐的实现方式：

* Vue + Vanilla JS
* Fomantic-UI（jQuery）
* Vanilla JS

不推荐的实现方式：

* Vue + Fomantic-UI（jQuery）
* jQuery + Vanilla JS

为了保持界面一致，Vue 组件可以使用 Fomantic-UI 的 CSS 类。
尽管不建议混合使用不同的框架，
但如果混合使用是必要的，并且代码设计良好且易于维护，也可以工作。

### async 函数

只有当函数内部存在`await`调用或返回`Promise`时，才将函数标记为`async`。

不建议使用`async`事件监听器，这可能会导致问题。
原因是`await`后的代码在事件分发之外执行。
参考：https://github.com/github/eslint-plugin-github/blob/main/docs/rules/async-preventdefault.md

如果一个事件监听器必须是`async`，应在任何`await`之前使用`e.preventDefault()`，
建议将其放在函数的开头。

如果我们想在非异步上下文中调用`async`函数，
建议使用`const _promise = asyncFoo()`来告诉读者
这是有意为之的，我们想调用异步函数并忽略Promise。
一些 lint 规则和 IDE 也会在未处理返回的 Promise 时发出警告。

### HTML 属性和 dataset

禁止使用`dataset`，它的驼峰命名行为使得搜索属性变得困难。
然而，仍然存在一些特殊情况，因此当前的准则是：

* 对于旧代码：
  * 应将`$.data()`重构为`$.attr()`。
  * 在极少数情况下，可以使用`$.data()`将一些非字符串数据绑定到元素上，但强烈不推荐使用。

* 对于新代码：
  * 不应使用`node.dataset`，而应使用`node.getAttribute`。
  * 不要将任何用户数据绑定到 DOM 节点上，使用合适的设计模式描述节点和数据之间的关系。

### 显示/隐藏元素

* 推荐在Vue组件中使用`v-if`和`v-show`来显示/隐藏元素。
* Go 模板代码应使用 Gitea 的 `.gt-hidden` 和 `showElem()/hideElem()/toggleElem()` 来显示/隐藏元素，请参阅`.gt-hidden`的注释以获取更多详细信息。

### Go HTML 模板中的样式和属性

建议使用以下方式：

```html
<div class="gt-name1 gt-name2 {{if .IsFoo}}gt-foo{{end}}" {{if .IsFoo}}data-foo{{end}}></div>
```

而不是：

```html
<div class="gt-name1 gt-name2{{if .IsFoo}} gt-foo{{end}}"{{if .IsFoo}} data-foo{{end}}></div>
```

以使代码更易读。

### 旧代码

许多旧代码已经存在于本文撰写之前。建议重构旧代码以遵循指南。

### Vue3 和 JSX

Gitea 现在正在使用 Vue3。我们决定不引入 JSX，以保持 HTML 代码和 JavaScript 代码分离。
