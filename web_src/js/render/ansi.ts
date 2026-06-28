import {AnsiUp} from 'ansi_up';

const replacements: Array<[RegExp, string]> = [
  [/\x1b\[\d+[A-H]/g, ''], // Move cursor, treat them as no-op
  [/\x1b\[\d?[JK]/g, '\r'], // Erase display/line, treat them as a Carriage Return
];

// render ANSI to HTML
export function renderAnsiInto(el: HTMLElement, line: string) {
  // create a fresh ansi_up instance because otherwise previous renders can influence
  // the output of future renders, because ansi_up is stateful and remembers things like
  // unclosed opening tags for colors.
  const ansi_up = new AnsiUp();
  ansi_up.use_classes = true;

  if (line.endsWith('\r\n')) {
    line = line.substring(0, line.length - 2);
  } else if (line.endsWith('\n')) {
    line = line.substring(0, line.length - 1);
  }

  if (line.includes('\x1b')) {
    for (const [regex, replacement] of replacements) {
      line = line.replace(regex, replacement);
    }
  }

  let result: string;
  if (!line.includes('\r')) {
    result = ansi_up.ansi_to_html(line);
  } else {
    // handle "\rReading...1%\rReading...5%\rReading...100%",
    // convert it into a multiple-line string: "Reading...1%\nReading...5%\nReading...100%"
    const lines: Array<string> = [];
    for (const part of line.split('\r')) {
      if (part === '') continue;
      const partHtml = ansi_up.ansi_to_html(part);
      if (partHtml !== '') {
        lines.push(partHtml);
      }
    }
    // the log message element is with "white-space: break-spaces;", so use "\n" to break lines
    result = lines.join('\n');
  }

  el.innerHTML = result;
  renderAnsiPostWalk(el);
}

function renderAnsiProcessText(node: ChildNode): ChildNode {
  const text = node.textContent!;
  // TODO: fine tune this regexp, and fine tune the last punctuation mark like "open url https://gitea.com."
  const match = /\bhttps?:\/\/[^\s<>[\]]+/.exec(text);
  if (!match || match.index === undefined) return node;

  const before = text.slice(0, match.index);
  const url = match[0];
  const after = text.slice(match.index + url.length);

  const link = document.createElement('a');
  link.setAttribute('href', url);
  link.setAttribute('target', '_blank');
  link.textContent = url;

  const newNodes: Array<Node | string> = [];
  if (before) newNodes.push(before);
  newNodes.push(link);
  if (after) newNodes.push(after);

  node.replaceWith(...newNodes);
  return link;
}

function renderAnsiPostWalk(el: ChildNode) {
  for (let node = el.firstChild; node; node = node.nextSibling) {
    if (node.nodeName === 'A') continue;
    if (node.nodeType !== Node.TEXT_NODE) {
      renderAnsiPostWalk(node);
      continue;
    }
    node = renderAnsiProcessText(node);
  }
}
