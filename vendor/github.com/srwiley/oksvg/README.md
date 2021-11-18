# oksvg
oksvg is a rasterizer for a partial implementation of the SVG2.0 specification in golang.

Although many SVG elements will not be read by oksvg, it is good enough to faithfully produce thousands, but certainly not all, SVG icons available both for free and commercially. A list of valid and invalid elements is in the doc folder.

oksvg uses the [rasterx](https://github.com/srwiley/rasterx) rasterizer package which implements full SVG2.0 path functions, including the newer 'arc' join-mode.

![arcs and caps](doc/TestShapes.png)

### Extra non-standard features.

In addition to 'arc' as a valid join mode value, oksvg also allows 'arc-clip' which is the arc analog of miter-clip and some extra capping and gap values. It can also specify different capping functions for line starts and ends.

#### Rasterizations of SVG to PNG from creative commons 3.0 sources.

Example renderings of unedited open source SVG files by oksvg and rasterx are shown below.

Thanks to [Freepik](http://www.freepik.com) from [Flaticon](https://www.flaticon.com/)
Licensed by [Creative Commons 3.0](http://creativecommons.org/licenses/by/3.0/) for the example icons shown below, and also used as test icons in the testdata folder.

![Jupiter](doc/jupiter.png)

![lander](doc/lander.png)

![mountains](doc/mountains.png)

![bus](doc/school-bus.png)

### Non-standard library dependencies
oksvg requires the following imports which are not included in the go standard library:

* golang.org/x/net/html/charset
* golang.org/x/image/colornames
* golang.org/x/image/math/fixed

These can be included in your gopath by the following 'get' commands:

* "go get golang.org/x/image/math/fixed"
* "go get golang.org/x/image/colornames"
* "go get golang.org/x/net/html/charset"

oksvg also requires the user to get or clone into the workspace the rasterx package located here:

* github.com/srwiley/rasterx





