// Although inspired by the https://github.com/bluef/gitgraph.js/blob/master/gitgraph.js
// this has been completely rewritten with almost no remaining code
import {RandomHuePalette} from './gitgraph/palette.js';

import {FlowParser} from './gitgraph/flowparser.js';
import {GitGraphCanvas} from './gitgraph/gitgraphcanvas.js';

const palette = new RandomHuePalette(
  [
    'hsl(204,70%,50%)', // dark blue
    'hsl(116,57%,60%)', // dark green
    'hsl(359,79%,50%)', // dark red
    'hsl(30,100%,50%)', // dark orange
    'hsl(269,60%,50%)', // dark purple
    'hsl(21,63%,50%)', // dark yellowish
    'hsl(201,60%,70%)', // light blue
    'hsl(92,64%,70%)', // light green
    'hsl(1,92%,70%)', // light red
    'hsl(34,97%,70%)', // light orange
    'hsl(280,60%,70%', // light purple
    'hsl(60,70%,60%)', // light yellowish
  ]
);

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


  const flowParser = new FlowParser(graphList);

  const flows = flowParser.generateFlows();
  if (flows) {
    const gitGraphCanvas = new GitGraphCanvas(
      graphCanvas,
      flows.maxWidth,
      flows.maxRow,
      config
    );
    flows.assignColors(palette);
    flows.draw(gitGraphCanvas);
  }
  graphCanvas.closest('#git-graph-container').classList.add('in');
}
