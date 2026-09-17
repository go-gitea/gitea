import {isDarkTheme} from '../utils.ts';
import {displayError} from './common.ts';
import {createElementFromAttrs, createElementFromHTML, isPlainClick, queryElems} from '../utils/dom.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {svg} from '../svg.ts';

const {mermaidMaxSourceCharacters} = window.config;

function initMermaidViewController(viewController: Element, dragElement: SVGSVGElement) {
  let scale = 1, left = 0, top = 0, lastPageX = 0, lastPageY = 0;
  const applyTransform = () => {
    dragElement.style.transform = `translate(${left}px, ${top}px) scale(${scale})`;
  };

  for (const el of viewController.querySelectorAll('[data-control-action]')) {
    el.addEventListener('click', () => {
      switch (el.getAttribute('data-control-action')) {
        case 'zoom-in':
          scale *= 1.2;
          break;
        case 'zoom-out':
          scale /= 1.2;
          break;
        case 'reset':
          scale = 1;
          left = top = 0;
          break;
      }
      applyTransform();
    });
  }

  dragElement.addEventListener('pointerdown', (e) => {
    if (!isPlainClick(e)) return;
    // don't start the drag if the click is on an interactive element (e.g.: link, button) or text element
    if ((e.target as Element).closest('div, p, a, span, button, input, text')) return;
    dragElement.setPointerCapture(e.pointerId);
    lastPageX = e.pageX;
    lastPageY = e.pageY;
    dragElement.style.cursor = 'grabbing';
  });

  dragElement.addEventListener('pointermove', (e) => {
    if (!dragElement.hasPointerCapture(e.pointerId)) return;
    left += e.pageX - lastPageX;
    top += e.pageY - lastPageY;
    lastPageX = e.pageX;
    lastPageY = e.pageY;
    applyTransform();
  });

  dragElement.addEventListener('lostpointercapture', () => dragElement.style.removeProperty('cursor'));
}

export async function initMarkupCodeMermaid(elMarkup: HTMLElement): Promise<void> {
  // .markup code.language-mermaid
  const mermaidBlocks: Array<{source: string, parentContainer: Element}> = [];
  const attrMermaidRendered = 'data-markup-mermaid-rendered';
  for (const elCodeBlock of queryElems(elMarkup, 'code.language-mermaid')) {
    const parentContainer = elCodeBlock.closest('pre.code-block')!; // it must exist, if no, there must be a bug
    if (parentContainer.hasAttribute(attrMermaidRendered)) continue;
    parentContainer.setAttribute(attrMermaidRendered, 'true');
    const source = elCodeBlock.textContent;
    if (mermaidMaxSourceCharacters >= 0 && source.length > mermaidMaxSourceCharacters) {
      displayError(parentContainer, new Error(`Mermaid source of ${source.length} characters exceeds the maximum allowed length of ${mermaidMaxSourceCharacters}.`));
      continue;
    }
    mermaidBlocks.push({source, parentContainer});
  }
  if (!mermaidBlocks.length) return;

  const {default: mermaid} = await import('mermaid');
  mermaid.initialize({
    startOnLoad: false,
    theme: isDarkTheme() ? 'dark' : 'neutral', // TODO: maybe it should use "darkMode" to adopt more user-specified theme instead of just "dark" or "neutral"
    look: 'neo',
    themeVariables: {dropShadow: 'none', useGradient: false},
    themeCSS: '[filter] { filter: none; }', // sequence diagrams hardcode the neo drop shadow as an attribute
    securityLevel: 'strict',
    suppressErrorRendering: true,
  });

  // mermaid is a globally shared instance, its document also says "Multiple calls to this function will be enqueued to run serially."
  // so here we just simply render the mermaid blocks one by one, no need to do "Promise.all" concurrently
  for (const {source, parentContainer} of mermaidBlocks) {
    try {
      // render the mermaid diagram to svg text, and parse it to a DOM node
      const {svg: svgText, bindFunctions} = await mermaid.render('mermaid', source, parentContainer);
      const svgNode = createElementFromHTML<SVGSVGElement>(svgText);

      const viewControllerHtml = html`
        <div class="view-controller auto-hide-control flex-text-block">
          <button type="button" class="ui tiny compact icon button" data-control-action="zoom-in">${htmlRaw(svg('octicon-zoom-in', 12))}</button>
          <button type="button" class="ui tiny compact icon button" data-control-action="reset">${htmlRaw(svg('octicon-sync', 12))}</button>
          <button type="button" class="ui tiny compact icon button" data-control-action="zoom-out">${htmlRaw(svg('octicon-zoom-out', 12))}</button>
        </div>
      `;
      const viewController = createElementFromHTML(viewControllerHtml);

      // create an iframe to sandbox the svg with styles, and set correct height by reading svg's viewBox height
      const iframe = document.createElement('iframe');
      iframe.classList.add('markup-content-iframe', 'tw-invisible');
      iframe.srcdoc = '<style>body { margin: 0; overflow: hidden; } #mermaid { display: block; margin: 0 auto; }</style>';
      const viewBoxHeight = svgNode.viewBox.baseVal?.height;
      if (viewBoxHeight) iframe.style.height = `${Math.max(Math.ceil(viewBoxHeight), 85)}px`; // min-height keeps the buttons from overlapping

      // the iframe will be fully reloaded if its DOM context is changed (e.g.: moved in the DOM tree).
      // to avoid unnecessary reloading, we should insert the iframe to its final position only once.
      iframe.addEventListener('load', () => {
        const iframeBody = iframe.contentDocument!.body;
        iframeBody.append(svgNode);
        bindFunctions?.(iframeBody); // follow "mermaid.render" doc, attach event handlers to the svg's container
        iframe.classList.remove('tw-invisible');
        initMermaidViewController(viewController, svgNode);
      });

      const container = createElementFromAttrs('div', {class: 'mermaid-block'}, iframe, viewController);
      parentContainer.replaceWith(container);
    } catch (err) {
      displayError(parentContainer, err);
    }
  }
}
