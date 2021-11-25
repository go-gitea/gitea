# CORS net/http middleware

[go-chi/cors](https://github.com/go-chi/cors) is a fork of [github.com/rs/cors](https://github.com/rs/cors) that
provides a `net/http` compatible middleware for performing preflight CORS checks on the server side. These headers
are required for using the browser native [Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API).

This middleware is designed to be used as a top-level middleware on the [chi](https://github.com/go-chi/chi) router.
Applying with within a `r.Group()` or using `With()` will not work without routes matching `OPTIONS` added.

## Usage

```go
func main() {
  r := chi.NewRouter()

  // Basic CORS
  // for more ideas, see: https://developer.github.com/v3/#cross-origin-resource-sharing
  r.Use(cors.Handler(cors.Options{
    // AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
    AllowedOrigins:   []string{"https://*", "http://*"},
    // AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
    ExposedHeaders:   []string{"Link"},
    AllowCredentials: false,
    MaxAge:           300, // Maximum value not ignored by any of major browsers
  }))

  r.Get("/", func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("welcome"))
  })

  http.ListenAndServe(":3000", r)
}
```

## Credits

All credit for the original work of this middleware goes out to [github.com/rs](github.com/rs).
