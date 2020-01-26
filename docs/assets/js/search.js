function ready(fn) {
    if (document.readyState != 'loading') {
        fn();
    } else {
        document.addEventListener('DOMContentLoaded', fn);
    }
}

ready(doSearch);

const summaryInclude = 60;
const fuseOptions = {
    shouldSort: true,
    includeMatches: true,
    matchAllTokens: true,
    threshold: 0.0, // for parsing diacritics
    tokenize: true,
    location: 0,
    distance: 100,
    maxPatternLength: 32,
    minMatchCharLength: 1,
    keys: [{
        name: "title",
        weight: 0.8
    },
        {
            name: "contents",
            weight: 0.5
        },
        {
            name: "tags",
            weight: 0.3
        },
        {
            name: "categories",
            weight: 0.3
        }
    ]
};

function param(name) {
    return decodeURIComponent((location.search.split(name + '=')[1] || '').split('&')[0]).replace(/\+/g, ' ');
}

let searchQuery = param("s");

function doSearch() {
    if (searchQuery) {
        document.getElementById("search-query").value = searchQuery;
        executeSearch(searchQuery);
    } else {
        const para = document.createElement("P");
        para.innerText = "Please enter a word or phrase above";
        document.getElementById("search-results").appendChild(para);
    }
}

function getJSON(url, fn) {
    const request = new XMLHttpRequest();
    request.open('GET', url, true);
    request.onload = function () {
        if (request.status >= 200 && request.status < 400) {
            const data = JSON.parse(request.responseText);
            fn(data);
        } else {
            console.log("Target reached on " + url + " with error " + request.status);
        }
    };
    request.onerror = function () {
        console.log("Connection error " + request.status);
    };
    request.send();
}

function executeSearch(searchQuery) {
    getJSON("/" + document.LANG + "/index.json", function (data) {
        const pages = data;
        const fuse = new Fuse(pages, fuseOptions);
        const result = fuse.search(searchQuery);
        console.log({
            "matches": result
        });
        document.getElementById("search-results").innerHTML = "";
        if (result.length > 0) {
            populateResults(result);
        } else {
            const para = document.createElement("P");
            para.innerText = "No matches found";
            document.getElementById("search-results").appendChild(para);
        }
    });
}

function populateResults(result) {
    result.forEach(function (value, key) {
        const content = value.item.contents;
        let snippet = "";
        const snippetHighlights = [];
        if (fuseOptions.tokenize) {
            snippetHighlights.push(searchQuery);
            value.matches.forEach(function (mvalue) {
                if (mvalue.key === "tags" || mvalue.key === "categories") {
                    snippetHighlights.push(mvalue.value);
                } else if (mvalue.key === "contents") {
                    const ind = content.toLowerCase().indexOf(searchQuery.toLowerCase());
                    const start = ind - summaryInclude > 0 ? ind - summaryInclude : 0;
                    const end = ind + searchQuery.length + summaryInclude < content.length ? ind + searchQuery.length + summaryInclude : content.length;
                    snippet += content.substring(start, end);
                    if (ind > -1) {
                        snippetHighlights.push(content.substring(ind, ind + searchQuery.length))
                    } else {
                        snippetHighlights.push(mvalue.value.substring(mvalue.indices[0][0], mvalue.indices[0][1] - mvalue.indices[0][0] + 1));
                    }
                }
            });
        }

        if (snippet.length < 1) {
            snippet += content.substring(0, summaryInclude * 2);
        }
        //pull template from hugo templarte definition
        const templateDefinition = document.getElementById("search-result-template").innerHTML;
        //replace values
        const output = render(templateDefinition, {
            key: key,
            title: value.item.title,
            link: value.item.permalink,
            tags: value.item.tags,
            categories: value.item.categories,
            snippet: snippet
        });
        document.getElementById("search-results").appendChild(htmlToElement(output));

        snippetHighlights.forEach(function (snipvalue) {
            new Mark(document.getElementById("summary-" + key)).mark(snipvalue);
        });

    });
}

function render(templateString, data) {
    let conditionalMatches, copy;
    const conditionalPattern = /\$\{\s*isset ([a-zA-Z]*) \s*\}(.*)\$\{\s*end\s*}/g;
    //since loop below depends on re.lastInxdex, we use a copy to capture any manipulations whilst inside the loop
    copy = templateString;
    while ((conditionalMatches = conditionalPattern.exec(templateString)) !== null) {
        if (data[conditionalMatches[1]]) {
            //valid key, remove conditionals, leave content.
            copy = copy.replace(conditionalMatches[0], conditionalMatches[2]);
        } else {
            //not valid, remove entire section
            copy = copy.replace(conditionalMatches[0], '');
        }
    }
    templateString = copy;
    //now any conditionals removed we can do simple substitution
    let key, find, re;
    for (key in data) {
        find = '\\$\\{\\s*' + key + '\\s*\\}';
        re = new RegExp(find, 'g');
        templateString = templateString.replace(re, data[key]);
    }
    return templateString;
}

/**
 * By Mark Amery: https://stackoverflow.com/a/35385518
 * @param {String} HTML representing a single element
 * @return {Element}
 */
function htmlToElement(html) {
    const template = document.createElement('template');
    html = html.trim(); // Never return a text node of whitespace as the result
    template.innerHTML = html;
    return template.content.firstChild;
}
