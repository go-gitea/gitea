export function pathEscapeSegments(s: string): string {
  return s.split('/').map(encodeURIComponent).join('/');
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
