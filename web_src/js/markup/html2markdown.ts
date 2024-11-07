import {htmlEscape} from 'escape-goat';

type Processors = {
  [tagName: string]: (el: HTMLElement) => string | HTMLElement | void;
}

type ProcessorContext = {
  elementIsFirst: boolean;
  elementIsLast: boolean;
  listNestingLevel: number;
}

function prepareProcessors(ctx:ProcessorContext): Processors {
  const processors = {
    H1(el) {
      const level = parseInt(el.tagName.slice(1));
      el.textContent = `${'#'.repeat(level)} ${el.textContent.trim()}`;
    },
    STRONG(el) {
      return `**${el.textContent}**`;
    },
    EM(el) {
      return `_${el.textContent}_`;
    },
    DEL(el) {
      return `~~${el.textContent}~~`;
    },

    A(el) {
      const text = el.textContent || 'link';
      const href = el.getAttribute('href');
      if (/^https?:/.test(text) && text === href) {
        return text;
      }
      return href ? `[${text}](${href})` : text;
    },
    IMG(el) {
      const alt = el.getAttribute('alt') || 'image';
      const src = el.getAttribute('src');
      const widthAttr = el.hasAttribute('width') ? ` width="${htmlEscape(el.getAttribute('width') || '')}"` : '';
      const heightAttr = el.hasAttribute('height') ? ` height="${htmlEscape(el.getAttribute('height') || '')}"` : '';
      if (widthAttr || heightAttr) {
        return `<img alt="${htmlEscape(alt)}"${widthAttr}${heightAttr} src="${htmlEscape(src)}">`;
      }
      return `![${alt}](${src})`;
    },

    P(el) {
      el.textContent = `${el.textContent}\n`;
    },
    BLOCKQUOTE(el) {
      el.textContent = `${el.textContent.replace(/^/mg, '> ')}\n`;
    },

    OL(el) {
      const preNewLine = ctx.listNestingLevel ? '\n' : '';
      el.textContent = `${preNewLine}${el.textContent}\n`;
    },
    LI(el) {
      const parent = el.parentNode;
      const bullet = parent.tagName === 'OL' ? `1. ` : '* ';
      const nestingIdentLevel = Math.max(0, ctx.listNestingLevel - 1);
      el.textContent = `${' '.repeat(nestingIdentLevel * 4)}${bullet}${el.textContent}${ctx.elementIsLast ? '' : '\n'}`;
      return el;
    },
    INPUT(el) {
      return el.checked ? '[x] ' : '[ ] ';
    },

    CODE(el) {
      const text = el.textContent;
      if (el.parentNode && el.parentNode.tagName === 'PRE') {
        el.textContent = `\`\`\`\n${text}\n\`\`\`\n`;
        return el;
      }
      if (text.includes('`')) {
        return `\`\` ${text} \`\``;
      }
      return `\`${text}\``;
    },
  };
  processors['UL'] = processors.OL;
  for (let level = 2; level <= 6; level++) {
    processors[`H${level}`] = processors.H1;
  }
  return processors;
}

function processElement(ctx :ProcessorContext, processors: Processors, el: HTMLElement) {
  if (el.hasAttribute('data-markdown-generated-content')) return el.textContent;
  if (el.tagName === 'A' && el.children.length === 1 && el.children[0].tagName === 'IMG') {
    return processElement(ctx, processors, el.children[0] as HTMLElement);
  }

  const isListContainer = el.tagName === 'OL' || el.tagName === 'UL';
  if (isListContainer) ctx.listNestingLevel++;
  for (let i = 0; i < el.children.length; i++) {
    ctx.elementIsFirst = i === 0;
    ctx.elementIsLast = i === el.children.length - 1;
    processElement(ctx, processors, el.children[i] as HTMLElement);
  }
  if (isListContainer) ctx.listNestingLevel--;

  if (processors[el.tagName]) {
    const ret = processors[el.tagName](el);
    if (ret && ret !== el) {
      el.replaceWith(typeof ret === 'string' ? document.createTextNode(ret) : ret);
    }
  }
}

export function convertHtmlToMarkdown(el: HTMLElement): string {
  const div = document.createElement('div');
  div.append(el);
  const ctx = {} as ProcessorContext;
  ctx.listNestingLevel = 0;
  processElement(ctx, prepareProcessors(ctx), el);
  return div.textContent;
}
