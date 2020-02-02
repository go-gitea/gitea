$(async () => {
  const graphCanvas = document.getElementById('graph-canvas');
  if (!graphCanvas) return;

  const { default: gitGraph } = await import(/* webpackChunkName: "gitgraph" */'./gitGraph.js');

  const graphList = [];
  $('#graph-raw-list li span.node-relation').each(function () {
    graphList.push($(this).text());
  });

  gitGraph(graphCanvas, graphList);
});
