import {html, htmlRaw} from './html.ts';

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
// The text passed to linkifyURLs is already HTML-escaped, so an HTML entity for a URL
// delimiter character (quote, apostrophe, angle bracket) marks the real end of a URL and
// must not be swallowed into it. Without this, `"https://x"` escaped to `&#34;https://x&#34;`
// leaked the trailing entity into the link (gitea/runner#1046). `&amp;` (a literal "&") is
// intentionally excluded because "&" is valid inside URLs.
// Code points: " 34/x22, ' 39/x27, < 60/x3c, > 62/x3e.
const boundaryEntityPattern = /&(?:quot|apos|lt|gt|#0*(?:34|39|60|62)|#[xX]0*(?:22|27|3[cCeE]));/;

// Convert URLs to clickable links in HTML, preserving existing HTML tags
export function linkifyURLs(htmlString: string): string {
  let inAnchor = false;
  return htmlString.replace(urlLinkifyPattern, (match, _openTagFull, openTag, _closeTagFull, closeTag, url) => {
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

    // Cut the URL at the first HTML entity that decodes to a delimiter; the entity and
    // everything after it is already HTML-escaped text, so emit it verbatim after the link.
    let remaining = '';
    const boundary = url.match(boundaryEntityPattern);
    if (boundary) {
      remaining = url.slice(boundary.index);
      url = url.slice(0, boundary.index);
    }

    const trailingPunct = url.match(trailingPunctPattern);
    const cleanUrl = trailingPunct ? url.slice(0, -trailingPunct[0].length) : url;
    const trailing = trailingPunct ? trailingPunct[0] : '';
    // safe because regexp only matches valid URLs (no quotes or angle brackets)
    return html`<a href="${htmlRaw(cleanUrl)}" target="_blank">${htmlRaw(cleanUrl)}</a>${htmlRaw(trailing)}${htmlRaw(remaining)}`;
  });
}
