# i18n [![GoDoc](https://godoc.org/github.com/unknwon/i18n?status.svg)](https://godoc.org/github.com/unknwon/i18n)

Package i18n is for app Internationalization and Localization.

## Introduction

This package provides multiple-language options to improve user experience. Sites like [Go Walker](http://gowalker.org) and [gogs.io](http://gogs.io) are using this module to implement Chinese and English user interfaces.

You can use following command to install this module:

    go get github.com/Unknwon/i18n

## Usage

First of all, you have to import this package:

```go
import "github.com/unknwon/i18n"
```

The format of locale files is very like INI format configuration file, which is basically key-value pairs. But this module has some improvements. Every language corresponding to a locale file, for example, suppose there are two files called `locale_en-US.ini` and `locale_zh-CN.ini`.

The name and extensions of locale files can be anything, but we strongly recommend you to follow the style of gogsweb.

## Minimal example

Here are two simplest locale file examples:

File `locale_en-US.ini`:

```ini
hi = hello, %s
bye = goodbye
```

File `locale_zh-CN.ini`:

```ini
hi = 您好，%s
bye = 再见
```

### Do Translation

There are two ways to do translation depends on which way is the best fit for your application or framework.

Directly use package function to translate:

```go
i18n.Tr("en-US", "hi", "Unknwon")
i18n.Tr("en-US", "bye")
```

Or create a struct and embed it:

```go
type MyController struct{
    // ...other fields
    i18n.Locale
}

//...

func ... {
    c := &MyController{
        Locale: i18n.Locale{"en-US"},
    }
    _ = c.Tr("hi", "Unknwon")
    _ = c.Tr("bye")
}
```

Code above will produce correspondingly:

- English `en-US`：`hello, Unknwon`, `goodbye`
- Chinese `zh-CN`：`您好，Unknwon`, `再见`

## Section

For different pages, one key may map to different values. Therefore, i18n module also uses the section feature of INI format configuration to achieve section.

For example, the key name is `about`, and we want to show `About` in the home page and `About Us` in about page. Then you can do following:

Content in locale file:

```ini
about = About

[about]
about = About Us
```

Get `about` in home page:

```go
i18n.Tr("en-US", "about")
```

Get `about` in about page:

```go
i18n.Tr("en-US", "about.about")
```

### Ambiguity

Because dot `.` is sign of section in both [INI parser](https://github.com/go-ini/ini) and locale files, so when your key name contains `.` will cause ambiguity. At this point, you just need to add one more `.` in front of the key.

For example, the key name is `about.`, then we can use:

```go
i18n.Tr("en-US", ".about.")
```

to get expect result.

## Helper tool

Module i18n provides a command line helper tool beei18n for simplify steps of your development. You can install it as follows:

	go get github.com/unknwon/i18n/ui18n

### Sync locale files

Command `sync` allows you use a exist local file as the template to create or sync other locale files:

	ui18n sync srouce_file.ini other1.ini other2.ini

This command can operate 1 or more files in one command.

## More information

- The first locale you load to the module is considered as **default locale**.
- When matching non-default locale and didn't find the string, i18n will have a second try on default locale.
- If i18n still cannot find string in the default locale, raw string will be returned. For instance, when the string is `hi` and it does not exist in locale file, simply return `hi` as output.
