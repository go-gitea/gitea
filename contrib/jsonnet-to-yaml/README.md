# jsonnet-to-yaml

This contrib is loosely based on the [jsonnet](https://github.com/google/go-jsonnet/blob/123396675b13d4fd40e9a93a3ae053ee7811efb7/cmd/jsonnet/cmd.go) command.

To compare the current jsonnet file with the output (to check if changes produce a diff, for example), use the `--compare` flag.
This can be important because, due to how the Go YAML library works, the output can _look_ different even if the overall output is the same.

## Generate `.drone.yml`
```
go run ./contrib/jsonnet-to-yaml
```

## Compare changes
```
go run ./contrib/jsonnet-to-yaml --compare
```
With HTML diff:
```
go run ./contrib/jsonnet-to-yaml --compare --html
```

## License

[Apache-2.0](https://github.com/google/go-jsonnet/blob/123396675b13d4fd40e9a93a3ae053ee7811efb7/LICENSE)
