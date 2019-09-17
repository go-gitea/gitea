/* globals gitGraph */

$(document).ready(function () {
	const graphList = [];

	if (!document.getElementById('graph-canvas')) {
		return;
	}

	$("#graph-raw-list li span.node-relation").each(function () {
		graphList.push($(this).text());
	})

	gitGraph(document.getElementById('graph-canvas'), graphList);
})
