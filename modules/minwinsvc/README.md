### Minimal windows service stub

Programs designed to run from most *nix style operating systems
can import this package to enable running programs as services without modifying
them.

```
import _ "github.com/kardianos/minwinsvc"
```

If you need more control over the exit behavior, set
```
minwinsvc.SetOnExit(func() {
	// Do something.
	// Within 10 seconds call:
	os.Exit(0)
})
```
