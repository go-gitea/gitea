function displayError(el, err) {
  let target = el;
  if (el.classList.contains('is-loading')) {
    // assume no pre
    el.classList.remove('is-loading');
  } else {
    target = el.closest('pre');
    target.classList.remove('is-loading');
  }
  const errorNode = document.createElement('div');
  errorNode.setAttribute('class', 'ui message error markup-block-error mono');
  errorNode.textContent = err.str || err.message || String(err);
  target.before(errorNode);
}

export async function renderMath() {
  const els = document.querySelectorAll('.markup code.language-math');
  if (!els.length) return;

  const [{default: katex}] = await Promise.all([
    import(/* webpackChunkName: "katex" */'katex'),
    import(/* webpackChunkName: "katex" */'katex/dist/katex.css'),
  ]);

  for (const el of els) {
    const source = el.textContent;

    const options = {display: el.classList.contains('display')};

    try {
      const markup = katex.renderToString(source, options);
      const target = document.createElement(options.display ? 'p' : 'span')
      target.innerHTML = markup;
      if (el.classList.contains('is-loading')) {
        el.replaceWith(target);
      } else {
        el.closest('pre').replaceWith(target);
      }
    } catch (error) {
      displayError(el, error);
    }
  }
}
