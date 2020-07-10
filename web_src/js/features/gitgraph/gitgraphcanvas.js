// GitGraphCanvas is a canvas for drawing gitgraphs on to
export class GitGraphCanvas {
  constructor(canvas, widthUnits, heightUnits, config) {
    this.ctx = canvas.getContext('2d');

    const width = widthUnits * config.unitSize;
    this.height = heightUnits * config.unitSize;

    const ratio = window.devicePixelRatio || 1;

    canvas.width = width * ratio;
    canvas.height = this.height * ratio;

    canvas.style.width = `${width}px`;
    canvas.style.height = `${this.height}px`;

    this.ctx.lineWidth = config.lineWidth;
    this.ctx.lineJoin = 'round';
    this.ctx.lineCap = 'round';

    this.ctx.scale(ratio, ratio);
    this.config = config;
  }
  drawLine(moveX, moveY, lineX, lineY, color) {
    this.ctx.strokeStyle = color;
    this.ctx.beginPath();
    this.ctx.moveTo(moveX, moveY);
    this.ctx.lineTo(lineX, lineY);
    this.ctx.stroke();
  }
  drawLineRight(x, y, color) {
    this.drawLine(
      x - 0.5 * this.config.unitSize,
      y + this.config.unitSize / 2,
      x + 0.5 * this.config.unitSize,
      y + this.config.unitSize / 2,
      color
    );
  }
  drawLineUp(x, y, color) {
    this.drawLine(
      x,
      y + this.config.unitSize / 2,
      x,
      y - this.config.unitSize / 2,
      color
    );
  }
  drawNode(x, y, color) {
    this.ctx.strokeStyle = color;

    this.drawLineUp(x, y, color);

    this.ctx.beginPath();
    this.ctx.arc(x, y, this.config.nodeRadius, 0, Math.PI * 2, true);
    this.ctx.fillStyle = color;
    this.ctx.fill();
  }
  drawLineIn(x, y, color) {
    this.drawLine(
      x + 0.5 * this.config.unitSize,
      y + this.config.unitSize / 2,
      x - 0.5 * this.config.unitSize,
      y - this.config.unitSize / 2,
      color
    );
  }
  drawLineOut(x, y, color) {
    this.drawLine(
      x - 0.5 * this.config.unitSize,
      y + this.config.unitSize / 2,
      x + 0.5 * this.config.unitSize,
      y - this.config.unitSize / 2,
      color
    );
  }
  drawSymbol(symbol, rowNumber, columnNumber, color) {
    const y = this.height - this.config.unitSize * (rowNumber + 0.5);
    const x = this.config.unitSize * 0.5 * (columnNumber + 1);
    switch (symbol) {
      case '-':
        if (columnNumber % 2 === 1) {
          this.drawLineRight(x, y, color);
        }
        break;
      case '_':
        this.drawLineRight(x, y, color);
        break;
      case '*':
        this.drawNode(x, y, color);
        break;
      case '|':
        this.drawLineUp(x, y, color);
        break;
      case '/':
        this.drawLineOut(x, y, color);
        break;
      case '\\':
        this.drawLineIn(x, y, color);
        break;
      case '.':
      case ' ':
        break;
      default:
        console.error('Unknown symbol', symbol, color);
    }
  }
}
