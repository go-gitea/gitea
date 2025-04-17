# گیتی

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

## هدف

هدف این پروژه، ساختن آسان‌ترین، سریع‌ترین و کم‌دردسرترین راه برای ارائهٔ خدمت خودمیزبانی گیت است.

گیتی به زبان گو نوشته شده و در **تمام** سکوها و معماری‌هایی که تحت پشتیبانی گو هستند، مثل گنو+لینوکس، مک‌اواس و ویندوز بر روی معماری‌های ۳۲بیتی، ۶۴بیتی، آرم و پاورپی‌سی پشتیبانی می‌شود.
این پروژه، از پروژهٔ [Gogs](https://gogs.io)، [انشعاب گرفته شده](https://blog.gitea.com/welcome-to-gitea/) و پس از آن، به شدّت تغییر کرده است.

برای نمونهٔ برخط، [demo.gitea.com](https://demo.gitea.com) را مشاهده کنید.

برای دسترسی به خدمت رایگان گیتی (با تعداد محدودی مخزن) می‌توانید به [gitea.com](https://gitea.com/user/login) مراجعه کنید.

برای استقرار سریع مخزن گیتی اختصاصی شما بر روی فضای ابری گیتی، [cloud.gitea.com](https://cloud.gitea.com) را ببینید.

## مستندات

شما می‌توانید مستندات جامع ما را در [وبگاه رسمی مستندات گیتی](https://docs.gitea.com/)، دنبال کنید.

مستندات ما، شامل نصب، مدیریت، استفاده، توسعه، راهنمای مشارکت و غیره است تا به شما در شروع گشت‌وگذار تمامی ویژگی‌ها، به طور مؤثر، کمک کند.

اگر پیشنهادی در این رابطه دارید و یا می‌خواهید که به مستندات، مشارکت کنید، [مخزن مستندات](https://gitea.com/gitea/docs) را پیگیری کنید.

## ساختن

از شاخهٔ ریشهٔ کدمنبع، دستور زیر را اجرا کنید:

    TAGS="bindata" make build

یا اگر پشتیبانی از SQLite را نیاز دارید، دستور زیر را اجرا کنید:

    TAGS="bindata sqlite sqlite_unlock_notify" make build

هدف ساختن، به دو زیرهدف تقسیم شده است:

- ‏`make backend` که نیازمند [گولنگ پایدار](https://go.dev/dl/) است. نسخهٔ مورد نیاز، در پروندهٔ [go.mod](/go.mod) تعریف شده است.
- ‏`make frontend` که نیازمند [نود جی‌اس LTS](https://nodejs.org/en/download/) یا بالاتر است.

اتّصال اینترنتی برای بارگیری پیمانه‌های گو و npm، مورد نیاز است. هنگام ساختن از بایگانی بان‌های رسمی، که پرونده‌های ساخته شدهٔ فرانت‌اند را داراست، هدف `frontend` اجرا نمی‌شود و ساختن می‌تواند بدون نیاز به نود جی‌اس، انجام شود.

اطّلاعات بیشتر: https://docs.gitea.com/installation/install-from-source

## استفاده

بعد از ساختن، به طور پیش‌فرض، یک پروندهٔ دوگانی به نام `gitea` در ریشهٔ درخت کدمنبع، ساخته می‌شود. برای اجرای آن، از دستور

    ./gitea web

استفاده کنید.

> [نکته]
> 
> اگر به استفاده از APIهای ما علاقه‌مندید، ما پشتیبانی تجربی را در [مستندات مربوطه](https://docs.gitea.com/api)، شرح داده‌ایم.

## مشارکت

روال مورد انتظار، انشعاب‌گیری، وصله کردن، اعمال و بعد درخواست ادغام است.

> [نکته]
>
> ۱. **شما باید [راهنمای مشارکت](CONTRIBUTING.md) را قبل از کار کردن بر روی یک درخواست ادغام، مطالعه کنید.**
> ۲. اگر شما یک آسیب‌پذیری در پروژه یافتید، لطفاً به شکل محرمانه آن را به **security@gitea.io**، بفرستید. متشکریم!

## ترجمه

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com)

ترجمه‌ها، از طریق سکّوی [Crowdin](https://translate.gitea.com) انجام می‌شوند. اگر می‌خواهید که به زبان جدیدی ترجمه کنید، از یکی از مدیران پروژه در Crowdin، بخواهید که زبان جدیدی به این سکّو، اضافه کند.

شما همچنین می‌توانید که یک مشکل جدید ایجاد کنید و یا در بخش #translation در دیسکورد، درخواست افزودن زبان جدید را، مطرح کنید. اگر به مشکلات محتوایی یا مشکل در ترجمه برخوردید، می‌توانید بر روی همان رشته نظر خود را قرار داده و یا در دیسکورد، مطرح نمایید. برای سؤالات عمومی در رابطه با ترجمه، یک بخش در مستندات وجود دارد ولی در حال حاضر، تکمیل نشده و ما امیدواریم که در آینده با افزودن سؤالات جدید مشارکت‌کنندگان به آن، آن بخش را تکمیل کنیم.

برای اطّلاعات بیشتر در بحث ترجمه، [مستندات](https://docs.gitea.com/contributing/localization) ما را ببینید.

## پروژه‌های رسمی و شخص ثالث

ما [رابط کاربری توسعهٔ گولنگ‌محور](https://gitea.com/gitea/go-sdk)، یک ابزار رابط کاربری خط فرمان به نام [tea](https://gitea.com/gitea/tea) و همچنین یک [اجرا کنندهٔ «کنش گیتی»](https://gitea.com/gitea/act_runner) برای کنش‌های گیتی داریم.

ما لیستی از پروژه‌های مرتبط با گیتی را در مخزن [gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea) نگه‌داری می‌کنیم. در آنجا شما می‌توانید پروژه‌های شخص ثالث بیشتری مثل رابط کاربری توسعه، افزونه‌ها و زمینه‌ها را بیابید.

## ارتباط

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")

اگر هر سؤالی دارید که در [مستندات](https://docs.gitea.com/) پوشش داده نشده، با ما در [دیسکورد](https://discord.gg/Gitea) ارتباط بگیرید و یا یک فرسته در [تالار گفتگوی دیسکورس ما](https://forum.gitea.com/) بفرستید.

## سازندگان

- [نگه‌دارندگان](https://github.com/orgs/go-gitea/people)
- [مشارکت‌کنندگان](https://github.com/go-gitea/gitea/graphs/contributors)
- [مترجمان](options/locale/TRANSLATORS)

## پشتیبان‌های مالی

از تمامی پشتیبان‌های مالی‌مان تشکّر می‌کنیم! 🙏 [پشتیبان ما شوید!](https://opencollective.com/gitea#backer)

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## حامیان مالی

با حامی شدن، پروژهٔ گیتی را حمایت کنید. در اینجا، نماد شما به همراه پیوندی به وبگاه شما، نمایان می‌شود. [حامی شوید!](https://opencollective.com/gitea#sponsor)

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

## سؤالات پرتکرار

**گیتی را چگونه تلفّظ می‌کنید؟**

گیتی به شکل [/ɡɪ’ti:/](https://youtu.be/EM71-2uDAoY) تلفّظ می‌شود.

**چرا خود این پروژه بر روی یک نمونهٔ گیتی، میزبانی نمی‌شود؟**

ما مشغول [کار بر روی آن](https://github.com/go-gitea/gitea/issues/1029)، هستیم.

**کجا می‌توانم وصله‌های امنیتی را بیابم؟**

در [گزارش انتشار](https://github.com/go-gitea/gitea/releases)، [گزارش تغییرات](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md) و یا با جستجو کردن کلیدواژهٔ `SECURITY`، می‌توانید وصله‌های امنیتی را پیدا کنید.

## پروانه

این پروژه تحت پروانهٔ MIT ارائه می‌شود.
پروندهٔ [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) را برای متن کامل پروانه، مشاهده کنید.

## اطّلاعات بیشتر

<details>
<summary>‏به دنبال تصاویری از رابط کاربری گیتی هستید؟ این بخش را ببینید!</summary>

### صفحهٔ ثبت‌نام و ورود

![Login](https://dl.gitea.com/screenshots/login.png)
![Register](https://dl.gitea.com/screenshots/register.png)

### پنل کاربری

![Home](https://dl.gitea.com/screenshots/home.png)
![Issues](https://dl.gitea.com/screenshots/issues.png)
![Pull Requests](https://dl.gitea.com/screenshots/pull_requests.png)
![Milestones](https://dl.gitea.com/screenshots/milestones.png)

### نمایهٔ کاربر

![Profile](https://dl.gitea.com/screenshots/user_profile.png)

### کاوش

![Repos](https://dl.gitea.com/screenshots/explore_repos.png)
![Users](https://dl.gitea.com/screenshots/explore_users.png)
![Orgs](https://dl.gitea.com/screenshots/explore_orgs.png)

### مخزن

![Home](https://dl.gitea.com/screenshots/repo_home.png)
![Commits](https://dl.gitea.com/screenshots/repo_commits.png)
![Branches](https://dl.gitea.com/screenshots/repo_branches.png)
![Labels](https://dl.gitea.com/screenshots/repo_labels.png)
![Milestones](https://dl.gitea.com/screenshots/repo_milestones.png)
![Releases](https://dl.gitea.com/screenshots/repo_releases.png)
![Tags](https://dl.gitea.com/screenshots/repo_tags.png)

#### مشکلات مخزن

![List](https://dl.gitea.com/screenshots/repo_issues.png)
![Issue](https://dl.gitea.com/screenshots/repo_issue.png)

#### درخواست‌های ادغام مخزن

![List](https://dl.gitea.com/screenshots/repo_pull_requests.png)
![Pull Request](https://dl.gitea.com/screenshots/repo_pull_request.png)
![File](https://dl.gitea.com/screenshots/repo_pull_request_file.png)
![Commits](https://dl.gitea.com/screenshots/repo_pull_request_commits.png)

#### کنش‌های مخزن

![List](https://dl.gitea.com/screenshots/repo_actions.png)
![Details](https://dl.gitea.com/screenshots/repo_actions_run.png)

#### فعّالیّت‌های مخزن

![Activity](https://dl.gitea.com/screenshots/repo_activity.png)
![Contributors](https://dl.gitea.com/screenshots/repo_contributors.png)
![Code Frequency](https://dl.gitea.com/screenshots/repo_code_frequency.png)
![Recent Commits](https://dl.gitea.com/screenshots/repo_recent_commits.png)

### سازمان

![Home](https://dl.gitea.com/screenshots/org_home.png)

</details>
