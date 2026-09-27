# Federation Groundwork (ActivityPub / ForgeFed)

Teabag preserves a minimal modular federation layer rather than a monolithic federation subsystem.

## Existing Architecture

- `models/auth/access_token_scope.go`: Defines `AccessTokenScopeReadActivityPub` and `AccessTokenScopeWriteActivityPub` for federated token scopes.
- `routers/api/v1/activitypub/person.go`: Provides a stub ActivityPub `person` endpoint (`NotImplemented`).
- HTTP signatures (`modules/auth/httpsign`) and keypair infrastructure remain intact.

## Security Considerations

Federation introduces remote content risks. Configuration controls (future work) should include:

- Federation enabled/disabled toggle
- Inbound federation allow/block lists
- Request timeout limits
- Maximum payload size limits
- Rate limits on inbox delivery
- Signature validation for all inbound activities

Remote actor validation and object validation must remain strict; no unsafe "fetch any URL" behavior should be added.

## Preferred Protocol

Use standard ActivityPub concepts (`actor`, `object`, `activity`) with ForgeFed vocabulary where applicable. Keep federation modular: the federation layer should depend on core repository/issue/PR abstractions rather than duplicating them.
