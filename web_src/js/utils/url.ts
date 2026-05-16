export function urlQueryEscape(s: string) {
  // See "TestQueryEscape" in backend
  // https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/encodeURIComponent#encoding_for_rfc3986
  return encodeURIComponent(s).replace(
    /[!'()*]/g,
    (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`,
  ).replaceAll('%20', '+');
}

export function pathEscape(s: string): string {
  // See "TestPathEscape" in backend
  return encodeURIComponent(s).replace(
    /[!'()*]/g,
    (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`,
  ).replaceAll(/%(\w\w)/g, (v) => {
    switch (v) {
      case '%24': return '$';
      case '%26': return '&';
      case '%2B': return '+';
      case '%3A': return ':';
      case '%3D': return '=';
      case '%40': return '@';
      default: return v;
    }
  });
}

export function pathEscapeSegments(s: string): string {
  // The same as backend's PathEscapeSegments
  return s.split('/').map(pathEscape).join('/');
}

// Match HTML tags (to skip) or URLs (to linkify) in HTML content
const urlLinkifyPattern = /(<([-\w]+)[^>]*>)|(<\/([-\w]+)[^>]*>)|(https?:\/\/[^\s<>"'`|(){}[\]]+)/gi;
const trailingPunctPattern = /[.,;:!?]+$/;

// Convert URLs to clickable links in HTML, preserving existing HTML tags
export function linkifyURLs(html: string): string {
  let inAnchor = false;
  return html.replace(urlLinkifyPattern, (match, _openTagFull, openTag, _closeTagFull, closeTag, url) => {
    // skip URLs inside existing <a> tags
    if (openTag === 'a') {
      inAnchor = true;
      return match;
    } else if (closeTag === 'a') {
      inAnchor = false;
      return match;
    }
    if (inAnchor || !url) {
      return match;
    }

    const trailingPunct = url.match(trailingPunctPattern);
    const cleanUrl = trailingPunct ? url.slice(0, -trailingPunct[0].length) : url;
    const trailing = trailingPunct ? trailingPunct[0] : '';
    // safe because regexp only matches valid URLs (no quotes or angle brackets)
    return `<a href="${cleanUrl}" target="_blank">${cleanUrl}</a>${trailing}`; // eslint-disable-line github/unescaped-html-literal
  });
}
