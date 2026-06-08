# Terraform Module Registry

Gitea exposes a private Terraform Module Registry implementing the
[HashiCorp module-registry protocol](https://developer.hashicorp.com/terraform/internals/module-registry-protocol).

## Scope (v1)

- Only the root module is parsed; submodules and examples are ignored.
- Only `.tar.gz` archives are accepted on upload.
- Only HCL (`.tf`) files are parsed. `.tf.json` files cause an upload
  to be rejected.
- Module versions are immutable — re-uploading the same
  `{namespace, name, provider, version}` returns `409 Conflict`.
- Delete is the only mutation other than upload. There is no search and
  no deprecation flow.

## Naming rules

| Segment   | Pattern                            |
| --------- | ---------------------------------- |
| namespace | `[a-z0-9][a-z0-9_-]{0,63}` (== Gitea user/org) |
| name      | `[a-z0-9][a-z0-9_-]{0,63}`         |
| provider  | `[a-z0-9][a-z0-9-]{0,63}` (no `_`) |
| version   | semver 2.0                          |

## Authentication

Generate a personal access token with `read:package` (and `write:package`
for upload/delete) scopes. Add it to `~/.terraformrc`:

```hcl
credentials "gitea.example.com" {
  token = "<token>"
}
```

## Service discovery

Terraform calls `https://gitea.example.com/.well-known/terraform.json`
and receives:

```json
{ "modules.v1": "/api/packages/-/terraform/modules/" }
```

The `-` prefix is reserved Gitea routing space, used so the namespace
path segment maps directly to a Gitea user/org without colliding with
the per-user package routes at `/api/packages/{username}/...`.

## HTTP API

All paths below are rooted at the discovered base
(`/api/packages/-/terraform/modules/`).

### List versions

```
GET :base/:namespace/:name/:provider/versions
```

Response:

```json
{ "modules": [ { "versions": [ { "version": "1.0.0" }, ... ] } ] }
```

### Download

```
GET :base/:namespace/:name/:provider/:version/download
```

Returns `204 No Content` with `X-Terraform-Get` pointing at the archive.

```
GET :base/:namespace/:name/:provider/:version/archive
```

Streams the `.tar.gz` blob.

### Publish

```
PUT :base/:namespace/:name/:provider/:version
```

Body is the raw `.tar.gz` archive. Requires `write:package` scope.

Example:

```sh
curl -u "$USER:$TOKEN" -X PUT \
  --data-binary @vpc-1.0.0.tar.gz \
  https://gitea.example.com/api/packages/-/terraform/modules/acme/vpc/aws/1.0.0
```

### Delete

```
DELETE :base/:namespace/:name/:provider/:version
```

Requires `write:package` scope. Returns `204 No Content`.

## Consuming a module

```hcl
module "vpc" {
  source  = "gitea.example.com/acme/vpc/aws"
  version = "~> 1.0"
}
```

`terraform init` resolves the module via the service-discovery document
and pulls the archive from `:base/.../{version}/archive`.

## Settings

`[packages]` section:

```ini
LIMIT_SIZE_TERRAFORM_MODULE = 100MB
```

`-1` disables the per-module *storage* limit; the global
`LIMIT_TOTAL_OWNER_SIZE` still applies. Independently of this setting,
the registry always enforces a 32 MiB ceiling on the decompressed bytes
read while parsing an archive, so a malformed or malicious gzip stream
cannot exhaust memory.
