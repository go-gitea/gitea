$(document).ready(function () {
	var graphList = [];
	
	if (!document.getElementById('graph-canvas')) {
		return;
	}
	
	$("#graph-raw-list li span.node-relation").each(function () {
		graphList.push($(this).text());
	})
	
	gitGraph(document.getElementById('graph-canvas'), graphList);
	
	if ($("#rev-container")) {
		$("#rev-container").css("width", document.body.clientWidth - document.getElementById('graph-canvas').width);
	}
})
