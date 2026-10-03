# Custom webhook types

Gitea supports admin-defined webhook types in addition to the built-in integrations
(Slack, Discord, Matrix, …). Types are stored in the `hook_type` table and listed
under **Site Administration → Integrations → Webhook Types**.

## Adding a type (admin UI)

1. Open **Webhook Types** → **Add Webhook Type**.
2. Choose a stable lowercase `Type ID` (e.g. `my-tool`).
3. Optionally define a **Form Schema** (JSON array) for configuration fields shown
   when users create a webhook of this type.
4. Provide **Payload Jsonnet** and/or **Request Jsonnet** (see below).
5. Save. The type appears in the webhook type dropdown for repos/orgs/users.

Built-in types are seeded from [`options/webhooks/<name>/`](../options/webhooks/)
(`metadata.yml` + `request.jsonnet` / `payload.jsonnet`). Admins can edit them and
**Reset** to the shipped definition.

## Jsonnet context

Programs receive these external variables (`std.extVar`):

| Name | Description |
|------|-------------|
| `event` | Parsed Gitea event payload (object) |
| `event_type` | Hook event type string (e.g. `push`) |
| `meta` | User configuration from the webhook form (object) |
| `hook_type` | Type id string |
| `webhook` | `{url, secret, http_method, content_type}` |

### Payload Jsonnet

Must evaluate to the HTTP body (usually a JSON object). Method/URL come from the
webhook row; default Gitea signature headers are added.

### Request Jsonnet

Must evaluate to:

```json
{
  "method": "POST",
  "url": "https://example.invalid/hook",
  "headers": {"Content-Type": "application/json"},
  "body": {},
  "with_default_headers": true
}
```

When `with_default_headers` is `false`, Gitea will not add `X-Gitea-*` / HMAC headers
(the program or a legacy convertor already did).

### Natives

| Native | Purpose |
|--------|---------|
| `std.native("gitea_hmac_sha256")(secret, message)` | Hex HMAC-SHA256 |
| `std.native("gitea_sha256")(message)` | Hex SHA256 |
| `std.native("gitea_uuid")()` | Random id |
| `std.native("gitea_url_query_escape")(s)` | `url.QueryEscape` |
| `std.native("gitea_legacy_request")(hook_type, event_type, event, meta, url, secret, method, content_type)` | Full request via built-in Go convertor |
| `std.native("gitea_legacy_payload")(...)` | Body only via built-in Go convertor |

## Developer: Go Handler

Native types (gitea/gogs, or complex convertors) implement `services/webhook.Handler`
and register with `RegisterHandler` / `RegisterWebhookRequester`. Delivery always goes
through `Handler.NewRequest`. Custom types are registered as `JsonnetHandler` from the
database at startup (`LoadHandlersFromDB`).

## Form schema example

```json
[
  {"id": "channel", "label": "Channel", "type": "text", "required": true},
  {"id": "token", "label": "Token", "type": "secret", "required": true}
]
```

Field `type` values: `text`, `number`, `bool`, `secret`, `url`.
