# bufpipe: Buffered Pipe

[![CircleCI](https://img.shields.io/circleci/build/github/acomagu/bufpipe.svg?style=flat-square)](https://circleci.com/gh/acomagu/bufpipe) [![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/acomagu/bufpipe)

The buffered version of io.Pipe. It's safe for concurrent use.

## How does it differ from io.Pipe?

Writes never block because the pipe has variable-sized buffer.

```Go
r, w := bufpipe.New(nil)
io.WriteString(w, "abc") // No blocking.
io.WriteString(w, "def") // No blocking, too.
w.Close()
io.Copy(os.Stdout, r)
// Output: abcdef
```

[Playground](https://play.golang.org/p/PdyBAS3pVob)

## How does it differ from bytes.Buffer?

Reads block if the internal buffer is empty until the writer is closed.

```Go
r, w := bufpipe.New(nil)

done := make(chan struct{})
go func() {
	io.Copy(os.Stdout, r) // The reads block until the writer is closed.
	done <- struct{}{}
}()

io.WriteString(w, "abc")
io.WriteString(w, "def")
w.Close()
<-done
// Output: abcdef
```

[Playground](https://play.golang.org/p/UppmyLeRgX6)
