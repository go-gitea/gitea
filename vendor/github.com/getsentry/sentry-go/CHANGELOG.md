# Changelog

## v0.2.1

- fix: Run `Contextify` integration on `Threads` as well

## v0.2.0

- feat: Add `SetTransaction()` method on the `Scope`
- feat: `fasthttp` framework support with `sentryfasthttp` package
- fix: Add `RWMutex` locks to internal `Hub` and `Scope` changes

## v0.1.3

- feat: Move frames context reading into `contextifyFramesIntegration` (#28)

_NOTE:_
In case of any performance isues due to source contexts IO, you can let us know and turn off the integration in the meantime with:

```go
sentry.Init(sentry.ClientOptions{
	Integrations: func(integrations []sentry.Integration) []sentry.Integration {
		var filteredIntegrations []sentry.Integration
		for _, integration := range integrations {
			if integration.Name() == "ContextifyFrames" {
				continue
			}
			filteredIntegrations = append(filteredIntegrations, integration)
		}
		return filteredIntegrations
	},
})
```

## v0.1.2

- feat: Better source code location resolution and more useful inapp frames (#26)
- feat: Use `noopTransport` when no `Dsn` provided (#27)
- ref: Allow empty `Dsn` instead of returning an error (#22)
- fix: Use `NewScope` instead of literal struct inside a `scope.Clear` call (#24)
- fix: Add to `WaitGroup` before the request is put inside a buffer (#25)

## v0.1.1

- fix: Check for initialized `Client` in `AddBreadcrumbs` (#20)
- build: Bump version when releasing with Craft (#19)

## v0.1.0

- First stable release! \o/

## v0.0.1-beta.5

- feat: **[breaking]** Add `NewHTTPTransport` and `NewHTTPSyncTransport` which accepts all transport options
- feat: New `HTTPSyncTransport` that blocks after each call
- feat: New `Echo` integration
- ref: **[breaking]** Remove `BufferSize` option from `ClientOptions` and move it to `HTTPTransport` instead
- ref: Export default `HTTPTransport`
- ref: Export `net/http` integration handler
- ref: Set `Request` instantly in the package handlers, not in `recoverWithSentry` so it can be accessed later on
- ci: Add craft config

## v0.0.1-beta.4

- feat: `IgnoreErrors` client option and corresponding integration
- ref: Reworked `net/http` integration, wrote better example and complete readme
- ref: Reworked `Gin` integration, wrote better example and complete readme
- ref: Reworked `Iris` integration, wrote better example and complete readme
- ref: Reworked `Negroni` integration, wrote better example and complete readme
- ref: Reworked `Martini` integration, wrote better example and complete readme
- ref: Remove `Handle()` from frameworks handlers and return it directly from New

## v0.0.1-beta.3

- feat: `Iris` framework support with `sentryiris` package
- feat: `Gin` framework support with `sentrygin` package
- feat: `Martini` framework support with `sentrymartini` package
- feat: `Negroni` framework support with `sentrynegroni` package
- feat: Add `Hub.Clone()` for easier frameworks integration
- feat: Return `EventID` from `Recovery` methods
- feat: Add `NewScope` and `NewEvent` functions and use them in the whole codebase
- feat: Add `AddEventProcessor` to the `Client`
- fix: Operate on requests body copy instead of the original
- ref: Try to read source files from the root directory, based on the filename as well, to make it work on AWS Lambda
- ref: Remove `gocertifi` dependence and document how to provide your own certificates
- ref: **[breaking]** Remove `Decorate` and `DecorateFunc` methods in favor of `sentryhttp` package
- ref: **[breaking]** Allow for integrations to live on the client, by passing client instance in `SetupOnce` method
- ref: **[breaking]** Remove `GetIntegration` from the `Hub`
- ref: **[breaking]** Remove `GlobalEventProcessors` getter from the public API

## v0.0.1-beta.2

- feat: Add `AttachStacktrace` client option to include stacktrace for messages
- feat: Add `BufferSize` client option to configure transport buffer size
- feat: Add `SetRequest` method on a `Scope` to control `Request` context data
- feat: Add `FromHTTPRequest` for `Request` type for easier extraction
- ref: Extract `Request` information more accurately
- fix: Attach `ServerName`, `Release`, `Dist`, `Environment` options to the event
- fix: Don't log events dropped due to full transport buffer as sent
- fix: Don't panic and create an appropriate event when called `CaptureException` or `Recover` with `nil` value

## v0.0.1-beta

- Initial release
