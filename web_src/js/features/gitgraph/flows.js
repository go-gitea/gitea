import {Flow} from './flow.js';

export class Flows {
  constructor() {
    this.maxWidth = -1;
    this.flows = [];
    this.refsToFlows = [];
    this.minRow = -1;
    this.maxRow = -1;
  }
  createNewFlow() {
    const flow = new Flow();
    this.flows.push(flow);
    return flow;
  }
  addNode(flow, symbol, row, column, commitRef) {
    if (this.minRow === -1 || this.minRow > row) {
      this.minRow = row;
    }
    if (this.maxRow < row) {
      this.maxRow = row;
    }
    if (commitRef) {
      this.refsToFlows[commitRef] = {
        flow,
        row,
        column,
      };
    }
    flow.addNode(symbol, row, column, commitRef);
  }
  assignColors(palette, top, colorOrder, commitRef, commitRefColorOrder) {
    let currentPalette = palette.colors();
    const coloredFlows = [];
    // Pre-assign pinned colors
    if (colorOrder) {
      let targetRow = 0;
      if (commitRef && this.refsToFlows[commitRef]) {
        targetRow = this.refsToFlows[commitRef].row;
        colorOrder = commitRefColorOrder;
      } else if (top) {
        targetRow = this.maxRow;
      }

      const columns = [];
      for (const flow of this.flows) {
        const columns = flow.getColumns(targetRow);
        for (const column of columns) {
          columns[column] = flow;
        }
      }
      for (let i = 0; i < colorOrder.length; i++) {
        const flow = columns[i];
        const color = colorOrder[i];
        if (flow && color) {
          flow.color = color;
          coloredFlows.push(flow);
          let found = false;
          for (const pColor of currentPalette) {
            if (pColor === color) {
              found = true;
              break;
            }
          }
          if (!found) {
            currentPalette.push(color);
          }
        }
      }
    }

    // Now sort the flows by presence in rows
    this.flows.sort(this.flowComparatorLongestFirst);
    coloredFlows.sort(this.flowComparatorEarliestFirst);

    for (const flow of this.flows) {
      if (flow.color) {
        continue;
      }

      const availablePalette = currentPalette.slice(0);
      const removed = [];
      for (const cFlow of coloredFlows) {
        if (cFlow.overlaps(flow)) {
          for (let i = 0; i < availablePalette.length; i++) {
            const pColor = availablePalette[i];
            if (pColor === cFlow.color) {
              removed.push(availablePalette.splice(i, 1)[0]);
              break;
            }
          }
        }
      }

      if (availablePalette.length === 0) {
        // generate another color
        flow.color = palette.generateNewColorWithFallback();
      } else {
        flow.color = availablePalette[0];
        currentPalette = availablePalette.splice(1);
        if (currentPalette.length === 0) {
          const newcolor = palette.generateNewColor();
          if (newcolor) {
            currentPalette.push(newcolor);
          }
        }
        currentPalette = currentPalette.concat(removed);
        currentPalette.push(flow.color);
      }
      coloredFlows.push(flow);
    }
  }
  flowComparatorEarliestFirst(a, b) {
    if (a.minRow !== b.minRow) {
      return a.minRow - b.minRow;
    }
    if (a.maxRow !== b.maxRow) {
      return a.maxRow - b.maxRow;
    }
    return this.flowComparatorLeftFirst(a, b);
  }
  flowComparatorLongestFirst(a, b) {
    if (a.minRow !== b.minRow) {
      return a.minRow - b.minRow;
    }
    if (a.maxRow !== b.maxRow) {
      return b.maxRow - a.maxRow;
    }

    return this.flowComparatorLeftFirst(a, b);
  }
  flowComparatorLeftFirst(a, b) {
    const columnsA = a.getColumns(a.minRow);
    const columnsB = b.getColumns(b.minRow);
    if (columnsA.length === 0 && columnsB.length === 0) {
      return 0;
    } else if (columnsA.length === 0) {
      return 1;
    } else if (columnsB.length === 0) {
      return -1;
    }

    let minA = columnsA[0];
    let minB = columnsB[0];
    for (const currA of columnsA.slice(1)) {
      if (minA < currA) {
        minA = currA;
      }
    }
    for (const currB of columnsB.slice(1)) {
      if (minB < currB) {
        minB = currB;
      }
    }
    return minA - minB;
  }
  draw(gitGraphCanvas) {
    for (const flow of this.flows) {
      flow.draw(gitGraphCanvas);
    }
  }
}
