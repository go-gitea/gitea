import type {FrontendRenderFunc} from '../plugin.ts';
import {marked} from 'marked';
import '../../../css/features/jupyter.css';

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

// Render markdown using marked library
function renderMarkdown(markdown: string): HTMLElement {
  const container = document.createElement('div');
  container.className = 'markup';
  container.innerHTML = marked.parse(markdown) as string;
  return container;
}

export const frontendRender: FrontendRenderFunc = async (opts) => {
  try {
    const notebook = JSON.parse(opts.contentString());

    if (!notebook.cells || !Array.isArray(notebook.cells)) {
      throw new Error('Invalid notebook format: missing or invalid cells array');
    }

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
        const source = Array.isArray(cell.source) ? cell.source.join('') : (cell.source || '');
        inputDiv.append(renderMarkdown(source));
        cellDiv.append(inputDiv);
      } else if (cell.cell_type === 'code') {
        const inputWrapper = createElement('div', {className: 'input-wrapper'});

        const prompt = createElement('div', {
          className: 'prompt input-prompt',
          textContent: `In [${cell.execution_count ?? executionCount}]:`,
        });
        inputWrapper.append(prompt);

        const inputDiv = createElement('div', {className: 'input'});

        const pre = document.createElement('pre');
        const code = createElement('code', {className: `language-${language}`});
        const source = Array.isArray(cell.source) ? cell.source.join('') : (cell.source || '');
        code.textContent = source;
        pre.append(code);
        inputDiv.append(pre);
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
                  const img = document.createElement('img');
                  const imgData = Array.isArray(output.data['image/png']) ?
                    output.data['image/png'].join('') : output.data['image/png'];
                  img.src = `data:image/png;base64,${imgData}`;
                  img.style.maxWidth = '100%';
                  outputDiv.append(img);
                } else if (output.data['image/jpeg']) {
                  const img = document.createElement('img');
                  const imgData = Array.isArray(output.data['image/jpeg']) ?
                    output.data['image/jpeg'].join('') : output.data['image/jpeg'];
                  img.src = `data:image/jpeg;base64,${imgData}`;
                  img.style.maxWidth = '100%';
                  outputDiv.append(img);
                } else if (output.data['image/svg+xml']) {
                  const svgDiv = document.createElement('div');
                  const svgData = Array.isArray(output.data['image/svg+xml']) ?
                    output.data['image/svg+xml'].join('') : output.data['image/svg+xml'];
                  svgDiv.innerHTML = svgData;
                  outputDiv.append(svgDiv);
                } else if (output.data['text/html']) {
                  const wrapperDiv = document.createElement('div');
                  wrapperDiv.style.overflowX = 'auto';
                  wrapperDiv.style.maxWidth = '100%';
                  const htmlDiv = document.createElement('div');
                  const htmlData = Array.isArray(output.data['text/html']) ?
                    output.data['text/html'].join('') : output.data['text/html'];
                  htmlDiv.innerHTML = htmlData;
                  // Ensure images inside HTML outputs are constrained
                  for (const img of htmlDiv.querySelectorAll('img')) {
                    img.style.maxWidth = '100%';
                    img.style.height = 'auto';
                  }
                  wrapperDiv.append(htmlDiv);
                  outputDiv.append(wrapperDiv);
                } else if (output.data['application/javascript']) {
                  const jsDiv = createElement('div', {
                    className: 'js-output-warning',
                    textContent: '[JavaScript output - execution disabled for security]',
                  });
                  jsDiv.style.color = 'var(--color-text-light-2)';
                  jsDiv.style.fontStyle = 'italic';
                  outputDiv.append(jsDiv);
                } else if (output.data['application/vnd.plotly.v1+json']) {
                  const plotlyDiv = createElement('div', {
                    className: 'plotly-output-warning',
                    textContent: '[Plotly output - interactive plots not supported]',
                  });
                  plotlyDiv.style.color = 'var(--color-text-light-2)';
                  plotlyDiv.style.fontStyle = 'italic';
                  outputDiv.append(plotlyDiv);
                } else if (output.data['application/vnd.jupyter.widget-view+json']) {
                  const widgetDiv = createElement('div', {
                    className: 'widget-output-warning',
                    textContent: '[Jupyter widget - interactive widgets not supported]',
                  });
                  widgetDiv.style.color = 'var(--color-text-light-2)';
                  widgetDiv.style.fontStyle = 'italic';
                  outputDiv.append(widgetDiv);
                } else if (output.data['text/latex']) {
                  const latex = Array.isArray(output.data['text/latex']) ?
                    output.data['text/latex'].join('') : output.data['text/latex'];
                  const pre = document.createElement('pre');
                  const mathCode = createElement('code', {
                    className: 'language-math display',
                    textContent: latex.replace(/^\$\$|\$\$$/g, ''),
                  });
                  pre.append(mathCode);
                  outputDiv.append(pre);
                } else if (output.data['text/plain']) {
                  const plainText = Array.isArray(output.data['text/plain']) ?
                    output.data['text/plain'].join('') : output.data['text/plain'];
                  const textPre = createElement('pre', {textContent: plainText});
                  outputDiv.append(textPre);
                }
              } else if (output.output_type === 'stream' && output.name) {
                const streamText = Array.isArray(output.text) ? output.text.join('') : (output.text || '');
                const streamPre = createElement('pre', {
                  className: `stream-${output.name}`,
                  textContent: streamText,
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
                const text = Array.isArray(output.text) ? output.text.join('') : output.text;
                const textPre = createElement('pre', {textContent: text});
                outputDiv.append(textPre);
              }
            } catch (outputError) {
              console.warn('Failed to render output:', outputError);
              const errorDiv = createElement('div', {
                textContent: '[Output rendering failed]',
              });
              errorDiv.style.color = 'var(--color-text-light-2)';
              errorDiv.style.fontStyle = 'italic';
              outputDiv.append(errorDiv);
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
    const errorDiv = document.createElement('div');
    errorDiv.style.padding = '20px';
    errorDiv.style.color = 'var(--color-red)';
    const errorTitle = document.createElement('strong');
    errorTitle.textContent = 'Failed to render notebook:';
    errorDiv.append(errorTitle);
    errorDiv.append(document.createElement('br'));
    const errorMessage = error instanceof Error ? error.message : String(error);
    errorDiv.append(document.createTextNode(errorMessage));
    opts.container.append(errorDiv);
    return false;
  }
};
