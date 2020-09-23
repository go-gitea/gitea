# Changes from 5.0 to 6.0

See [breaking changes](https://www.elastic.co/guide/en/elasticsearch/reference/master/breaking-changes-6.0.html).

## _all removed

6.0 has removed support for the `_all` field.

## Boolean values coerced

Only use `true` or `false` for boolean values, not `0` or `1` or `on` or `off`.

## Single Type Indices

Notice that 6.0 and future versions will default to single type indices, i.e. you may not use multiple types when e.g. adding an index with a mapping.

See [here for details](https://www.elastic.co/guide/en/elasticsearch/reference/6.7/removal-of-types.html#_what_are_mapping_types).

