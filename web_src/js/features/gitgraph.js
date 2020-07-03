export default async function initGitGraph() {
  const graphCanvas = document.getElementById('graph-canvas');
  if (!graphCanvas) return;

  const graphList = [];
  $('#graph-raw-list li span.node-relation').each(function () {
    graphList.push($(this).text());
  });

  gitGraph(graphCanvas, graphList);
  graphCanvas.closest('#git-graph-container').classList.add('in');
}

// This is a continuation of https://github.com/bluef/gitgraph.js/blob/master/gitgraph.js
//
// Copyright (c) 2011, Terrence Lee <kill889@gmail.com>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//     * Redistributions of source code must retain the above copyright
//       notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above copyright
//       notice, this list of conditions and the following disclaimer in the
//       documentation and/or other materials provided with the distribution.
//     * Neither the name of the <organization> nor the
//       names of its contributors may be used to endorse or promote products
//       derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> BE LIABLE FOR ANY
// DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
// ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
function gitGraph(canvas, rawGraphList, config) {
  if (!canvas.getContext) {
    return;
  }

  if (typeof config === 'undefined') {
    config = {
      unitSize: 20,
      lineWidth: 3,
      nodeRadius: 4,
    };
  }

  const flows = [];
  const graphList = [];

  const ctx = canvas.getContext('2d');
  const ratio = window.devicePixelRatio || 1;

  const init = function () {
    let maxWidth = 0;
    let i;
    const l = rawGraphList.length;
    let row;
    let midStr;

    for (i = 0; i < l; i++) {
      midStr = rawGraphList[i].replace(/\s+/g, ' ').replace(/^\s+|\s+$/g, '');
      midStr = midStr.replace(/(--)|(-\.)/g, '-');
      maxWidth = Math.max(midStr.replace(/(_|\s)/g, '').length, maxWidth);

      row = midStr.split('');

      graphList.unshift(row);
    }

    const width = maxWidth * config.unitSize;
    const height = graphList.length * config.unitSize;

    canvas.width = width * ratio;
    canvas.height = height * ratio;

    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;

    ctx.lineWidth = config.lineWidth;
    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';

    ctx.scale(ratio, ratio);
  };

  const genRandomStr = function () {
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
  };

  const findFlow = function (id) {
    let i = flows.length;

    while (i-- && flows[i].id !== id);

    return i;
  };

  const findColumn = function (symbol, row) {
    let i = row.length;

    while (i-- && row[i] !== symbol);

    return i;
  };

  const findBranchOut = function (row) {
    if (!row) {
      return -1;
    }

    let i = row.length;

    while (
      i-- &&
      !(row[i - 1] && row[i] === '/' && row[i - 1] === '|') &&
      !(row[i - 2] && row[i] === '_' && row[i - 2] === '|')
    );

    return i;
  };

  const findLineBreak = function (row) {
    if (!row) {
      return -1;
    }

    let i = row.length;

    while (
      i-- &&
      !(
        row[i - 1] &&
        row[i - 2] &&
        row[i] === ' ' &&
        row[i - 1] === '|' &&
        row[i - 2] === '_'
      )
    );

    return i;
  };

  const genNewFlow = function () {
    let newId;

    do {
      newId = genRandomStr();
    } while (findFlow(newId) !== -1);

    return {id: newId, color: `#${newId}`};
  };

  // Draw methods
  const drawLine = function (moveX, moveY, lineX, lineY, color) {
    ctx.strokeStyle = color;
    ctx.beginPath();
    ctx.moveTo(moveX, moveY);
    ctx.lineTo(lineX, lineY);
    ctx.stroke();
  };

  const drawLineRight = function (x, y, color) {
    drawLine(
      x,
      y + config.unitSize / 2,
      x + config.unitSize,
      y + config.unitSize / 2,
      color
    );
  };

  const drawLineUp = function (x, y, color) {
    drawLine(x, y + config.unitSize / 2, x, y - config.unitSize / 2, color);
  };

  const drawNode = function (x, y, color) {
    ctx.strokeStyle = color;
    drawLineUp(x, y, color);
    ctx.beginPath();
    ctx.arc(x, y, config.nodeRadius, 0, Math.PI * 2, true);
    ctx.fillStyle = color;
    ctx.fill();
  };

  const drawLineIn = function (x, y, color) {
    drawLine(
      x + config.unitSize,
      y + config.unitSize / 2,
      x,
      y - config.unitSize / 2,
      color
    );
  };

  const drawLineOut = function (x, y, color) {
    drawLine(
      x,
      y + config.unitSize / 2,
      x + config.unitSize,
      y - config.unitSize / 2,
      color
    );
  };

  const draw = function (graphList) {
    let column;
    let columnIndex;
    let prevColumn;
    let condenseIndex;
    let breakIndex = -1;
    let x, y;
    let color;
    let nodePos;
    let tempFlow;
    let prevRowLength = 0;
    let flowSwapPos = -1;
    let lastLinePos;
    let i, l;
    let condenseCurrentLength;
    let condensePrevLength = 0;
    let inlineIntersect = false;

    // initiate color array for first row
    for (i = 0, l = graphList[0].length; i < l; i++) {
      if (graphList[0][i] !== '_' && graphList[0][i] !== ' ') {
        flows.push(genNewFlow());
      }
    }

    y = canvas.height / ratio - config.unitSize * 0.5;

    // iterate
    for (i = 0, l = graphList.length; i < l; i++) {
      x = config.unitSize * 0.5;

      const currentRow = graphList[i];
      const nextRow = graphList[i + 1];
      const prevRow = graphList[i - 1];

      flowSwapPos = -1;

      condenseCurrentLength = currentRow.filter((val) => {
        return val !== ' ' && val !== '_';
      }).length;

      // pre process begin
      // use last row for analysing
      if (prevRow) {
        if (!inlineIntersect) {
          // intersect might happen
          for (columnIndex = 0; columnIndex < prevRowLength; columnIndex++) {
            if (
              (prevRow[columnIndex + 1] &&
                prevRow[columnIndex] === '/' &&
                prevRow[columnIndex + 1] === '|') ||
              (prevRow[columnIndex] === '_' &&
                prevRow[columnIndex + 1] === '|' &&
                prevRow[columnIndex + 2] === '/')
            ) {
              flowSwapPos = columnIndex;

              // swap two flow
              tempFlow = {
                id: flows[flowSwapPos].id,
                color: flows[flowSwapPos].color,
              };

              flows[flowSwapPos].id = flows[flowSwapPos + 1].id;
              flows[flowSwapPos].color = flows[flowSwapPos + 1].color;

              flows[flowSwapPos + 1].id = tempFlow.id;
              flows[flowSwapPos + 1].color = tempFlow.color;
            }
          }
        }

        if (
          condensePrevLength < condenseCurrentLength &&
          (nodePos = findColumn('*', currentRow)) !== -1 &&
          findColumn('_', currentRow) === -1
        ) {
          flows.splice(nodePos - 1, 0, genNewFlow());
        }

        if (
          prevRowLength > currentRow.length &&
          (nodePos = findColumn('*', prevRow)) !== -1
        ) {
          if (
            findColumn('_', currentRow) === -1 &&
            findColumn('/', currentRow) === -1 &&
            findColumn('\\', currentRow) === -1
          ) {
            flows.splice(nodePos + 1, 1);
          }
        }
      } // done with the previous row

      prevRowLength = currentRow.length; // store for next round
      columnIndex = 0; // reset index
      condenseIndex = 0;
      condensePrevLength = 0;
      breakIndex = -1; // reset break index
      while (columnIndex < currentRow.length) {
        column = currentRow[columnIndex];

        if (column !== ' ' && column !== '_') {
          ++condensePrevLength;
        }

        // check and fix line break in next row
        if (
          column === '/' &&
          currentRow[columnIndex - 1] &&
          currentRow[columnIndex - 1] === '|'
        ) {
          if ((breakIndex = findLineBreak(nextRow)) !== -1) {
            nextRow.splice(breakIndex, 1);
          }
        }
        // if line break found replace all '/' with '|' after breakIndex in previous row
        if (breakIndex !== -1 && column === '/' && columnIndex > breakIndex) {
          currentRow[columnIndex] = '|';
          column = '|';
        }

        if (
          column === ' ' &&
          currentRow[columnIndex + 1] &&
          currentRow[columnIndex + 1] === '_' &&
          currentRow[columnIndex - 1] &&
          currentRow[columnIndex - 1] === '|'
        ) {
          currentRow.splice(columnIndex, 1);
          currentRow[columnIndex] = '/';
          column = '/';
        }

        // create new flow only when no intersect happened
        if (
          flowSwapPos === -1 &&
          column === '/' &&
          currentRow[columnIndex - 1] &&
          currentRow[columnIndex - 1] === '|'
        ) {
          flows.splice(condenseIndex, 0, genNewFlow());
        }

        // change \ and / to | when it's in the last position of the whole row
        if (column === '/' || column === '\\') {
          if (!(column === '/' && findBranchOut(nextRow) === -1)) {
            if (
              (lastLinePos = Math.max(
                findColumn('|', currentRow),
                findColumn('*', currentRow)
              )) !== -1 &&
              lastLinePos < columnIndex - 1
            ) {
              while (currentRow[++lastLinePos] === ' ');

              if (lastLinePos === columnIndex) {
                currentRow[columnIndex] = '|';
              }
            }
          }
        }

        if (column === '*' && prevRow && prevRow[condenseIndex + 1] === '\\') {
          flows.splice(condenseIndex + 1, 1);
        }

        if (column !== ' ') {
          ++condenseIndex;
        }

        ++columnIndex;
      }

      condenseCurrentLength = currentRow.filter((val) => {
        return val !== ' ' && val !== '_';
      }).length;

      columnIndex = 0;

      // a little inline analysis and draw process
      while (columnIndex < currentRow.length) {
        column = currentRow[columnIndex];
        prevColumn = currentRow[columnIndex - 1];

        if (currentRow[columnIndex] === ' ') {
          currentRow.splice(columnIndex, 1);
          x += config.unitSize;

          continue;
        }

        // inline intersect
        if (
          (column === '_' || column === '/') &&
          currentRow[columnIndex - 1] === '|' &&
          currentRow[columnIndex - 2] === '_'
        ) {
          inlineIntersect = true;

          tempFlow = flows.splice(columnIndex - 2, 1)[0];
          flows.splice(columnIndex - 1, 0, tempFlow);
          currentRow.splice(columnIndex - 2, 1);

          columnIndex -= 1;
        } else {
          inlineIntersect = false;
        }

        if (
          column === '|' &&
          currentRow[columnIndex - 1] &&
          currentRow[columnIndex - 1] === '\\'
        ) {
          flows.splice(columnIndex, 0, genNewFlow());
        }

        color = flows[columnIndex].color;

        switch (column) {
          case '-':
          case '_':
            drawLineRight(x, y, color);

            x += config.unitSize;
            break;

          case '*':
            drawNode(x, y, color);
            break;

          case '|':
            if (prevColumn && prevColumn === '\\') {
              x += config.unitSize;
            }
            drawLineUp(x, y, color);
            break;

          case '/':
            if (prevColumn && (prevColumn === '/' || prevColumn === ' ')) {
              x -= config.unitSize;
            }

            drawLineOut(x, y, color);

            x += config.unitSize;
            break;

          case '\\':
            drawLineIn(x, y, color);
            break;
        }

        ++columnIndex;
      }

      y -= config.unitSize;
    }

    // do some clean up
    if (flows.length > condenseCurrentLength) {
      flows.splice(condenseCurrentLength, flows.length - condenseCurrentLength);
    }
  };

  init();
  draw(graphList);
}
