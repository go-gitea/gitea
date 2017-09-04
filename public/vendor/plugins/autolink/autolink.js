(function () {
    var re = /((([A-Za-z]{3,9}:(?:\/\/)?)(?:[\-;:&=\+\$,\w]+@)?[A-Za-z0-9\.\-]+|(?:www\.|[\-;:&=\+\$,\w]+@)[A-Za-z0-9\.\-]+)((?:\/[\+~%\/\.\w\-]*)?\??(?:[\-\+:=&;%@\.\w]*)#?(?:[\.\!\/\\\w]*))?)/g;
    function textNodesUnder(node) {
        var textNodes = [];
        if(typeof document.createTreeWalker === 'function') {
            // Efficient TreeWalker
            var currentNode, walker;
            walker = document.createTreeWalker(node, NodeFilter.SHOW_TEXT, null, false);
            while(currentNode = walker.nextNode()) {
                textNodes.push(currentNode);
            }
        } else {
            // Less efficient recursive function
            for(node = node.firstChild; node; node = node.nextSibling) {
                if(node.nodeType === 3) {
                    textNodes.push(node);
                } else {
                    textNodes = textNodes.concat(textNodesUnder(node));
                }
            }
        }
        return textNodes;
    }

    function processNode(node) {
        re.lastIndex = 0;
        var results = re.exec(node.textContent);
        if(results !== null) {
            if($(node).parents().filter('code').length === 0) {
                $(node).replaceWith(
                    $('<span />').html(
                        node.nodeValue.replace(re, '<a href="$1">$1</a>')
                    )
                );
            }
        }
    }

    jQuery.fn.autolink = function () {
        this.each(function () {
            textNodesUnder(this).forEach(processNode);
        });
        return this;
    };
})();
