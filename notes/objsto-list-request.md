# Request: Add List Method to objsto

## Summary

Add a `List` method to the `objsto` package that returns objects matching a prefix.

## Current State

The `objsto.Client` currently supports:
- `Get(ctx, object)` - get an object
- `Put(ctx, object, reader)` - put an object

## Proposed Addition

```go
// List returns object keys matching the given prefix.
func (c *Client) List(ctx context.Context, prefix string) (keys []string, err error)
```

## Use Case

We need to load the most recent daily export file from S3. Files follow this naming pattern:

```
export_1_20260227T092613889745Z.json.gz
export_1_20260227T234212579822Z.json.gz
export_1_20260228T234047582234Z.json.gz
export_1_20260301T234233401877Z.json.gz
```

The workflow:
1. List objects with prefix `export_1_`
2. Sort keys to find the most recent (timestamp is embedded in filename)
3. Get that object

## S3 API Reference

The S3 `ListObjectsV2` API:
- Endpoint: `GET /?list-type=2&prefix=<prefix>`
- Returns XML with `Contents` elements containing `Key` for each object
- Supports pagination via `ContinuationToken` if results exceed 1000

Example response:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>bucket-name</Name>
  <Prefix>export_1_</Prefix>
  <Contents>
    <Key>export_1_20260301T234233401877Z.json.gz</Key>
    <Size>31536069</Size>
    <LastModified>2026-03-02T06:00:53.000Z</LastModified>
  </Contents>
  ...
</ListBucketResult>
```

## Notes

- Pagination may not be needed initially if we expect fewer than 1000 objects
- The existing signing logic in `objsto` should work for this request type
