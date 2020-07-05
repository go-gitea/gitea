// Although inspired by the https://github.com/bluef/gitgraph.js/blob/master/gitgraph.js
// this has been completely rewritten with almost no remaining code

// GitGraphCanvas is a canvas for drawing gitgraphs on to
class GitGraphCanvas {
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
  drawSymbol(symbol, columnNumber, rowNumber, color) {
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

class GitGraph {
  constructor(canvas, rawRows, config) {
    this.rows = [];
    let maxWidth = 0;

    for (let i = 0; i < rawRows.length; i++) {
      const rowStr = rawRows[i];
      maxWidth = Math.max(rowStr.replace(/([_\s.-])/g, '').length, maxWidth);

      const rowArray = rowStr.split('');

      this.rows.unshift(rowArray);
    }

    this.currentFlows = [];
    this.previousFlows = [];

    this.gitGraphCanvas = new GitGraphCanvas(
      canvas,
      maxWidth,
      this.rows.length,
      config
    );
  }

  generateNewFlow(column) {
    let newId;

    do {
      newId = generateRandomColorString();
    } while (this.hasFlow(newId, column));

    return {id: newId, color: `#${newId}`};
  }

  hasFlow(id, column) {
    // We want to find the flow with the current ID
    // Possible flows are those in the currentFlows
    // Or flows in previousFlows[column-2:...]
    for (
      let idx = column - 2 < 0 ? 0 : column - 2;
      idx < this.previousFlows.length;
      idx++
    ) {
      if (this.previousFlows[idx] && this.previousFlows[idx].id === id) {
        return true;
      }
    }
    for (let idx = 0; idx < this.currentFlows.length; idx++) {
      if (this.currentFlows[idx] && this.currentFlows[idx].id === id) {
        return true;
      }
    }
    return false;
  }

  takePreviousFlow(column) {
    if (column < this.previousFlows.length && this.previousFlows[column]) {
      const flow = this.previousFlows[column];
      this.previousFlows[column] = null;
      return flow;
    }
    return this.generateNewFlow(column);
  }

  draw() {
    if (this.rows.length === 0) {
      return;
    }

    this.currentFlows = new Array(this.rows[0].length);

    // Generate flows for the first row - I do not believe that this can contain '_', '-', '.'
    for (let column = 0; column < this.rows[0].length; column++) {
      if (this.rows[0][column] === ' ') {
        continue;
      }
      this.currentFlows[column] = this.generateNewFlow(column);
    }

    // Draw the first row
    for (let column = 0; column < this.rows[0].length; column++) {
      const symbol = this.rows[0][column];
      const color = this.currentFlows[column] ? this.currentFlows[column].color : '';
      this.gitGraphCanvas.drawSymbol(symbol, column, 0, color);
    }

    for (let row = 1; row < this.rows.length; row++) {
      // Done previous row - step up the row
      const currentRow = this.rows[row];
      const previousRow = this.rows[row - 1];

      this.previousFlows = this.currentFlows;
      this.currentFlows = new Array(currentRow.length);

      // Set flows for this row
      for (let column = 0; column < currentRow.length; column++) {
        column = this.setFlowFor(column, currentRow, previousRow);
      }

      // Draw this row
      for (let column = 0; column < currentRow.length; column++) {
        const symbol = currentRow[column];
        const color = this.currentFlows[column] ? this.currentFlows[column].color : '';
        this.gitGraphCanvas.drawSymbol(symbol, column, row, color);
      }
    }
  }

  setFlowFor(column, currentRow, previousRow) {
    const symbol = currentRow[column];
    switch (symbol) {
      case '|':
      case '*':
        return this.setUpFlow(column, currentRow, previousRow);
      case '/':
        return this.setOutFlow(column, currentRow, previousRow);
      case '\\':
        return this.setInFlow(column, currentRow, previousRow);
      case '_':
        return this.setRightFlow(column, currentRow, previousRow);
      case '-':
        return this.setLeftFlow(column, currentRow, previousRow);
      case ' ':
        // In space no one can hear you flow ... (?)
        return column;
      default:
        // Unexpected so let's generate a new flow and wait for bug-reports
        this.currentFlows[column] = this.generateNewFlow(column);
        return column;
    }
  }

  // setUpFlow handles '|' or '*' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setUpFlow(column, currentRow, previousRow) {
    // If ' |/' or ' |_'
    //    '/|'     '/|'  -> Take the '|' flow directly beneath us
    if (
      column + 1 < currentRow.length &&
      (currentRow[column + 1] === '/' || currentRow[column + 1] === '_') &&
      column < previousRow.length &&
      (previousRow[column] === '|' || previousRow[column] === '*') &&
      previousRow[column - 1] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column);
      return column;
    }

    // If ' |/' or ' |_'
    //    '/ '     '/ '  -> Take the '/' flow from the preceding column
    if (
      column + 1 < currentRow.length &&
      (currentRow[column + 1] === '/' || currentRow[column + 1] === '_') &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 1);
      return column;
    }

    // If ' |'
    //    '/'   ->  Take the '/' flow - (we always prefer the left-most flow)
    if (
      column > 0 &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 1);
      return column;
    }

    // If '|' OR '|' take the '|' flow
    //    '|'    '*'
    if (
      column < previousRow.length &&
      (previousRow[column] === '|' || previousRow[column] === '*')
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column);
      return column;
    }

    // If '| ' keep the '\' flow
    //    ' \'
    if (column + 1 < previousRow.length && previousRow[column + 1] === '\\') {
      this.currentFlows[column] = this.takePreviousFlow(column + 1);
      return column;
    }

    // Otherwise just create a new flow - probably this is an error...
    this.currentFlows[column] = this.generateNewFlow(column);
    return column;
  }

  // setOutFlow handles '/' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setOutFlow(column, currentRow, previousRow) {
    // If  '_/' -> keep the '_' flow
    if (column > 0 && currentRow[column - 1] === '_') {
      this.currentFlows[column] = this.currentFlows[column - 1];
      return column;
    }

    // If '_|/' -> keep the '_' flow
    if (
      column > 1 &&
      (currentRow[column - 1] === '|' || currentRow[column - 1] === '*') &&
      currentRow[column - 2] === '_'
    ) {
      this.currentFlows[column] = this.currentFlows[column - 2];
      return column;
    }

    // If  '|/'
    //    '/'   -> take the '/' flow (if it is still available)
    if (
      column > 1 &&
      currentRow[column - 1] === '|' &&
      column - 2 < previousRow.length &&
      previousRow[column - 2] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 2);
      return column;
    }

    // If ' /'
    //    '/'  -> take the '/' flow, but transform the symbol to '|' due to our spacing
    // This should only happen if there are 3 '/' - in a row so we don't need to be cleverer here
    if (
      column > 0 &&
      currentRow[column - 1] === ' ' &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 1);
      currentRow[column] = '|';
      return column;
    }

    // If ' /'
    //    '|'  -> take the '|' flow
    if (
      column > 0 &&
      currentRow[column - 1] === ' ' &&
      column - 1 < previousRow.length &&
      (previousRow[column - 1] === '|' || previousRow[column - 1] === '*')
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 1);
      return column;
    }

    // If '/' <- Not sure this ever happens... but take the '\' flow
    //    '\'
    if (column < previousRow.length && previousRow[column] === '\\') {
      this.currentFlows[column] = this.takePreviousFlow(column);
      return column;
    }

    // Otherwise just generate a new flow and wait for bug-reports...
    this.currentFlows[column] = this.generateNewFlow(column);
    return column;
  }

  // setInFlow handles '\' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setInFlow(column, currentRow, previousRow) {
    // If '\?'
    //    '/?' -> take the '/' flow
    if (column < previousRow.length && previousRow[column] === '/') {
      this.currentFlows[column] = this.takePreviousFlow(column);
      return column;
    }

    // If '\?'
    //    ' \' -> take the '\' flow and reassign to '|'
    // This should only happen if there are 3 '\' - in a row so we don't need to be cleverer here
    if (column + 1 < previousRow.length && previousRow[column + 1] === '\\') {
      this.currentFlows[column] = this.takePreviousFlow(column + 1);
      currentRow[column] = '|';
      return column;
    }

    // If '\?'
    //    ' |' -> take the '|' flow
    if (
      column + 1 < previousRow.length &&
      (previousRow[column + 1] === '|' || previousRow[column + 1] === '*')
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column + 1);
      return column;
    }

    // Otherwise just generate a new flow and wait for bug-reports if we're wrong...
    this.currentFlows[column] = this.generateNewFlow(column);
    return column;
  }

  // setRightFlow handles '_' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setRightFlow(column, currentRow, previousRow) {
    // if '__' keep the '_' flow
    if (column > 0 && currentRow[column - 1] === '_') {
      this.currentFlows[column] = this.currentFlows[column - 1];
      return column;
    }

    // if '_|_' -> keep the '_' flow
    if (
      column > 1 &&
      currentRow[column - 1] === '|' &&
      currentRow[column - 2] === '_'
    ) {
      this.currentFlows[column] = this.currentFlows[column - 2];
      return column;
    }

    // if ' _' -> take the '/' flow
    //    '/ '
    if (
      column > 0 &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 1);
      return column;
    }

    // if ' |_'
    //    '/? ' -> take the '/' flow (this may cause generation...)
    //             we can do this because we know that git graph
    //             doesn't create compact graphs like: ' |_'
    //                                                 '//'
    if (
      column > 1 &&
      column - 2 < previousRow.length &&
      previousRow[column - 2] === '/'
    ) {
      this.currentFlows[column] = this.takePreviousFlow(column - 2);
      return column;
    }

    // There really shouldn't be another way of doing this - generate and wait for bug-reports...

    this.currentFlows[column] = this.generateNewFlow(column);
    return column;
  }

  // setLeftFlow handles '----.' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row that terminates this left recursion
  setLeftFlow(column, currentRow, previousRow) {
    // This is: '----------.' or the like
    //          '   \  \  /|\'

    // Find the end of the '-' or nearest '/|\' in the previousRow :
    let originalColumn = column;
    let flow;
    for (; column < currentRow.length && currentRow[column] === '-'; column++) {
      if (column > 0 && column - 1 < previousRow.length && previousRow[column - 1] === '/') {
        flow = this.takePreviousFlow(column - 1);
        break;
      } else if (column < previousRow.length && previousRow[column] === '|') {
        flow = this.takePreviousFlow(column);
        break;
      } else if (
        column + 1 < previousRow.length &&
        previousRow[column + 1] === '\\'
      ) {
        flow = this.takePreviousFlow(column + 1);
        break;
      }
    }

    // if we have a flow then we found a '/|\' in the previousRow
    if (flow) {
      for (; originalColumn < column + 1; originalColumn++) {
        this.currentFlows[originalColumn] = flow;
      }
      return column;
    }

    // If the symbol in the column is not a '.' then there's likely an error
    if (currentRow[column] !== '.') {
      // It really should end in a '.' but this one doesn't...
      // 1. Step back - we don't want to eat this column
      column--;
      // 2. Generate a new flow and await bug-reports...
      this.currentFlows[column] = this.generateNewFlow(column);

      // 3. Assign all of the '-' to the same flow.
      for (; originalColumn < column; originalColumn++) {
        this.currentFlows[originalColumn] = this.currentFlows[column];
      }
      return column;
    }

    // We have a terminal '.' eg. the current row looks like '----.'
    // the previous row should look like one of '/|\' eg.    '     \'
    if (column > 0 && column - 1 < previousRow.length && previousRow[column - 1] === '/') {
      flow = this.takePreviousFlow(column - 1);
    } else if (column < previousRow.length && previousRow[column] === '|') {
      flow = this.takePreviousFlow(column);
    } else if (
      column + 1 < previousRow.length &&
      previousRow[column + 1] === '\\'
    ) {
      flow = this.takePreviousFlow(column + 1);
    } else {
      // Again unexpected so let's generate and wait the bug-report
      flow = this.generateNewFlow(column);
    }

    // Assign all of the rest of the ----. to this flow.
    for (; originalColumn < column + 1; originalColumn++) {
      this.currentFlows[originalColumn] = flow;
    }

    return column;
  }
}

function generateRandomColorString() {
  const chars = '0123456789ABCDEF';
  const stringLength = 6;
  let randomString = '',
    rnum,
    i;
  for (i = 0; i < stringLength; i++) {
    rnum = Math.floor(Math.random() * chars.length);
    randomString += chars.substring(rnum, rnum + 1);
  }

  return randomString;
}

export default async function initGitGraph() {
  const graphCanvas = document.getElementById('graph-canvas');
  if (!graphCanvas || !graphCanvas.getContext) return;

  // Grab the raw graphList
  const graphList = [];
  $('#graph-raw-list li span.node-relation').each(function () {
    graphList.push($(this).text());
  });

  // Define some drawing parameters
  const config = {
    unitSize: 20,
    lineWidth: 3,
    nodeRadius: 4
  };


  const gitGraph = new GitGraph(graphCanvas, graphList, config);
  gitGraph.draw();
  graphCanvas.closest('#git-graph-container').classList.add('in');
}
