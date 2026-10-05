local event = std.extVar("event");
local event_type = std.extVar("event_type");
local meta = std.extVar("meta");
local webhook = std.extVar("webhook");
local hook_type = std.extVar("hook_type");
std.native("gitea_legacy_request")(
  hook_type,
  event_type,
  event,
  meta,
  webhook.url,
  webhook.secret,
  webhook.http_method,
  webhook.content_type
)
