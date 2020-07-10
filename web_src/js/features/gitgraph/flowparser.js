import {Flows} from './flows.js';

export class FlowParser {
  constructor(rawRows) {
    this.rows = [];
    this.row = 0;
    this.flows = new Flows();

    for (let i = 0; i < rawRows.length; i++) {
      const rowStr = rawRows[i];
      this.flows.maxWidth = Math.max(rowStr.replace(/([_\s.-])/g, '').length, this.flows.maxWidth);

      const rowArray = rowStr.split('');

      this.rows.unshift(rowArray);
    }

    this.currentFlows = [];
    this.previousFlows = [];
  }
  createNewFlowAndAddNode(column) {
    this.currentFlows[column] = this.flows.createNewFlow();
    return this.addNode(column);
  }
  takePreviousFlowAndAddNode(column, prevColumn) {
    this.currentFlows[column] = this.takePreviousFlow(prevColumn);
    return this.addNode(column);
  }
  takeCurrentFlowAndAddNode(column, prevColumn) {
    this.currentFlows[column] = this.currentFlows[prevColumn];
    return this.addNode(column);
  }
  addNode(column) {
    const symbol = this.rows[this.row][column];
    this.flows.addNode(this.currentFlows[column], symbol, this.row, column);
    return this.currentFlows[column];
  }
  takePreviousFlow(column) {
    if (column < this.previousFlows.length && this.previousFlows[column]) {
      const flow = this.previousFlows[column];
      this.previousFlows[column] = null;
      return flow;
    }
    return this.flows.createNewFlow();
  }
  generateFlows() {
    if (this.rows.length === 0) {
      return this.flows;
    }

    this.currentFlows = new Array(this.rows[0].length);
    this.row = 0;

    // Generate flows for the first row - I do not believe that this can contain '_', '-', '.'
    for (let column = 0; column < this.rows[0].length; column++) {
      if (this.rows[0][column] === ' ') {
        continue;
      }
      this.createNewFlowAndAddNode(column);
    }

    for (this.row = 1; this.row < this.rows.length; this.row++) {
      // Done previous row - step up the row
      const currentRow = this.rows[this.row];
      const previousRow = this.rows[this.row - 1];

      this.previousFlows = this.currentFlows;
      this.currentFlows = new Array(currentRow.length);

      // Set flows for this row
      for (let column = 0; column < currentRow.length; column++) {
        column = this.setFlowFor(column, currentRow, previousRow);
      }
    }
    return this.flows;
  }

  draw() {
    if (this.rows.length === 0 || this.flows.length === 0) {
      return;
    }
    this.flows.draw(this.gitGraphCanvas);
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
        this.createNewFlowAndAddNode(column);
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
      this.takePreviousFlowAndAddNode(column, column);
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
      this.takePreviousFlowAndAddNode(column, column - 1);
      return column;
    }

    // If in trunk mode
    // If '|' OR '|' take the '|' flow
    //    '|'    '*'
    if (
      this.trunkMode &&
      column < previousRow.length &&
      (previousRow[column] === '|' || previousRow[column] === '*')
    ) {
      this.takePreviousFlowAndAddNode(column, column);
      return column;
    }

    // If ' |'
    //    '/'   ->  Take the '/' flow - (we always prefer the left-most flow)
    if (
      column > 0 &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.takePreviousFlowAndAddNode(column, column - 1);
      return column;
    }

    // If '|' OR '|' take the '|' flow
    //    '|'    '*'
    if (
      column < previousRow.length &&
      (previousRow[column] === '|' || previousRow[column] === '*')
    ) {
      this.takePreviousFlowAndAddNode(column, column);
      return column;
    }

    // If '| ' keep the '\' flow
    //    ' \'
    if (column + 1 < previousRow.length && previousRow[column + 1] === '\\') {
      this.takePreviousFlowAndAddNode(column, column + 1);
      return column;
    }

    // Otherwise just create a new flow - probably this is an error...
    this.createNewFlowAndAddNode(column);
    return column;
  }

  // setOutFlow handles '/' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setOutFlow(column, currentRow, previousRow) {
    // If  '_/' -> keep the '_' flow
    if (column > 0 && currentRow[column - 1] === '_') {
      this.takeCurrentFlowAndAddNode(column, column - 1);
      return column;
    }

    // If '_|/' -> keep the '_' flow
    if (
      column > 1 &&
      (currentRow[column - 1] === '|' || currentRow[column - 1] === '*') &&
      currentRow[column - 2] === '_'
    ) {
      this.takeCurrentFlowAndAddNode(column, column - 2);
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
      this.takePreviousFlowAndAddNode(column, column - 2);
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
      currentRow[column] = '|';
      this.takePreviousFlowAndAddNode(column, column - 1);
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
      this.takePreviousFlowAndAddNode(column, column - 1);
      return column;
    }

    // If '/' <- Not sure this ever happens... but take the '\' flow
    //    '\'
    if (column < previousRow.length && previousRow[column] === '\\') {
      this.takePreviousFlowAndAddNode(column, column);
      return column;
    }

    // Otherwise just generate a new flow and wait for bug-reports...
    this.createNewFlowAndAddNode(column);
    return column;
  }

  // setInFlow handles '\' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setInFlow(column, currentRow, previousRow) {
    // If '\?'
    //    '/?' -> take the '/' flow
    if (column < previousRow.length && previousRow[column] === '/') {
      this.takePreviousFlowAndAddNode(column, column);
      return column;
    }

    // If '\?'
    //    ' \' -> take the '\' flow and reassign to '|'
    // This should only happen if there are 3 '\' - in a row so we don't need to be cleverer here
    if (column + 1 < previousRow.length && previousRow[column + 1] === '\\') {
      currentRow[column] = '|';
      this.takePreviousFlowAndAddNode(column, column + 1);
      return column;
    }

    // If trunk mode:
    // If '\|'
    //    ' |' -> cannot take the '|' flow
    if (
      column + 1 < currentRow.length &&
      (currentRow[column + 1] === '|' || currentRow[column + 1] === '*') &&
      column + 1 < previousRow.length &&
      (previousRow[column + 1] === '|' || previousRow[column + 1] === '*')
    ) {
      this.createNewFlowAndAddNode(column);
      return column;
    }


    // If '\?'
    //    ' |' -> take the '|' flow
    if (
      column + 1 < previousRow.length &&
      (previousRow[column + 1] === '|' || previousRow[column + 1] === '*')
    ) {
      this.takePreviousFlowAndAddNode(column, column + 1);
      return column;
    }

    // Otherwise just generate a new flow and wait for bug-reports if we're wrong...
    this.createNewFlowAndAddNode(column);
    return column;
  }

  // setRightFlow handles '_' - returns the last column that was set
  // generally we prefer to take the left most flow from the previous row
  setRightFlow(column, currentRow, previousRow) {
    // if '__' keep the '_' flow
    if (column > 0 && currentRow[column - 1] === '_') {
      this.takeCurrentFlowAndAddNode(column, column - 1);
      return column;
    }

    // if '_|_' -> keep the '_' flow
    if (
      column > 1 &&
      currentRow[column - 1] === '|' &&
      currentRow[column - 2] === '_'
    ) {
      this.takeCurrentFlowAndAddNode(column, column - 2);
      return column;
    }

    // if ' _' -> take the '/' flow
    //    '/ '
    if (
      column > 0 &&
      column - 1 < previousRow.length &&
      previousRow[column - 1] === '/'
    ) {
      this.takePreviousFlowAndAddNode(column, column - 1);
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
      this.takePreviousFlowAndAddNode(column, column - 2);
      return column;
    }

    // There really shouldn't be another way of doing this - generate and wait for bug-reports...

    this.createNewFlowAndAddNode(column);
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
        flow = this.takePreviousFlowAndAddNode(column, column - 1);
        break;
      } else if (column < previousRow.length && previousRow[column] === '|') {
        flow = this.takePreviousFlowAndAddNode(column, column);
        break;
      } else if (
        column + 1 < previousRow.length &&
        previousRow[column + 1] === '\\'
      ) {
        flow = this.takePreviousFlowAndAddNode(column, column + 1);
        break;
      }
    }

    // if we have a flow then we found a '/|\' in the previousRow
    if (flow) {
      for (; originalColumn < column; originalColumn++) {
        this.takeCurrentFlowAndAddNode(originalColumn, column);
      }
      return column;
    }

    // If the symbol in the column is not a '.' then there's likely an error
    if (currentRow[column] !== '.') {
      // It really should end in a '.' but this one doesn't...
      // 1. Step back - we don't want to eat this column
      column--;
      // 2. Generate a new flow and await bug-reports...
      this.createNewFlowAndAddNode(column);

      // 3. Assign all of the '-' to the same flow.
      for (; originalColumn < column; originalColumn++) {
        this.takeCurrentFlowAndAddNode(originalColumn, column);
      }
      return column;
    }

    // We have a terminal '.' eg. the current row looks like '----.'
    // the previous row should look like one of '/|\' eg.    '     \'
    if (column > 0 && column - 1 < previousRow.length && previousRow[column - 1] === '/') {
      flow = this.takePreviousFlowAndAddNode(column, column - 1);
    } else if (column < previousRow.length && previousRow[column] === '|') {
      flow = this.takePreviousFlowAndAddNode(column, column);
    } else if (
      column + 1 < previousRow.length &&
      previousRow[column + 1] === '\\'
    ) {
      flow = this.takePreviousFlowAndAddNode(column, column + 1);
    } else {
      // Again unexpected so let's generate and wait the bug-report
      flow = this.createNewFlowAndAddNode(column);
    }

    // Assign all of the rest of the ----. to this flow.
    for (; originalColumn < column; originalColumn++) {
      this.takeCurrentFlowAndAddNode(originalColumn, column);
    }

    return column;
  }
}
