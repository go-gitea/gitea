# identicon

[![Go](https://github.com/issue9/identicon/actions/workflows/go.yml/badge.svg)](https://github.com/issue9/identicon/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/issue9/identicon/branch/master/graph/badge.svg)](https://codecov.io/gh/issue9/identicon)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/issue9/identicon)](https://pkg.go.dev/github.com/issue9/identicon)
![Go version](https://img.shields.io/github/go-mod/go-version/issue9/identicon)
![License](https://img.shields.io/github/license/issue9/identicon)

根据用户的 IP 、邮箱名等任意数据为用户产生漂亮的随机头像。

![screenshot.1](https://raw.github.com/issue9/identicon/master/screenshot/1.png)
![screenshot.4](https://raw.github.com/issue9/identicon/master/screenshot/4.png)
![screenshot.5](https://raw.github.com/issue9/identicon/master/screenshot/5.png)
![screenshot.6](https://raw.github.com/issue9/identicon/master/screenshot/6.png)
![screenshot.7](https://raw.github.com/issue9/identicon/master/screenshot/7.png)

```go
// 根据用户访问的IP，为其生成一张头像
img, _ := identicon.Make(128, color.NRGBA{},color.NRGBA{}, []byte("192.168.1.1"))
fi, _ := os.Create("/tmp/u1.png")
png.Encode(fi, img)
fi.Close()

// 或者
ii, _ := identicon.New(128, color.NRGBA{}, color.NRGBA{}, color.NRGBA{}, color.NRGBA{})
img := ii.Make([]byte("192.168.1.1"))
img = ii.Make([]byte("192.168.1.2"))
```

## 安装

```shell
go get github.com/issue9/identicon
```

## 版权

本项目采用 [MIT](https://opensource.org/licenses/MIT) 开源授权许可证，完整的授权说明可在 [LICENSE](LICENSE) 文件中找到。
