// A very simple web worker for highlight.js
// See: https://highlightjs.org/usage/
onmessage = function(event) {
    importScripts('/vendor/plugins/highlight/highlight.pack.js');
    var result = self.hljs.highlightAuto(event.data);
    postMessage(result.value);
}
