$(async () => {
  const graphCanvas = document.getElementById('graph-canvas');
  if (!graphCanvas) return;

  const [{ default: gitGraph }] = await Promise.all([
    import(/* webpackChunkName: "gitgraph" */'./gitGraph.js'),
    import(/* webpackChunkName: "gitgraph" */'../css/gitGraph.css'),
  ]);

  const graphList = [];
  $('#graph-raw-list li span.node-relation').each(function () {
    graphList.push($(this).text());
  });

  gitGraph(graphCanvas, graphList);
});
