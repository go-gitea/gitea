---
date: "2016-12-01T16:00:00+02:00"
title: "Localization"
slug: "localization"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Localization"
    weight: 20
    identifier: "localization"
---

# Localization

Gitea's localization happens through our [Crowdin project](https://crowdin.com/project/gitea).

For changes to an **English** translation, a pull request can be made that changes the appropriate key in
the [english locale](https://github.com/go-gitea/gitea/blob/master/options/locale/locale_en-US.ini).

For changes to a **non-English** translation, refer to the Crowdin project above.

## Supported Languages

Any language listed in the above Crowdin project will be supported as long as 25% or more has been translated.

After a translation has been accepted, it will be reflected in the main repository after the next Crowdin sync, which is generally after any PR is merged.  
At the time of writing, this means that a changed translation may not appear until the following Gitea release.  
If you use a bleeding edge build, it should appear as soon as you update after the change is synced.

## Plurals

Prior to version 1.18, Gitea handled plurals using the .TrN function which has some
built in rules for managing plurals but is unable to properly all languages.

From 1.18 we will migrate to use the CLDR formulation.

Translation keys which handle plurals will be marked with a `_plural` suffix with
additional suffices as per the CLDR form (which the exception that the `Other` form is
left unsuffixed), e.g.

```ini
form.reach_limit_of_creation_plural = You have already reached your limit of %d repositories.
form.reach_limit_of_creation_plural_one = You have already reached your limit of %d repository.
```

These keys should be used with the `.TrPlural` function. (Ordinals are also handled with `.TrOrdinal`.)

Each language has different numbers of plural forms, in English (and a number of other
languages) there are two plural forms:

* `_plural_one`: This matches the singular form. (CLDR `One`.)
* `_plural`: This matches the plural form. (CLDR `Other`.)

Other languages, e.g. Mandarin, have no plural forms, and others many more.

The possible suffices and forms are:

* `_plural`: CLDR `Other` - the most common form and will often match to standard plural form.
* `_plural_zero`: CLDR `Zero` - matches a zeroth form, which in Latvian would match the form used for 10-20, 30 and so on.
* `_plural_one`: CLDR `One` - matches the singular form in English, but in Latvian matches the form used for 1, 21, 31 and so on.
* `_plural_two`: CLDR `Two` - matches the dual form used in for example Arabic for 2 items, but also more complexly in Celtic languages.
* `_plural_few`: CLDR `Few` - matches the form used in Arabic for 3-10, 103-110, and the ternary form in Celtic languages. In Russian and Ukranian for 2-4, 22-24.
* `_plural_many`: CLDR `Many` - matches the form used for large numbers in romance lanaguages like French, e.g. 1 000 000 *de* chat*s*, but in Russian and Ukranian it handles 0, 5~19, 100, 1000 and so on.

Some plural forms are only relevant if the object being counted is of a certain
grammatical gender or in certain tenses. If a language suggests a non-Other form e.g.
Two but the `_plural_two` key is absent - the system will fall back to the `_plural`
form.

Translators may want to review the CLDR information for their language or look at
`modules/translation/i18n/plurals/generate/plurals.xml`.

Ordinal forms, e.g. 1st, 2nd, 3rd and so on can be handled with `.TrOrdinal`. These
have the same forms as the plural forms, and we will use `_ordinal` as a base suffix
in future.
