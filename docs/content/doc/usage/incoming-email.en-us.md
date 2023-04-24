---
date: "2022-12-01T00:00:00+00:00"
title: "Incoming Email"
slug: "incoming-email"
weight: 13
draft: false
toc: false
menu:
  sidebar:
    parent: "usage"
    name: "Incoming Email"
    weight: 13
    identifier: "incoming-email"
---

# Incoming Email

Gitea supports the execution of several actions through incoming mails. This page describes how to set this up.

**Table of Contents**

{{< toc >}}

## Requirements

Handling incoming email messages requires an IMAP-enabled email account.
The recommended strategy is to use [email sub-addressing](https://en.wikipedia.org/wiki/Email_address#Sub-addressing) but a catch-all mailbox does work too.
The receiving email address contains a user/action specific token which tells Gitea which action should be performed.
This token is expected in the `To` and `Delivered-To` header fields.

Gitea tries to detect automatic responses to skip and the email server should be configured to reduce the incoming noise too (spam, newsletter).

## Configuration

To activate the handling of incoming email messages you have to configure the `email.incoming` section in the configuration file.

The `REPLY_TO_ADDRESS` contains the address an email client will respond to.
This address needs to contain the `%{token}` placeholder which will be replaced with a token describing the user/action.
This placeholder must only appear once in the address and must be in the user part of the address (before the `@`).

An example using email sub-addressing may look like this: `incoming+%{token}@example.com`

If a catch-all mailbox is used, the placeholder may be used anywhere in the user part of the address: `incoming+%{token}@example.com`, `incoming_%{token}@example.com`, `%{token}@example.com`

## Security

Be careful when choosing the domain used for receiving incoming email.
It's recommended receiving incoming email on a subdomain, such as `incoming.example.com` to prevent potential security problems with other services running on `example.com`.
