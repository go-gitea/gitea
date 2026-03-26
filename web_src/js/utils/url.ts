export function pathEscapeSegments(s: string): string {
  return s.split('/').map(encodeURIComponent).join('/');
}

// Match HTML tags (to skip) or URLs (to linkify) in ANSI-rendered HTML output
const urlLinkifyPattern = /(<[^>]*>)|(https?:\/\/[^\s<>"'`|(){}[\]]+)/gi;
const trailingPunctPattern = /[.,;:!?]+$/;

// Convert URLs to clickable links in HTML, preserving existing HTML tags
export function linkifyURLs(html: string): string {
  let inAnchor = false;
  return html.replace(urlLinkifyPattern, (_match, tag, url) => {
    if (tag) {
      // skip URLs inside existing <a> tags (e.g. from ansi_up OSC 8 hyperlinks)
      if (tag.startsWith('<a ') || tag.startsWith('<a>')) { // eslint-disable-line github/unescaped-html-literal
        inAnchor = true;
      } else if (tag === '</a>') {
        inAnchor = false;
      }
      return tag;
    }
    if (inAnchor) return url;
    const trailingPunct = url.match(trailingPunctPattern);
    const cleanUrl = trailingPunct ? url.slice(0, -trailingPunct[0].length) : url;
    const trailing = trailingPunct ? trailingPunct[0] : '';
    // safe because ansi_up already HTML-escaped the content
    return `<a href="${cleanUrl}" target="_blank">${cleanUrl}</a>${trailing}`; // eslint-disable-line github/unescaped-html-literal
  });
}

/** Convert an absolute or relative URL to an absolute URL with the current origin. It only
 *  processes absolute HTTP/HTTPS URLs or relative URLs like '/xxx' or '//host/xxx'. */
export function toOriginUrl(urlStr: string) {
  try {
    if (urlStr.startsWith('http://') || urlStr.startsWith('https://') || urlStr.startsWith('/')) {
      const {origin, protocol, hostname, port} = window.location;
      const url = new URL(urlStr, origin);
      url.protocol = protocol;
      url.hostname = hostname;
      url.port = port || (protocol === 'https:' ? '443' : '80');
      return url.toString();
    }
  } catch {}
  return urlStr;
}
