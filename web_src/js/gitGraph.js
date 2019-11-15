$(async () => {
  const graphCanvas = document.getElementById('graph-canvas');
  if (!graphCanvas) return;

  const [{ default: gitGraph }] = await Promise.all([
    import(/* webpackChunkName: "gitgraph" */'../vendor/gitgraph.js/gitgraph.custom.js'),
    import(/* webpackChunkName: "gitgraph" */'../vendor/gitgraph.js/gitgraph.custom.css'),
  ]);

  const graphList = [];
  $('#graph-raw-list li span.node-relation').each(function () {
    graphList.push($(this).text());
  });

  gitGraph(graphCanvas, graphList);
});
