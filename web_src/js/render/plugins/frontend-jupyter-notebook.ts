import type {FrontendRenderFunc} from '../plugin.ts';
import {marked} from 'marked';
import {createHighlighter} from 'shiki';
import {createJavaScriptRegexEngine} from '@shikijs/engine-javascript';
import '../../../css/features/jupyter.css';

// Create highlighter instance with JavaScript engine
let highlighterPromise: Promise<any> | null = null;
async function getHighlighter() {
  if (!highlighterPromise) {
    const jsEngine = createJavaScriptRegexEngine();
    highlighterPromise = createHighlighter({
      themes: ['github-light', 'github-dark'],
      langs: ['python', 'javascript', 'typescript', 'r', 'julia', 'sql', 'bash', 'ruby'],
      engine: jsEngine,
    });
  }
  return highlighterPromise;
}

// Helper to create elements with properties
function createElement<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  props?: {className?: string; textContent?: string; innerHTML?: string},
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  if (props?.className) el.className = props.className;
  if (props?.textContent) el.textContent = props.textContent;
  if (props?.innerHTML) el.innerHTML = props.innerHTML;
  return el;
}

// Helper to create message/error divs
function createMessage(text: string, isError = false): HTMLDivElement {
  const div = createElement('div', {
    className: isError ? 'jupyter-notebook-error' : 'jupyter-notebook-message',
    textContent: text,
  });
  return div;
}

// Helper to create warning messages for unsupported output types
function createOutputWarning(text: string): HTMLDivElement {
  const div = createElement('div', {textContent: text});
  div.style.color = 'var(--color-text-light-2)';
  div.style.fontStyle = 'italic';
  return div;
}

// Helper to create image element from base64 data
function createImageFromData(data: string | string[], mimeType: string): HTMLImageElement {
  const img = document.createElement('img');
  const imgData = Array.isArray(data) ? data.join('') : data;
  img.src = `data:image/${mimeType};base64,${imgData}`;
  img.style.maxWidth = '100%';
  return img;
}

// Helper to join array or return string
function joinSource(source: string | string[]): string {
  return Array.isArray(source) ? source.join('') : (source || '');
}

// Render markdown using marked library with image URL resolution
function renderMarkdown(markdown: string, treePath: string, mediaPrefix: string): HTMLElement {
  const markupContainer = document.createElement('div');
  markupContainer.className = 'markup';

  // Parse markdown first
  const html = marked.parse(markdown) as string;
  markupContainer.innerHTML = html;

  // Post-process images to fix relative URLs
  for (const img of markupContainer.querySelectorAll('img')) {
    const src = img.getAttribute('src');
    if (src && !src.startsWith('http://') && !src.startsWith('https://') && !src.startsWith('data:') && !src.startsWith('/')) {
      // Construct image path relative to notebook location
      const notebookDir = treePath.substring(0, treePath.lastIndexOf('/'));
      const imagePath = notebookDir ? `${notebookDir}/${src}` : src;

      // Use media prefix from backend to construct full URL
      img.src = `${mediaPrefix}/${imagePath}`;
    }
  }

  return markupContainer;
}

// Highlight code using Shiki with JavaScript regex engine (no WASM)
async function highlightCode(code: string, language: string): Promise<HTMLElement> {
  try {
    // Detect if dark mode is active using Gitea's theme attribute
    const isDark = document.documentElement.getAttribute('data-gitea-theme-dark') === 'true';

    const highlighter = await getHighlighter();
    const html = highlighter.codeToHtml(code, {
      lang: language,
      theme: isDark ? 'github-dark' : 'github-light',
    });

    // Parse HTML and remove background-color from pre element
    const container = document.createElement('div');
    container.innerHTML = html;
    const pre = container.querySelector('pre');
    if (pre) {
      pre.style.backgroundColor = '';
    }
    return pre || container;
  } catch (error) {
    console.warn('Shiki highlighting failed:', error);
    // Fallback to plain code
    const pre = document.createElement('pre');
    const codeEl = document.createElement('code');
    codeEl.className = `chroma language-${language}`;
    codeEl.textContent = code;
    pre.append(codeEl);
    return pre;
  }
}

export const frontendRender: FrontendRenderFunc = async (opts) => {
  try {
    const notebook = JSON.parse(opts.contentString());

    // Only support nbformat 4+
    if (notebook.nbformat && notebook.nbformat < 4) {
      opts.container.append(createMessage(
        `This notebook uses an older format (nbformat ${notebook.nbformat}). Only nbformat 4+ is supported for rendering. Please upgrade the notebook in Jupyter or view the raw JSON below.`,
      ));
      return false;
    }

    if (!notebook.cells || !Array.isArray(notebook.cells)) {
      throw new Error('Invalid notebook format: missing or invalid cells array');
    }

    // Get media prefix from container data attribute
    const viewerContainer = document.querySelector<HTMLElement>('#frontend-render-viewer');
    const mediaPrefix = viewerContainer?.getAttribute('data-media-prefix') || '';

    // Detect language from notebook metadata
    const language = notebook.metadata?.language_info?.name ||
                     notebook.metadata?.kernelspec?.language ||
                     'text';

    const container = createElement('div', {className: 'jupyter-notebook'});

    let executionCount = 1;

    for (const cell of notebook.cells) {
      if (!cell.cell_type) continue;

      const cellDiv = createElement('div', {className: `cell ${cell.cell_type}`});

      if (cell.cell_type === 'markdown') {
        const inputDiv = createElement('div', {className: 'input markup'});
        const source = joinSource(cell.source);
        inputDiv.append(renderMarkdown(source, opts.treePath, mediaPrefix));
        cellDiv.append(inputDiv);
      } else if (cell.cell_type === 'code') {
        const inputWrapper = createElement('div', {className: 'input-wrapper'});

        const prompt = createElement('div', {
          className: 'prompt input-prompt',
          textContent: `In [${cell.execution_count ?? executionCount}]:`,
        });
        inputWrapper.append(prompt);

        const inputDiv = createElement('div', {className: 'input'});

        const source = joinSource(cell.source);

        // Highlight code with Shiki
        const highlightedElement = await highlightCode(source, language);
        inputDiv.append(highlightedElement);

        inputWrapper.append(inputDiv);
        cellDiv.append(inputWrapper);

        if (cell.outputs && Array.isArray(cell.outputs) && cell.outputs.length > 0) {
          const outputWrapper = createElement('div', {className: 'output-wrapper'});

          const hasExecutionResult = cell.outputs.some((o: any) => o.output_type === 'execute_result');

          const outPrompt = createElement('div', {className: 'prompt output-prompt'});
          if (hasExecutionResult) {
            outPrompt.textContent = `Out[${cell.execution_count ?? executionCount}]:`;
          }
          outputWrapper.append(outPrompt);

          const outputDiv = createElement('div', {className: 'output'});

          for (const output of cell.outputs) {
            try {
              if (output.data) {
                if (output.data['image/png']) {
                  outputDiv.append(createImageFromData(output.data['image/png'], 'png'));
                } else if (output.data['image/jpeg']) {
                  outputDiv.append(createImageFromData(output.data['image/jpeg'], 'jpeg'));
                } else if (output.data['image/svg+xml']) {
                  const svgDiv = document.createElement('div');
                  svgDiv.innerHTML = joinSource(output.data['image/svg+xml']);
                  outputDiv.append(svgDiv);
                } else if (output.data['text/html']) {
                  const wrapperDiv = document.createElement('div');
                  wrapperDiv.style.overflowX = 'auto';
                  wrapperDiv.style.maxWidth = '100%';
                  const htmlDiv = document.createElement('div');
                  htmlDiv.innerHTML = joinSource(output.data['text/html']);
                  // Ensure images inside HTML outputs are constrained
                  for (const img of htmlDiv.querySelectorAll('img')) {
                    img.style.maxWidth = '100%';
                    img.style.height = 'auto';
                  }
                  wrapperDiv.append(htmlDiv);
                  outputDiv.append(wrapperDiv);
                } else if (output.data['application/javascript']) {
                  outputDiv.append(createOutputWarning('[JavaScript output - execution disabled for security]'));
                } else if (output.data['application/vnd.plotly.v1+json']) {
                  outputDiv.append(createOutputWarning('[Plotly output - interactive plots not supported]'));
                } else if (output.data['application/vnd.jupyter.widget-view+json']) {
                  outputDiv.append(createOutputWarning('[Jupyter widget - interactive widgets not supported]'));
                } else if (output.data['text/latex']) {
                  const latex = joinSource(output.data['text/latex']);
                  const pre = document.createElement('pre');
                  const mathCode = createElement('code', {
                    className: 'language-math display',
                    textContent: latex.replace(/^\$\$|\$\$$/g, ''),
                  });
                  pre.append(mathCode);
                  outputDiv.append(pre);
                } else if (output.data['text/plain']) {
                  const textPre = createElement('pre', {textContent: joinSource(output.data['text/plain'])});
                  outputDiv.append(textPre);
                }
              } else if (output.output_type === 'stream' && output.name) {
                const streamPre = createElement('pre', {
                  className: `stream-${output.name}`,
                  textContent: joinSource(output.text),
                });
                outputDiv.append(streamPre);
              } else if (output.output_type === 'error') {
                const traceback = Array.isArray(output.traceback) ? output.traceback.join('\n') :
                  (output.ename && output.evalue ? `${output.ename}: ${output.evalue}` : 'Error');
                const errorPre = createElement('pre', {
                  className: 'error-output',
                  textContent: traceback,
                });
                errorPre.style.color = 'var(--color-red)';
                outputDiv.append(errorPre);
              } else if (output.text) {
                const textPre = createElement('pre', {textContent: joinSource(output.text)});
                outputDiv.append(textPre);
              }
            } catch (outputError) {
              console.warn('Failed to render output:', outputError);
              outputDiv.append(createOutputWarning('[Output rendering failed]'));
            }
          }

          if (outputDiv.children.length > 0) {
            outputWrapper.append(outputDiv);
            cellDiv.append(outputWrapper);
          }
        }

        executionCount++;
      }

      container.append(cellDiv);
    }

    opts.container.append(container);

    const {initMarkupCodeMath} = await import('../../markup/math.ts');
    await initMarkupCodeMath(container);

    return true;
  } catch (error) {
    console.error('Jupyter notebook rendering failed:', error);
    const errorMessage = error instanceof Error ? error.message : String(error);
    opts.container.append(createMessage(`Failed to render notebook: ${errorMessage}`, true));
    return false;
  }
};
