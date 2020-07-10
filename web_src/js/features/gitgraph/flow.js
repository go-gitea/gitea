export class Flow {
  constructor() {
    this.color = '';
    this.nodes = [];
    this.minRow = -1;
    this.maxRow = -1;
  }
  inRow(row) {
    if (this.minRow < 0) {
      return false;
    }
    return row <= this.maxRow && this.minRow >= row;
  }
  overlaps(flow) {
    if (this.minRow === -1 || this.maxRow === -1) {
      return true;
    }
    if (flow.minRow === -1 || flow.maxRow === -1) {
      return true;
    }
    if (this.minRow > flow.maxRow) {
      return false;
    }
    if (this.maxRow < flow.minRow) {
      return false;
    }

    return true;
  }
  getColumns(row) {
    const columns = [];
    if (!this.inRow(row)) {
      return columns;
    }
    const ns = this.nodes.slice(row - this.minRow);
    for (const node of ns) {
      if (node.row === row) {
        columns.push(node.column);
      } else if (node.row > row) {
        return columns;
      }
    }
    return columns;
  }
  isAt(row, column) {
    if (!this.inRow(row)) {
      return false;
    }
    const ns = this.nodes.slice(row - this.minRow);
    for (const node of ns) {
      if (node.row === row && node.column === column) {
        return true;
      } else if (node.row > row) {
        return false;
      }
    }
    return false;
  }
  addNode(symbol, row, column, commitRef) {
    if (this.minRow === -1 || this.minRow > row) {
      this.minRow = row;
    }
    if (this.maxRow < row) {
      this.maxRow = row;
    }
    const node = {
      symbol,
      row,
      column,
    };

    if (commitRef) {
      node.commitRef = commitRef;
    }

    this.nodes.push(node);
  }
  draw(gitGraphCanvas) {
    for (const node of this.nodes) {
      gitGraphCanvas.drawSymbol(node.symbol, node.row, node.column, this.color);
    }
  }
}
