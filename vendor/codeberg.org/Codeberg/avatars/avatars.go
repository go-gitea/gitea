package avatars

import (
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
)

// MakeAvatar create svg from seed
func MakeAvatar(seedString string) string {
	var seed uint64
	for _, c := range []byte(seedString) {
		seed = bits.RotateLeft64(seed, 8)
		seed ^= uint64(c)
	}
	if seed&1 == 0 {
		return femaleAvatar(seed, "")
	}
	return maleAvatar(seed, "")
}

/**
 *  Based on TypeScript DiceBear Avatars, which in turn were inspired by 8biticon avatars:
 *  (MIT License, Copyright (c) 2012 Plastic Jam, Copyright (c) 2019 DiceBear)
 *    cf. https://github.com/DiceBear/avatars/blob/master/packages/avatars-male-sprites/src/index.ts
 */

type lcg struct {
	seed uint64
}

func (g *lcg) random() uint32 {
	/* Linear Congruent Generator, POSIX/glibc [de]rand48 setting, bits [47..0] are output bits */
	g.seed = (25214903917*g.seed + 11) % 281474976710656
	return uint32(g.seed)
}

func (g *lcg) binomial(p float64) bool {
	/* Sample from Binomial distribution with probability p */
	var sample = float64(g.random()) * float64(1.0/4294967295.0)
	return sample > p
}

func (g *lcg) pickOne(s []string) string {
	/* Pick one element from list */
	var N = uint32(len(s))
	return s[g.random()%N]
}

func (g *lcg) pickOneFloat(s []float64) float64 {
	/* Pick one element from list - float version*/
	var N = uint32(len(s))
	return s[g.random()%N]
}

func (g *lcg) pickAorB(p float64, a string, b string) string {
	/* Pick a or b with probability p of picking a */
	if g.binomial(p) {
		return a
	}
	return b
}

func linearCongruentialGenerator(seed uint64) *lcg {
	g := new(lcg)
	g.seed = seed
	return g
}

type rgb struct {
	r uint8
	g uint8
	b uint8
	a uint8
}

type hsv struct {
	h float64
	s float64
	v float64
}

func f2rgb(r, g, b float64) *rgb {
	// make rgb from 3 floats
	c := new(rgb)
	c.r = uint8(r * 1.0 / 255)
	c.g = uint8(g * 1.0 / 255)
	c.b = uint8(b * 1.0 / 255)
	c.a = 255
	return c
}

func f2hsv(h, s, v float64) *hsv {
	// make hsv from 3 floats
	c := new(hsv)
	c.h = h
	c.s = s
	c.v = v
	return c
}

func toRgb(s string) *rgb {
	c := new(rgb)
	c.a = 255
	fmt.Sscanf(s, "#%02x%02x%02x", &c.r, &c.g, &c.b)
	return c
}

func (c *rgb) toHsv() *hsv {
	var h float64
	var s float64
	var v float64
	r := 255.0 * float64(c.r)
	g := 255.0 * float64(c.g)
	b := 255.0 * float64(c.b)
	min := math.Min(math.Min(r, g), b)
	v = math.Max(math.Max(r, g), b)
	C := v - min
	s = 0.0
	if v != 0.0 {
		s = C / v
	}
	h = 0.0
	if min != v {
		if v == r {
			h = math.Mod((g-b)/C, 6.0)
		}
		if v == g {
			h = (b-r)/C + 2.0
		}
		if v == b {
			h = (r-g)/C + 4.0
		}
		h *= 60.0
		if h < 0.0 {
			h += 360.0
		}
	}
	return f2hsv(h, s, v)
}

func (c *hsv) toRgb() *rgb {
	h := int((c.h / 60))
	f := c.h/60 - float64(h)
	p := c.v * (1 - c.s)
	q := c.v * (1 - c.s*f)
	t := c.v * (1 - c.s*(1-f))
	switch h {
	case 6:
	case 0:
		return f2rgb(c.v, t, p)
	case 1:
		return f2rgb(q, c.v, p)
	case 2:
		return f2rgb(p, c.v, t)
	case 3:
		return f2rgb(p, q, c.v)
	case 4:
		return f2rgb(t, p, c.v)
	case 5:
		return f2rgb(c.v, p, q)
	}
	return f2rgb(0, 0, 0)
}

func (c *rgb) brighterThan(ref *rgb, delta float64) *rgb {
	primary := c.toHsv()
	secondary := ref.toHsv()
	if primary.v >= secondary.v+delta {
		return c
	}
	primary.v = secondary.v + delta
	if primary.v > 360 {
		primary.v = 360
	}
	return primary.toRgb()
}

func (c *rgb) darkerThan(ref *rgb, delta float64) *rgb {
	primary := c.toHsv()
	secondary := ref.toHsv()
	if primary.v <= secondary.v-delta {
		return c
	}
	primary.v = secondary.v - delta
	if primary.v < 0 {
		primary.v = 0
	}
	return primary.toRgb()
}

func (c *rgb) brighterOrDarkerThan(ref *rgb, delta float64) *rgb {
	primary := c.toHsv()
	secondary := ref.toHsv()
	if primary.v <= secondary.v {
		return c.darkerThan(ref, delta)
	}
	return c.brighterThan(ref, delta)
}

func (c *rgb) withAlpha(alpha float64) *rgb {
	c.a = uint8(alpha * 255)
	return c
}

func (c *rgb) html() string {
	if c.a == 255 {
		return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.r, c.g, c.b, c.a)
}

func maleAvatar(seed uint64, mood string) string {
	var g = linearCongruentialGenerator(seed)
	var skinColor = toRgb(g.pickOne([]string{"#FFDBAC", "#F5CFA0", "#EAC393", "#E0B687", "#CB9E6E", "#B68655", "#A26D3D", "#8D5524"}))
	var hairColor = toRgb(g.pickOne([]string{"#090806", "#2c222b", "#71635a", "#b7a69e", "#b89778", "#a56b46", "#b55239", "#8d4a43",
		"#91553d", "#533d32", "#3b3024", "#554838", "#4e433f", "#504444", "#6a4e42", "#a7856a", "#977961"})).brighterOrDarkerThan(skinColor, 17)
	var eyesColor = toRgb(g.pickOne([]string{"#76778b", "#697b94", "#647b90", "#5b7c8b", "#588387"}))
	var eyebrowsColor = hairColor.darkerThan(skinColor, 7).darkerThan(hairColor, 10)
	var mustacheColor = hairColor.darkerThan(skinColor, 7).withAlpha(g.pickOneFloat([]float64{1, 0.75, 0.5}))
	var mouthColor = toRgb(g.pickOne([]string{"#eec1ad", "#dbac98", "#d29985"})).brighterOrDarkerThan(skinColor, 10)
	var glassesColor = toRgb(g.pickOne([]string{"#5f705c", "#43677d", "#5e172d", "#ffb67a", "#a04b5d", "#191919", "#323232", "#4b4b4b"}))
	var clothesColor = toRgb(g.pickOne([]string{"#5bc0de", "#5cb85c", "#428bca", "#03396c", "#005b96", "#6497b1", "#1b85b8", "#5a5255", "#559e83", "#ae5a41", "#c3cb71", "#666547", "#ffe28a"}))
	var hatColor = toRgb(g.pickOne([]string{"#18293b", "#2e1e05", "#989789", "#3d6ba7", "#517459", "#a62116"}))

	if mood == "" {
		mood = g.pickOne([]string{"sad", "happy", "surprised"})
	}
	var mouth string
	if mood == "sad" {
		mouth = "<path d='M8 13h3v1H8v-1z' fill='${mouthColor}'/>" +
			"<path d='M8 13h4v1H8v-1z' fill='${mouthColor}'/>" +
			"<path d='M9 13h2v1H9v-1z' fill='${mouthColor}'/>" +
			"<path d='M8 12v1h3v1h1v-1h-1v-1H8z' fill='${mouthColor}'/>" +
			"<path d='M8 13v1h1v-1h3v-1H9v1H8z' fill='${mouthColor}'/>"
	} else if mood == "happy" {
		mouth = "<path d='M7 12v1h1v1h4v-1H8v-1H7z' fill='${mouthColor}'/>" +
			"<path d='M10 12v1H9v1h2v-2h-1z' fill='${mouthColor}'/>" +
			"<path d='M8 13v1h4v-1h1v-1h-1v1H8z' fill='${mouthColor}'/>" +
			"<path d='M8 12v2h4v-2H8z' fill='#FFF'/>"
	} else if mood == "surprised" {
		mouth = "<path d='M9 12v2h2v-2H9z' fill='${mouthColor}'/>" +
			"<path d='M9 13v1h1v-1H9z' fill='${mouthColor}'/>"
	}

	var s = strings.Join([]string{
		"<svg xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' style='isolation:isolate' viewBox='0 0 20 20' version='1.1' shape-rendering='crispEdges'>",
		// Head
		"<path d='M8 15v1H4v1H3v3h14v-3h-1v-1h-4v-1h3v-1h1v-1h1v-3h1V7h-1V4h-1V3h-1V2H5v1H4v1H3v3H2v3h1v3h1v1h1v1h3z' fill='${skinColor}'/><path d='M5 15v-1H4v-1H3v-3H2V7h1V4h1V3h1V2h10v1h1v1h1v3h1v3h-1v3h-1v1h-1v1H5z' fill='#FFF' fill-opacity='.1'/>",
		// Eyes
		g.pickOne([]string{
			"<path d='M5 9V7h3v2H5zm7-2h3v2h-3V7z' fill='#FFF'/><path d='M7 8v1h1V8H7zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 1V8h1v1h-1z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M7 8h1v1H7V8zm5 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M6 7h1v1h1v1H6V7zm6 0h1v1h1v1h-2V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm6 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='#FFF'/><path d='M7 8h1v1H7V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='#FFF'/><path d='M7 8h1v1H7V8zm5 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7v1h3V7H5zm7 0h3v1h-3V7z' fill='#FFF'/><path d='M5 9V8h1V7h1v1h1v1H5zm7 0V8h1V7h1v1h1v1h-3z' fill='${eyesColor}'/><path d='M5 9V8h1V7h1v1h1v1H7V8H6v1H5zm7 0V8h1V7h1v1h1v1h-1V8h-1v1h-1z' fill='#FFF' fill-opacity='.5'/>",
			"<path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M6 7h2v2H6V7zm7 0h2v2h-2V7z' fill='${eyesColor}'/><path d='M6 7v1h1v1h1V8H7V7H6zm7 0v1h1v1h1V8h-1V7h-1z' fill='#FFF' fill-opacity='.4'/><path d='M7 7v1h1V7H7zm7 0h1v1h-1V7z' fill='#FFF' fill-opacity='.7'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='${eyesColor}'/><path d='M5 8h1v1H5V8zm7 0h1v1h-1V8z' fill='#FFF' fill-opacity='.7'/>",
			"<path d='M6 7h1v2H5V8h1V7zm7 0h1v2h-2V8h1V7z' fill='#FFF'/><path d='M7 7v1H6v1h2V7H7zm7 0v1h-1v1h2V7h-1z' fill='${eyesColor}'/><path d='M7 7v1h1V7H7zM6 8v1h1V8H6zm8-1v1h1V7h-1zm-1 1v1h1V8h-1z' fill='#FFF' fill-opacity='.5'/>",
		}),
		// Eyebrows
		g.pickOne([]string{
			"<path d='M7 5v1H5v1H4V6h1V5h2zm7 0v1h-2v1h-1V6h1V5h2z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M8 4v1H7v1H5V5h2V4h1zm4 0h1v1h2v1h-2V5h-1V4z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h3v2H8V6H6V5zm5 0h3v1h-2v1h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M5 5h3v1h1v1H8V6H5V5zm10 0h-3v1h-1v1h1V6h3V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5H4v2h1V6h1V5zm8 0h2v2h-1V6h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M5 6h2v1H5V6zm8 0h2v1h-2V6z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h1v1h1v1H5V6h1V5zm7 0h1v1h1v1h-3V6h1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h2v1h1v1H8V6H6V5zm8 0h-2v1h-1v1h1V6h2V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M12 7V6h1V5h1v1h1v1h-1V6h-1v1h-1zM5 7V6h1V5h1v1h1v1H7V6H6v1H5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M7 5v1H5v1H4V6h1V5h2zm6 0h2v1h1v1h-1V6h-2V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M4 7V5h3v1H5v1H4zm12-2v2h-1V6h-2V5h3z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M7 5h1v1h1v1H8V6H7V5zm6 0v1h-1v1h-1V6h1V5h1z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M4 7V6h1V5h1v1H5v1H4zm10-2h1v1h1v1h-1V6h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
		}),
		// Mustache (50% chance)
		g.pickAorB(0.5,
			g.pickOne([]string{
				"<path d='M3 10v3h1v1h1v1h10v-1h1v-1h1v-3h-3v1H6v-1H3z' id='Path' fill='${mustacheColor}' fill-opacity='${mustacheColorAlpha}'/>",
				"<path d='M3 13h1v1h1v1h10v-1h1v-1h1v-3h-1v1h-1v1H5v-1H4v-1H3v3z' id='Path' fill='${mustacheColor}' fill-opacity='${mustacheColorAlpha}'/>",
				"<path d='M3 11v2h1v1h1v1h10v-1h1v-1h1v-2H3z' id='Path' fill='${mustacheColor}' fill-opacity='${mustacheColorAlpha}'/>",
				"<path d='M3 7v6h1v1h1v1h10v-1h1v-1h1V7h-1v2h-1v1h-1v1H6v-1H5V9H4V7H3z' id='Path' fill='${mustacheColor}' fill-opacity='${mustacheColorAlpha}'/>",
			}),
			""),
		// Mouth
		mouth,
		// Glasses (25% chance)
		g.pickAorB(0.25,
			g.pickOne([]string{
				"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M12 10V7h3v3h-3zm-1-4v1H9V6H4v1H3v1h1v3h5V8h2v3h5V8h1V7h-1V6h-5zm-6 4V7h3v3H5z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M5 7v2h3V7H5zM4 6v1H3v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7h-1V6h-5v1H9V6H4zm8 1v2h3V7h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 8h1v1H7V8zm7 0h1v1h-1V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M5 8v1h3V8H5zM3 7v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7H3zm9 1v1h3V8h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7v1h1V7H3zm6 0v1h2V7H9zm7 0v1h1V7h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M12 7v2h3V7h-3zM8 6H5v1H3v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7h-2V6h-3v1H8V6zM5 7v2h3V7H5z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 8H3V7h1V6h5v1h2V6h5v1h1v1h-1v2h-5V8H9v2H4V8zm1 0V7h3v2H5V8zm7-1v2h3V7h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M14 7h1v1h-1V7zM7 7h1v1H7V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M3 8V7h1v1H3zm6-1v1h2V7H9zm7 0v1h1V7h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 8H3V7h14v1h-1v2h-5V8H9v2H4V8zm1 0h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 8v1h1V8H7zm7 0v1h1V8h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M3 7v1h1V7H3zm13 0v1h1V7h-1zM9 7v1h2V7H9z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			}),
			""),
		// Clothes
		g.pickOne([]string{
			"<path d='M3 20v-3h1v-1h4v-1h4v1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='#FFF' fill-opacity='.2'/><path d='M12 19v-1h3v1h-3z' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='${clothesColor}'/><path d='M5 20v-2h1v-1h8v1h1v2h-2v-1h-2v1H9v-1H7v1H5z' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='${clothesColor}'/><path d='M8 16H4v1H3v3h14v-3h-1v-1h-4v1h1v1h-1v1h-1v-1H9v1H8v-1H7v-1h1v-1z' fill='#FFF' fill-opacity='.2'/><path d='M9 16v1h2v-1H9z' fill='#FFF'/>",
			"<path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='${clothesColor}'/><path d='M9 16H4v1H3v3h6v-2H8v-1h1v-1zm2 0h5v1h1v3h-6v-2h1v-1h-1v-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.3'/>",
			"<path d='M3 20v-3h1v-1h3v2h6v-2h3v1h1v3H3z' fill='${clothesColor}'/><path d='M5 16H4v1H3v3h2v-4zm1 0h1v2h6v-2h1v4H6v-4zm9 0h1v1h1v3h-2v-4z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h4v1h1v1h2v-1h1v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M4 17v-1h3v1H4zm9 0v-1h3v1h-3z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h3v-1h1v1h1v1h2v-1h1v-1h1v1h3v1h1v3H3z' fill='${clothesColor}'/><path d='M6 16H4v1H3v3h6v-2H8v-1H6v-1zm2 0h1-1zm3 0h1-1zm2 0h1v1h-2v1h-1v2h6v-3h-1v-1h-3z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M5 16H4v1H3v3h14v-3h-1v-1h-3v1H7v-1H5z' fill='${clothesColor}'/><path d='M10 20v-1h3v1h-3z' fill='#FFF' fill-opacity='.5'/><path d='M5 16H4v1H3v3h1v-1h1v-3zm1 0h1v1h6v-1h1v2H6v-2zm9 0h1v1h1v3h-1v-1h-1v-3z' fill-rule='evenodd' fill='#FFF' fill-opacity='.8'/>",
			"<path d='M3 20v-3h1v-1h4v1h4v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M3 20v-1h1v1H3zm2 0v-1h1v1H5zm2 0v-1h1v1H7zm2 0v-1h1v1H9zm2 0v-1h1v1h-1zm2 0v-1h1v1h-1zm2 0v-1h1v1h-1zm1-2h1v1h-1v-1zm-2 0h1v1h-1v-1zm-2 0h1v1h-1v-1zm-2 0h1v1h-1v-1zm-2 0h1v1H8v-1zm-2 0h1v1H6v-1zm-2 0h1v1H4v-1zm-1-1h1v1H3v-1zm2 0h1v1H5v-1zm2 0h1v1H7v-1zm2 0h1v1H9v-1zm2 0h1v1h-1v-1zm2 0h1v1h-1v-1zm2 0h1v1h-1v-1zM4 16h1v1H4v-1zm2 0h1v1H6v-1zm6 0h1v1h-1v-1zm2 0h1v1h-1v-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h4v1h4v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M3 20v-2h1v2H3zm3 0v-2h2v2H6zm4 0v-2h2v2h-2zm4 0v-2h2v2h-2zm2-3v1h1v-1h-1zm-2 1v-2h-2v2h2zm-6-1v1h2v-1H8zm-4-1v2h2v-2H4z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h4v1h4v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M3 19h14v1H3v-1zm0-2h14v1H3v-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h4v1h4v-1h4v1h1v3H3z' fill='${clothesColor}'/>",
			"<path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='${clothesColor}'/>",
		}),
		// Hair (95% chance)
		g.pickAorB(0.95,
			g.pickOne([]string{
				"<path d='M3 3v2h1V4h1V3h10v1h1v1h1V3h-1V2H4v1H3z' fill='${hairColor}'/>",
				"<path d='M5 2h10v1h1v1h1v3h-1V6h-1V5h-1V4h-4v1H8v1H7v1H4V6H3V4h1V3h1V2z' fill='${hairColor}'/>",
				"<path d='M3 6h1V4h1V3h2v1h1v1h4V4h1V3h2v1h1v2h1V4h-1V3h-1V2H5v1H4v1H3v2z' fill='${hairColor}'/>",
				"<path d='M3 8h1V5h12v3h1V4h-1V3h-1V2H5v1H4v1H3v4z' fill='${hairColor}'/>",
				"<path d='M2 4v1h1v1h2V4h1V2H4v1H3v1H2zm6-1h2v1h2V3h1V2H8v1zm6 1h1v2h2V5h1V4h-1V3h-1V2h-2v2z' fill-rule='evenodd' fill='${hairColor}'/>",
				"<path d='M3 7h1V5h2V3h8v1h1v1h1v2h1V3h-2V2h-2V1h-1v1h-2V1H9v1H8V1H7v1H5v1H4v1H3v3z' fill='${hairColor}'/>",
				"<path d='M8 2h4v1h-1v1H9V3H8V2z' fill='${hairColor}'/>",
				"<path d='M9 0v1H8v1h4V1h-1V0H9z' fill='${hairColor}'/>",
				"<path d='M3 7h1V5h2V4h2V3h1v1h2v1h2v1h2v1h2V4h-1V3h-1V2H5v1H4v1H3v3z' fill='${hairColor}'/>",
				"<path d='M4 4h12V3h-1V2H5v1H4v1z' fill='${hairColor}'/>",
				"<path d='M2 7h1V5h2V4h1V3h1v1h2V3h4V2h1v1h1v1h1v1h1v2h1V6h1V4h-1V3h-1V2h-1V1h-1V0h-1v1h-2V0h-1v1H9V0H8v1H7V0H5v1H4v1H2v5z' fill='${hairColor}'/>",
				"<path d='M0 7h1v5h1v1h1V9h1V7h1V6h1V4h1V3h7v1h1v1h1v1h1v7h1v-2h1V7h-1V6h1V4h-1v1h-1V3h1V2h-1v1h-1V2h-2V1h-1V0h-1v1H5V0H4v1H3V0H2v1h1v2H2V2H1v1h1v1H1v2H0v1z' fill='${hairColor}'/>",
				"<path d='M5 2v1H4v1H3v3h2V6h1V5h6V4h1V3h1v1h-1v1h1v1h1v1h2V4h-1V3h-1V2H5z' fill='${hairColor}'/>",
			}),
			""),
		// Hat (5% chance)
		g.pickAorB(0.05,
			g.pickOne([]string{
				"<path d='M4 0v2H2v2h16V2h-2V0H4z' fill='${hatColor}'/>",
				"<path d='M4 3H2v1h16V3h-2V0H4v3z' fill='${hatColor}'/>",
				"<path d='M2 2v2h16V2h-1V1h-1V0H4v1H3v1H2z' fill='${hatColor}'/>",
				"<path d='M6 0v1H5v1H4v2h14V3h-2V2h-1V1h-1V0H6z' fill='${hatColor}'/>",
				"<path d='M2 4V2h2V0h12v2h2v2H2z' fill='${hatColor}'/><path d='M4 0v2h12V0H4z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 4V3h2V0h12v3h2v1H2z' fill='${hatColor}'/><path d='M4 0v3h12V0H4z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 4V2h1V1h1V0h12v1h1v1h1v2H2z' fill='${hatColor}'/><path d='M3 1v1h14V1H3zM2 3v1h16V3H2z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M14 0H6v1H5v1H4v2h14V3h-2V2h-1V1h-1V0z' fill='${hatColor}'/><path d='M5 3h1V2h1V1h1V0H7v1H6v1H5v1z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 0v2H2v2h16V2h-2V0H4z' fill='${hatColor}'/><path d='M15 3V0h-1v3h1z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 0v3H2v1h16V3h-2V0H4z' fill='${hatColor}'/><path d='M15 3V0h-1v3h1zm-2-3v2h-1V0h1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 2v2h16V2h-1V1h-1V0H4v1H3v1H2z' fill='${hatColor}'/><path d='M15 0v4h-1V0h1zm-2 0v4h-1V0h1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 2H4v2h14V3h-2V2h-1V1h-1V0H6v1H5v1z' fill='${hatColor}'/><path d='M14 2h-3v1h3V2z' fill='#FFF' fill-opacity='.2'/>",
			}),
			""),
		"</svg>",
	}, "")

	m := []string{
		"${skinColor}", skinColor.html(),
		"${hairColor}", hairColor.html(),
		"${eyesColor}", eyesColor.html(),
		"${eyebrowsColor}", eyebrowsColor.html(),
		"${mustacheColor}", mustacheColor.html(),
		"${mustacheColorAlpha}", strconv.Itoa(int(mustacheColor.a)),
		"${mouthColor}", mouthColor.html(),
		"${glassesColor}", glassesColor.html(),
		"${clothesColor}", clothesColor.html(),
		"${hatColor}", hatColor.html(),
	}
	return strings.NewReplacer(m...).Replace(s)
}

func femaleAvatar(seed uint64, mood string) string {
	var g = linearCongruentialGenerator(seed)

	var skinColor = toRgb(g.pickOne([]string{"#FFDBAC", "#F5CFA0", "#EAC393", "#E0B687", "#CB9E6E", "#B68655", "#A26D3D", "#8D5524"}))
	var hairColor = toRgb(g.pickOne([]string{"#090806", "#2c222b", "#71635a", "#b7a69e", "#d6c4c2", "#cabfb1", "#dcd0ba", "#fff5e1",
		"#e6cea8", "#e5c8a8", "#debc99", "#b89778", "#a56b46", "#b55239", "#8d4a43", "#91553d",
		"#533d32", "#3b3024", "#554838", "#4e433f", "#504444", "#6a4e42", "#a7856a", "#977961"})).brighterOrDarkerThan(skinColor, 17)
	var eyesColor = toRgb(g.pickOne([]string{"#76778b", "#697b94", "#647b90", "#5b7c8b", "#588387"}))
	var eyebrowsColor = hairColor.darkerThan(skinColor, 7).darkerThan(hairColor, 10)
	var accessoriesColor = toRgb(g.pickOne([]string{"#daa520", "#ffd700", "#eee8aa", "#fafad2", "#d3d3d3", "#a9a9a9"}))
	var mouthColor = toRgb(g.pickOne([]string{"#dbac98", "#d29985", "#c98276", "#e35d6a", "#e32153", "#de0f0d"})).brighterOrDarkerThan(skinColor, 10)
	var glassesColor = toRgb(g.pickOne([]string{"#5f705c", "#43677d", "#5e172d", "#ffb67a", "#a04b5d", "#191919", "#323232", "#4b4b4b"}))
	var clothesColor = toRgb(g.pickOne([]string{"#d11141", "#00b159", "#00aedb", "#f37735", "#ffc425", "#740001", "#ae0001", "#eeba30",
		"#96ceb4", "#ffeead", "#ff6f69", "#ffcc5c", "#88d8b0"}))
	var hatColor = toRgb(g.pickOne([]string{"#cc6192", "#2663a3", "#a62116", "#3d8a6b", "#614f8a"}))

	if mood == "" {
		mood = g.pickOne([]string{"sad", "happy", "surprised"})
	}
	var mouth string
	if mood == "sad" {
		mouth = "<path d='M9 11v1H8v1h4v-1h-1v-1H9z' fill='${mouthColor}'/>" +
			"<path d='M11 11v1H9v1H8v-1h1v-1h2z' fill='${mouthColor}'/>" +
			"<path d='M9 12h2v1H9v-1z' fill='${mouthColor}'/>" +
			"<path d='M9 12v1h1v1h1v-2H9z' fill='${mouthColor}'/>"
	} else if mood == "happy" {
		mouth = "<path d='M9 11v2h2v-1h-1v-1H9z' fill='${mouthColor}'/><path d='M11 13v-1h-1v-1H9v1h1v1h1z' fill='#FFF' fill-opacity='.2'/>" +
			"<path d='M10 11v1H9v1h2v-2h-1z' fill='${mouthColor}'/>" +
			"<path d='M8 11v1h1v1h2v-1h1v-1H8z' fill='${mouthColor}'/>" +
			"<path d='M9 12v1h2v-1h1v-1h-1v1H9z' fill='${mouthColor}'/>" +
			"<path d='M8 11v1h1v1h2v-1H9v-1H8z' fill='${mouthColor}'/>" +
			"<path d='M8 12v1h1v1h2v-1h1v-1h-1v-1H9v1H8z' fill='${mouthColor}'/><path d='M9 12v1h2v-1H9z' fill='#FFF'/>" +
			"<path d='M8 12v1h1v1h2v-1h1v-1h-1v-1H9v1H8z' fill='${mouthColor}'/><path d='M9 12v1h2v-1H9z' fill='#FFF' fill-opacity='.2'/>"
	} else if mood == "surprised" {
		mouth = "<path d='M9 12v1h1v-1H9z' fill='${mouthColor}'/>" +
			"<path d='M9 11v2h2v-2H9z' fill='${mouthColor}'/>"
	}

	var s = strings.Join([]string{
		"<svg xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' style='isolation:isolate' viewBox='0 0 20 20' version='1.1' shape-rendering='crispEdges'>",
		// Head
		"<path d='M3 20v-3h1v-1h4v-2H6v-1H5v-1H4v-1H3V9H2V7h1V4h1V3h1V2h10v1h1v1h1v3h1v2h-1v2h-1v1h-1v1h-1v1h-2v2h4v1h1v3H3z' fill='${skinColor}'/><path d='M14 14v-1h1v-1h1v-1h1V9h1V7h-1V4h-1V3h-1V2H5v1H4v1H3v3H2v2h1v2h1v1h1v1h1v1h8z' fill='#FFF' fill-opacity='.1'/>",
		// Eyes
		g.pickOne([]string{
			"<path d='M5 9V7h3v2H5zm7-2h3v2h-3V7z' fill='#FFF'/><path d='M7 8v1h1V8H7zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 1V8h1v1h-1z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M7 8h1v1H7V8zm5 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M6 7h1v1h1v1H6V7zm6 0h1v1h1v1h-2V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm6 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='#FFF'/><path d='M7 8h1v1H7V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='#FFF'/><path d='M7 8h1v1H7V8zm5 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7v1h3V7H5zm7 0h3v1h-3V7z' fill='#FFF'/><path d='M5 9V8h1V7h1v1h1v1H5zm7 0V8h1V7h1v1h1v1h-3z' fill='${eyesColor}'/><path d='M5 9V8h1V7h1v1h1v1H7V8H6v1H5zm7 0V8h1V7h1v1h1v1h-1V8h-1v1h-1z' fill='#FFF' fill-opacity='.5'/>",
			"<path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill='#FFF'/><path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M6 8h1v1H6V8zm7 0h1v1h-1V8z' fill='${eyesColor}'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M6 7h2v2H6V7zm7 0h2v2h-2V7z' fill='${eyesColor}'/><path d='M6 7v1h1v1h1V8H7V7H6zm7 0v1h1v1h1V8h-1V7h-1z' fill='#FFF' fill-opacity='.4'/><path d='M7 7v1h1V7H7zm7 0h1v1h-1V7z' fill='#FFF' fill-opacity='.7'/>",
			"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill='#FFF'/><path d='M5 8h2v1H5V8zm7 0h2v1h-2V8z' fill='${eyesColor}'/><path d='M5 8h1v1H5V8zm7 0h1v1h-1V8z' fill='#FFF' fill-opacity='.7'/>",
			"<path d='M6 7h1v2H5V8h1V7zm7 0h1v2h-2V8h1V7z' fill='#FFF'/><path d='M7 7v1H6v1h2V7H7zm7 0v1h-1v1h2V7h-1z' fill='${eyesColor}'/><path d='M7 7v1h1V7H7zM6 8v1h1V8H6zm8-1v1h1V7h-1zm-1 1v1h1V8h-1z' fill='#FFF' fill-opacity='.5'/>",
		}),
		// Eyebrows
		g.pickOne([]string{
			"<path d='M7 5v1H5v1H4V6h1V5h2zm7 0v1h-2v1h-1V6h1V5h2z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M8 4v1H7v1H5V5h2V4h1zm4 0h1v1h2v1h-2V5h-1V4z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h3v2H8V6H6V5zm5 0h3v1h-2v1h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M5 5h3v1h1v1H8V6H5V5zm10 0h-3v1h-1v1h1V6h3V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5H4v2h1V6h1V5zm8 0h2v2h-1V6h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M5 6h2v1H5V6zm8 0h2v1h-2V6z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h1v1h1v1H5V6h1V5zm7 0h1v1h1v1h-3V6h1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M6 5h2v1h1v1H8V6H6V5zm8 0h-2v1h-1v1h1V6h2V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M12 7V6h1V5h1v1h1v1h-1V6h-1v1h-1zM5 7V6h1V5h1v1h1v1H7V6H6v1H5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M7 5v1H5v1H4V6h1V5h2zm6 0h2v1h1v1h-1V6h-2V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M4 7V5h3v1H5v1H4zm12-2v2h-1V6h-2V5h3z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M7 5h1v1h1v1H8V6H7V5zm6 0v1h-1v1h-1V6h1V5h1z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
			"<path d='M4 7V6h1V5h1v1H5v1H4zm10-2h1v1h1v1h-1V6h-1V5z' fill-rule='evenodd' fill='${eyebrowsColor}'/>",
		}),
		// Accessories (15% chance)
		g.pickAorB(0.15,
			g.pickOne([]string{
				"<path d='M2 9v1h1V9H2zm15 0v1h1V9h-1z' fill-rule='evenodd' fill='${accessoriesColor}'/>",
				"<path d='M2 9v2h1V9H2zm15 0h1v2h-1V9z' fill-rule='evenodd' fill='${accessoriesColor}'/>",
				"<path d='M2 9v2h1V9H2zm15 0h1v2h-1V9z' fill='${accessoriesColor}'/><path d='M2 9v1h1V9H2zm15 0h1v1h-1V9z' fill='#FFF' fill-opacity='.4'/>",
				"<path d='M1 9v3h3V9H1zm1 1v1h1v-1H2zm14-1v3h3V9h-3zm1 1v1h1v-1h-1z' fill-rule='evenodd' fill='${accessoriesColor}'/>",
			}),
			""),
		// Mouth
		mouth,
		// Glasses (25% chance)
		g.pickAorB(0.25,
			g.pickOne([]string{
				"<path d='M3 8V7h1V6h2v1h1V6h2v1h2V6h2v1h1V6h2v1h1v1h-1v1h-1v1h-1v1h-1v-1h-1V9h-1V8H9v1H8v1H7v1H6v-1H5V9H4V8H3z' fill='${glassesColor}'/><path d='M3 7v1h1V7h1V6H4v1H3zm5-1v1h1v1h2V7h1V6h-1v1H9V6H8zm7 0v1h1v1h1V7h-1V6h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 7h3v3H5V7zm7 0h3v3h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M12 10V7h3v3h-3zm-1-4v1H9V6H4v1H3v1h1v3h5V8h2v3h5V8h1V7h-1V6h-5zm-6 4V7h3v3H5z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M5 7v2h3V7H5zM4 6v1H3v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7h-1V6h-5v1H9V6H4zm8 1v2h3V7h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 8h1v1H7V8zm7 0h1v1h-1V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M5 8v1h3V8H5zM3 7v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7H3zm9 1v1h3V8h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7v1h1V7H3zm6 0v1h2V7H9zm7 0v1h1V7h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 7h1v1H7V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M12 7v2h3V7h-3zM8 6H5v1H3v1h1v1h1v1h3V9h1V8h2v1h1v1h3V9h1V8h1V7h-2V6h-3v1H8V6zM5 7v2h3V7H5z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M3 7h1v1H3V7zm6 0h2v1H9V7zm7 0h1v1h-1V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 8H3V7h1V6h5v1h2V6h5v1h1v1h-1v2h-5V8H9v2H4V8zm1 0V7h3v2H5V8zm7-1v2h3V7h-3z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M5 7h3v2H5V7zm7 0h3v2h-3V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M14 7h1v1h-1V7zM7 7h1v1H7V7z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M3 8V7h1v1H3zm6-1v1h2V7H9zm7 0v1h1V7h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 8H3V7h14v1h-1v2h-5V8H9v2H4V8zm1 0h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='${glassesColor}'/><path d='M5 8h3v1H5V8zm7 0h3v1h-3V8z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M7 8v1h1V8H7zm7 0v1h1V8h-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M3 7v1h1V7H3zm13 0v1h1V7h-1zM9 7v1h2V7H9z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			}),
			""),
		// Clothes
		g.pickOne([]string{
			"<path d='M3 20v-3h1v-1h12v1h1v3H3z' fill='${clothesColor}'/>",
			"<path d='M4 16v4h4v-1H7v-1H6v-1H5v-1H4zm12 0v4h-4v-1h1v-1h1v-1h1v-1h1z' fill-rule='evenodd' fill='${clothesColor}'/>",
			"<path d='M5 16h1v2h1v1h1v1H5v-4zm9 0h1v4h-3v-1h1v-1h1v-2z' fill-rule='evenodd' fill='${clothesColor}'/>",
			"<path d='M4 20v-2h1v-1h1v-1h2v1h1v1h2v-1h1v-1h2v1h1v1h1v2H4z' fill='${clothesColor}'/>",
			"<path d='M3 20v-3h1v-1h4v1h1v1h2v-1h1v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M3 20v-3h1v-1h2v1h1v1h1v1h4v-1h1v-1h1v-1h2v1h1v3H3z' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h5v1h2v-1h5v1h1v3H3z' fill='${clothesColor}'/><path d='M3 20v-2h1v1h3v1H3zm14 0v-2h-1v1h-3v1h4z' fill-rule='evenodd' fill='#FFF' fill-opacity='.4'/><path d='M7 16H4v1H3v1h1v1h3v1h6v-1h3v-1h1v-1h-1v-1h-3v1H7v-1z' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h4v1h4v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M15 20h2v-3h-1v-1h-4v1H8v-1H4v1H3v3h2v-2h10v2z' fill='#FFF' fill-opacity='.4'/>",
			"<path d='M3 20v-3h1v-1h4v1h1v1h2v-1h1v-1h4v1h1v3H3z' fill='${clothesColor}'/><path d='M6 16v1h1v1h1v1h4v-1h1v-1h1v-1h-2v1h-1v1H9v-1H8v-1H6z' fill='#FFF' fill-opacity='.4'/><path d='M13 20v-1h2v1h-2zm1-4v1h2v-1h-2zm-8 0H4v1h2v-1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
			"<path d='M3 20v-3h1v-1h3v1h1v1h1v1h2v-1h1v-1h1v-1h3v1h1v3H3z' fill='${clothesColor}'/>",
			"<path d='M3 20v-3h1v-1h2v1h1v1h1v1h1v1H3zm14 0v-3h-1v-1h-2v1h-1v1h-1v1h-1v1h6z' fill-rule='evenodd' fill='${clothesColor}'/>",
			"<path d='M4 16v4h4v-1H7v-1H6v-2H4zM16 20v-4h-2v2h-1v1h-1v1h4z' fill='${clothesColor}'/>",
			"<path d='M3 20v-3h1v-1h3v1h1v1h1v1h2v-1h1v-1h1v-1h3v1h1v3H3z' fill='${clothesColor}'/><path d='M6 16v1h1v1h1v1h1v1h2v-1h1v-1h1v-1h1v-1h-1v1h-1v1h-1v1H9v-1H8v-1H7v-1H6z' fill='#FFF' fill-opacity='.4'/><path d='M15 16v1h-1v1h-1v1h-1v1h-1v-1h1v-1h1v-1h1v-1h1zM5 16v1h1v1h1v1h1v1h1v-1H8v-1H7v-1H6v-1H5z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/><path d='M3 20h1v-3h1v1h1v1h1v1h1v-1H7v-1H6v-1H5v-1H4v1H3v3zm14 0v-3h-1v-1h-1v1h-1v1h-1v1h-1v1h1v-1h1v-1h1v-1h1v3h1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.1'/>",
			"<path d='M3 20v-3h1v-1h5v1h2v-1h5v1h1v3H3z' fill='${clothesColor}'/><path d='M3 17h14v1H3v-1zm0 2v1h14v-1H3z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
		}),
		// Hair
		g.pickOne([]string{
			"<path d='M2 9v6h2v-4H3V9H2zm0-2h2V4h12v3h2V3h-1V2H3v1H2v4zm15 2h1v6h-2v-4h1V9z' fill-rule='evenodd' fill='${hairColor}'/>",
			"<path d='M4 12h1v1H3V4h1V3h1V2h10v1h1v1h1v9h-2v-1h1V5H4v7z' fill='${hairColor}'/>",
			"<path d='M2 17h2v-1h4v-2H6v-1H5v-1H4V4h1V3h1v1h1V3h1v1h1V3h6v1h1v8h-1v1h-1v1h-2v2h4v1h2V3h-1V2h-1V1H4v1H3v1H2v14z' fill='${hairColor}'/>",
			"<path d='M2 13V9h1v2h1v1h1v1H2zm15-4h1v4h-3v-1h1v-1h1V9zM2 7h1V4h1V3h1V2h10v1h1v1h1v3h1V3h-1V2h-1V1H4v1H3v1H2v4z' fill-rule='evenodd' fill='${hairColor}'/>",
			"<path d='M2 14h2V4h12v10h2V3h-1V2h-1V1H4v1H3v1H2v11z' fill='${hairColor}'/>",
			"<path d='M2 14h2V7h1V6h1V5h1V4h1V3h6v1h1v1h1v9h2V3h-1V2h-1V1H4v1H3v1H2v11z' fill='${hairColor}'/>",
			"<path d='M3 4v3h1V4h2v1h1V4h1v1h1V4h7v3h1V4h1V2h-1V1h-2v1H5V1H3v1H2v2h1z' fill='${hairColor}'/>",
			"<path d='M2 10h1V8h1V7h1V6h1V5h1V4h7v1h3V4h-1V3h-1V2H4v1H3v1H2v6z' fill='${hairColor}'/>",
			"<path d='M1 17h3v-1h4v-2H6v-1H5v-1H4v-1H3V9H2V7h1V5h1V4h7V3h1v1h1V3h1v1h1V3h1v2h1v2h1v2h-1v2h-1v1h-1v1h-1v1h-2v2h4v1h3V4h-1V3h-1V2h-1V1H4v1H3v1H2v1H1v13z' fill='${hairColor}'/>",
			"<path d='M2 13h3v-1H4v-1H3V9H2v4zm13 0h3V9h-1v2h-1v1h-1v1zm2-6h1V3h-1V2h-1V1H4v1H3v1H2v4h1V5h1V4h1V3h1V2h8v1h1v1h1v1h1v2z' fill-rule='evenodd' fill='${hairColor}'/>",
			"<path d='M2 9v3h1v1H2v2H1v1h1v-1h1v-1h1v-1h1v-1H4v-1H3V9H2zm0-2h1V6h1V5h2V4h9v1h1v1h1v1h1V3h-1V2h-1V1H4v1H3v1H2v4z' fill-rule='evenodd' fill='${hairColor}'/>",
			"<path d='M1 15h5v-2H5v-1H4v-1H3V6h2V5h2V4h1V3h6v1h1v1h1v1h1v5h-1v1h-1v1h-1v2h5V4h-1V3h-1V2h-1V1H4v1H3v1H2v1H1v11z' fill='${hairColor}'/>",
			"<path d='M2 9v3h1v1H2v2H1v1h1v-1h1v-1h1v-1h1v-1H4v-1H3V9H2zm16 0v3h-1v1h1v2h1v1h-1v-1h-1v-1h-1v-1h-1v-1h1v-1h1V9h1zm-1-2h1V3h-1V2h-1V1H4v1H3v1H2v4h1V6h1V5h2V4h9v1h1v1h1v1z' fill-rule='evenodd' fill='${hairColor}'/>",
		}),
		// Hat (5% chance)
		g.pickAorB(0.05,
			g.pickOne([]string{
				"<path d='M4 0v2H2v2h16V2h-2V0H4z' fill='${hatColor}'/>",
				"<path d='M4 3H2v1h16V3h-2V0H4v3z' fill='${hatColor}'/>",
				"<path d='M2 2v2h16V2h-1V1h-1V0H4v1H3v1H2z' fill='${hatColor}'/>",
				"<path d='M6 0v1H5v1H4v2h14V3h-2V2h-1V1h-1V0H6z' fill='${hatColor}'/>",
				"<path d='M2 4V2h2V0h12v2h2v2H2z' fill='${hatColor}'/><path d='M4 0v2h12V0H4z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 4V3h2V0h12v3h2v1H2z' fill='${hatColor}'/><path d='M4 0v3h12V0H4z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 4V2h1V1h1V0h12v1h1v1h1v2H2z' fill='${hatColor}'/><path d='M3 1v1h14V1H3zM2 3v1h16V3H2z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M14 0H6v1H5v1H4v2h14V3h-2V2h-1V1h-1V0z' fill='${hatColor}'/><path d='M5 3h1V2h1V1h1V0H7v1H6v1H5v1z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 0v2H2v2h16V2h-2V0H4z' fill='${hatColor}'/><path d='M15 3V0h-1v3h1z' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M4 0v3H2v1h16V3h-2V0H4z' fill='${hatColor}'/><path d='M15 3V0h-1v3h1zm-2-3v2h-1V0h1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M2 2v2h16V2h-1V1h-1V0H4v1H3v1H2z' fill='${hatColor}'/><path d='M15 0v4h-1V0h1zm-2 0v4h-1V0h1z' fill-rule='evenodd' fill='#FFF' fill-opacity='.2'/>",
				"<path d='M5 2H4v2h14V3h-2V2h-1V1h-1V0H6v1H5v1z' fill='${hatColor}'/><path d='M14 2h-3v1h3V2z' fill='#FFF' fill-opacity='.2'/>",
			}),
			""),
		"</svg>",
	}, "")

	m := []string{
		"${skinColor}", skinColor.html(),
		"${hairColor}", hairColor.html(),
		"${eyesColor}", eyesColor.html(),
		"${eyebrowsColor}", eyebrowsColor.html(),
		"${accessoriesColor}", accessoriesColor.html(),
		"${mouthColor}", mouthColor.html(),
		"${glassesColor}", glassesColor.html(),
		"${clothesColor}", clothesColor.html(),
		"${hatColor}", hatColor.html(),
	}
	return strings.NewReplacer(m...).Replace(s)
}
