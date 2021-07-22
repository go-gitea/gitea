import renderMathInElement from 'katex/dist/contrib/auto-render.js';

const mathNodes = document.querySelectorAll('.math');

const ourRender = (nodes) => {
  for (const element of nodes) {
    if (element.hasAttribute('katex-rendered') || !element.textContent) {
      continue;
    }

    renderMathInElement(element, {
      delimiters: [
        {left: '\\[', right: '\\]', display: true},
        {left: '\\(', right: '\\)', display: false}
      ],
      errorCallback: (_, stack) => {
        element.setAttribute('title', stack);
      },
    });
    element.setAttribute('katex-rendered', 'yes');
  }
};

ourRender(mathNodes);

// Options for the observer (which mutations to observe)
const config = {childList: true, subtree: true};

// Callback function to execute when mutations are observed
const callback = (records) => {
  for (const record of records) {
    const mathNodes = record.target.querySelectorAll('.math');
    ourRender(mathNodes);
  }
};

// Create an observer instance linked to the callback function
const observer = new MutationObserver(callback);

// Start observing the target node for configured mutations
observer.observe(document, config);
