(function () {

    if (typeof self === 'undefined' || !self.Prism || !self.document || !document.createRange) {
        return;
    }

    Prism.plugins.KeepLineMarkup = true;

    Prism.hooks.add('before-highlight', function (env) {
        if (!env.element.children.length) {
            return;
        }

        env.highlightedLines = {};

        var pos = 0;
        var data = [];
        var f = function (elt, baseNode) {
            for (var i = 0, l = elt.childNodes.length; i < l; i++) {
                var child = elt.childNodes[i];
                if (child.nodeType === 1) { // element
                    if (child.classList && child.classList.contains('active')) {
                        env.highlightedLines[child.classList[0]] = true;
                    }
                    f(child);
                }
            }
        };
        f(env.element);
    });

    Prism.hooks.add('after-highlight', function (env) {
        var ol = document.createElement('ol');
        ol.className = 'linenums';
        var lines = env.element.innerHTML.split(/\n/g);
        var multilineSpan = '';
        for (var i = 0, j = lines.length; i < j; i++) {
            var line = multilineSpan + lines[i];
            if (multilineSpan !== '' && line.indexOf('</span>') === -1) {
                line += '</span>'
            } else {
                multilineSpan = '';
            }
            if (multilineSpan === '') {
                var si = line.lastIndexOf('<span');
                if (si !== -1) {
                    sj = line.indexOf('>', si);
                    se = line.indexOf('</span>', Math.max(si, sj));
                    if (se === -1) {
                        multilineSpan = line.substring(si, sj + 1);
                        line += '</span>';
                    }
                }
            }
            var li = document.createElement('li');
            var lname = 'L' + (i + 1);
            li.classList.add(lname);
            li.setAttribute('rel', lname);
            if (env.highlightedLines[lname]) {
                li.classList.add('active');
            }
            li.innerHTML = line;
            ol.appendChild(li);
        }
        env.element.innerHTML = ol.outerHTML;

        // Store new highlightedCode for later hooks calls
        env.highlightedCode = env.element.innerHTML;
    });
}());
