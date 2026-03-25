import {html, htmlRaw} from './html.ts';

export function pathEscapeSegments(s: string): string {
  return s.split('/').map(encodeURIComponent).join('/');
}

// Match HTML tags (to skip) or URLs (to linkify) in ANSI-rendered HTML output
const urlLinkifyPattern = /(<[^>]*>)|(https?:\/\/[^\s<>"'`|(){}[\]]+)/gi;
const trailingPunctPattern = /[.,;:!?]+$/;

// Convert URLs to clickable links in HTML, preserving existing HTML tags
export function linkifyURLs(content: string): string {
  return content.replace(urlLinkifyPattern, (_match, tag, url) => {
    if (tag) return tag;
    const trailingPunct = url.match(trailingPunctPattern);
    const cleanUrl = trailingPunct ? url.slice(0, -trailingPunct[0].length) : url;
    const trailing = trailingPunct ? trailingPunct[0] : '';
    const rawUrl = htmlRaw(cleanUrl);
    return html`<a href="${rawUrl}" target="_blank" rel="noopener noreferrer">${rawUrl}</a>` + trailing;
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
