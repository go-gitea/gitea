# Changes from 6.0 to 7.0

See [breaking changes](https://www.elastic.co/guide/en/elasticsearch/reference/7.x/breaking-changes-7.0.html).

## SearchHit.Source changed from `*json.RawMessage` to `json.RawMessage`

The `SearchHit` structure changed from

```
// SearchHit is a single hit.
type SearchHit struct {
	...
	Source         *json.RawMessage               `json:"_source,omitempty"`         // stored document source
	...
}
```

to

```
// SearchHit is a single hit.
type SearchHit struct {
	...
	Source         json.RawMessage                `json:"_source,omitempty"`         // stored document source
	...
}
```

As `json.RawMessage` is a `[]byte`, there is no need to specify it
as `*json.RawMessage` as `json.RawMessage` is perfectly ok to represent
a `nil` value.

So when deserializing the search hits, you need to change your code from:

```
for _, hit := range searchResult.Hits.Hits {
    var doc Doc
    err := json.Unmarshal(*hit.Source, &doc) // notice the * here
    if err != nil {
        // Deserialization failed
    }
}
```

to

```
for _, hit := range searchResult.Hits.Hits {
    var doc Doc
    err := json.Unmarshal(hit.Source, &doc) // it's missing here
    if err != nil {
        // Deserialization failed
    }
}
```
