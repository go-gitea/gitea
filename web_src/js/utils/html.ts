export function htmlEscape(s: string, ...args: Array<any>): string {
  if (args.length !== 0) throw new Error('use html or htmlRaw instead of htmlEscape'); // check legacy usages
  return s.replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

class rawObject {
  private readonly value: string;
  constructor(v: string) { this.value = v }
  toString(): string { return this.value }
}

export function html(tmpl: TemplateStringsArray, ...parts: Array<any>): string {
  let output = tmpl[0];
  for (let i = 0; i < parts.length; i++) {
    const value = parts[i];
    const valueEscaped = (value instanceof rawObject) ? value.toString() : htmlEscape(String(value));
    output = output + valueEscaped + tmpl[i + 1];
  }
  return output;
}

export function htmlRaw(s: string|TemplateStringsArray, ...tmplParts: Array<any>): rawObject {
  if (typeof s === 'string') {
    if (tmplParts.length !== 0) throw new Error("either htmlRaw('str') or htmlRaw`tmpl`");
    return new rawObject(s);
  }
  return new rawObject(html(s, ...tmplParts));
}
