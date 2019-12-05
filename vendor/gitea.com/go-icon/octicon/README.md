# octicon
https://octicons.github.com/  
Credit to [shurcooL](https://github.com/shurcooL) for a lot of the groundwork for this library.  
The original source of this library is [his octicon library](https://github.com/shurcooL/octicon).  

# Installation
`go get -u gitea.com/go-icon/octicon`

# Usage
```go
icon := octicon.Alert()

// Get the raw XML
xml := icon.XML()

// Get something suitable to pass directly to an html/template
html := icon.HTML()
```

# Build
`go generate generate.go`

# New Versions
To update the version of octicons, simply change `octiconVersion` in `octicon_generate.go` and re-build.

# License
[MIT License](LICENSE)