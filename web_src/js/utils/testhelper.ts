// browser tests can not stub "window.location", so navigation goes through these wrappers instead

export function navigateTo(href: string) {
  window.location.assign(href);
}

export function reloadPage() {
  window.location.reload();
}

/** strip common indentation from a string and trim it */
export function dedent(str: string) {
  const match = str.match(/^[ \t]*(?=\S)/gm);
  if (!match) return str;

  let minIndent = Infinity;
  for (const indent of match) {
    minIndent = Math.min(minIndent, indent.length);
  }
  if (minIndent === 0 || minIndent === Infinity) {
    return str;
  }

  return str.replace(new RegExp(`^[ \\t]{${minIndent}}`, 'gm'), '').trim();
}

export function normalizeTestHtml(s: string) {
  const lines = s.replace(/>\s+</g, '>\n<').trim().split('\n');
  for (let i = 0; i < lines.length; i++) {
    lines[i] = lines[i].trim();
  }
  return lines.join('\n');
}
