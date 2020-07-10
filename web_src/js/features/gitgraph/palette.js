export class RandomHuePalette {
  constructor(palette) {
    if (palette) {
      this.palette = palette;
    } else {
      this.palette = [];
    }
  }
  colors() {
    return this.palette.slice(0);
  }
  generateNewColor() {
    let i = 0;
    while (i < 50 || this.palette.length < 30) {
      const hue = Math.floor(Math.random() * 360);
      const saturation = Math.floor(60 + Math.random() * 40);
      const luminance = Math.floor(50 + Math.random() * 20);

      const randomColor = `hsl(${hue},${saturation}%,${luminance}%)`;
      let found = false;
      for (const pColor of this.palette) {
        if (pColor === randomColor) {
          found = true;
          break;
        }
      }
      if (!found) {
        this.palette.push(randomColor);
        return randomColor;
      }
      i++;
    }
    return null;
  }
  generateNewColorWithFallback() {
    let color = this.generateNewColor();
    const alternate = this.generateNewColor();
    if (color) {
      if (!alternate) {
        this.generateNewColor();
      }
      return color;
    }
    if (alternate) {
      this.generateNewColor();
      return alternate;
    }
    color = this.generateNewColor();
    if (color) {
      this.generateNewColor();
      return color;
    }
    // Otherwise just take the first color from the palette and push it to the back
    color = this.palette.splice(0, 1);
    this.palette.push(color);
    return color;
  }
}
