Cutter
======

A Go library to crop images.

[![Build Status](https://travis-ci.org/oliamb/cutter.png?branch=master)](https://travis-ci.org/oliamb/cutter)
[![GoDoc](https://godoc.org/github.com/oliamb/cutter?status.png)](https://godoc.org/github.com/oliamb/cutter)

Cutter was initially developped to be able
to crop image resized using github.com/nfnt/resize.

Usage
-----

Read the doc on https://godoc.org/github.com/oliamb/cutter

Import package with

```go
import "github.com/oliamb/cutter"
```

Package cutter provides a function to crop image.

By default, the original image will be cropped at the
given size from the top left corner.

```go
croppedImg, err := cutter.Crop(img, cutter.Config{
  Width:  250,
  Height: 500,
})
```

Most of the time, the cropped image will share some memory
with the original, so it should be used read only. You must
ask explicitely for a copy if nedded.

```go
croppedImg, err := cutter.Crop(img, cutter.Config{
  Width:  250,
  Height: 500,
  Options: cutter.Copy,
})
```

It is possible to specify the top left position:

```go
croppedImg, err := cutter.Crop(img, cutter.Config{
  Width:  250,
  Height: 500,
  Anchor: image.Point{100, 100},
  Mode:   cutter.TopLeft, // optional, default value
})
```

The Anchor property can represents the center of the cropped image
instead of the top left corner:

```go
croppedImg, err := cutter.Crop(img, cutter.Config{
  Width: 250,
  Height: 500,
  Mode: cutter.Centered,
})
```

The default crop use the specified dimension, but it is possible
to use Width and Heigth as a ratio instead. In this case,
the resulting image will be as big as possible to fit the asked ratio
from the anchor position.

```go
croppedImg, err := cutter.Crop(baseImage, cutter.Config{
  Width: 4,
  Height: 3,
  Mode: cutter.Centered,
  Options: cutter.Ratio&cutter.Copy, // Copy is useless here
})
```

About resize
------------
This lib only manage crop and won't resize image, but it works great in combination with [github.com/nfnt/resize](https://github.com/nfnt/resize)

Contributing
------------
I'd love to see your contributions to Cutter. If you'd like to hack on it: 

- fork the project,
- hack on it,
- ensure tests pass,
- make a pull request

If you plan to modify the API, let's disscuss it first.

Licensing
---------
MIT License, Please see the file called LICENSE.

Credits
-------
Test Picture: Gopher picture from Heidi Schuyt, http://www.flickr.com/photos/hschuyt/7674222278/,
© copyright Creative Commons(http://creativecommons.org/licenses/by-nc-sa/2.0/)

Thanks to Urturn(http://www.urturn.com) for the time allocated to develop the library.
