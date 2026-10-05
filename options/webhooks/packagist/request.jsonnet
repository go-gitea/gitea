local meta = std.extVar("meta");
local event_type = std.extVar("event_type");
local webhook = std.extVar("webhook");
local package_url = if std.objectHas(meta, "package_url") then meta.package_url else "";
// Match Go PackagistPayload: always include repository.url (empty for non-push).
{
  method: "POST",
  url: webhook.url,
  headers: { "Content-Type": "application/json" },
  body: {
    repository: {
      url: if event_type == "push" then package_url else "",
    },
  },
  with_default_headers: true,
}
