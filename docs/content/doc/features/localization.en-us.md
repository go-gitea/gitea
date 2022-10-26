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

Prior to version 1.19, Gitea handled plurals using the .TrN function which has some
built in rules for managing plurals but is unable to properly all languages.

From 1.19 we will migrate to use the CLDR formulation.

Translation keys which handle plurals should be marked with a `_plural` suffix. This
will allow autogeneration of the various forms using go templates, e.g.

```ini
form.reach_limit_of_creation_plural = You have already reached your limit of %d {{if .One}}repository{{else}}repositories{{end}}.
```

Only the `form` is provided to this template - not the operand value itself. This is to allow autogeneration of the forms at time of loading.

These will be compiled to various different keys based on the available formats in the
language. e.g. assuming the above key is in the English locale it becomes:

```ini
form.reach_limit_of_creation_plural_one = You have already reached your limit of %d repository.
form.reach_limit_of_creation_plural_other = You have already reached your limit of %d repositories.

```

If the template format is too cumbersome forms can be directly created and they will
be used in preference to the template generated variants.

These keys should be used with the `.TrPlural` function. (Ordinals can be handled with `.TrOrdinal`.)

Each language has different numbers of plural forms, in English (and a number of other
languages) there are two plural forms:

* `One`: This matches the singular form.
* `Other`: This matches the plural form.

Other languages, e.g. Mandarin, have no plural forms, and others many more.

The possible forms are:

* `Other` - the most common form and will often match to standard plural form.
* `Zero` - matches a zeroth form, which in Latvian would match the form used for 10-20, 30 and so on.
* `One` - matches the singular form in English, but in Latvian matches the form used for 1, 21, 31 and so on.
* `Two` - matches the dual form used in for example Arabic for 2 items, but also more complexly in Celtic languages.
* `Few` - matches the form used in Arabic for 3-10, 103-110, and the ternary form in Celtic languages. In Russian and Ukranian for 2-4, 22-24.
* `Many` - matches the form used for large numbers in romance lanaguages like French, e.g. 1 000 000 *de* chat*s*, but in Russian and Ukranian it handles 0, 5~19, 100, 1000 and so on.

Some plural forms are only relevant if the object being counted is of a certain
grammatical gender or in certain tenses. Write your translation template appropriately to take account of this using `not` or `and` as appropriately.

Translators may want to review the CLDR information for their language or look at
`modules/translation/i18n/plurals/generate/plurals.xml`.

Ordinal forms, e.g. 1st, 2nd, 3rd and so on can be handled with `.TrOrdinal`. These
have the same forms as the plural forms, and we will use `_ordinal` as a base suffix
in future.

### Technical details

The following is technical and is provided to aid understanding in cases of problems only. Only use `.TrPlural` (and `.TrOrdinal`) with translation keys that have the suffix `_plural` (or `_ordinal`.) If you do not the specific per plural forms must be provided explicitly in the locale file. In this case keys for plural forms will be searched for in the following hierarchy:

1. `${key}_${form}` in the locale.
2. `${key}_other` in the locale.
3. `${key}` in the locale.
4. `${key}_${form}` in the default locale.
5. `${key}_other` in the default locale.
6. `${key}` in the default locale.
7. Use the string `${key}_${form}` directly as the format.

You do not have to worry about this if the key has the `_plural` (or `_ordinal`) suffix as the correct keys will be created automatically.
