// A very simple web worker for highlight.js
// See: https://highlightjs.org/usage/
'use strict';

importScripts('/vendor/plugins/highlight/highlight.pack.js');

onmessage = function(event) {
    var result = self.hljs.highlightAuto(event.data.text);
    postMessage({index: event.data.index, html: result.value});
};
