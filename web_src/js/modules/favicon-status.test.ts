import {buildStatusFaviconSvg, resetActionFavicon, syncActionRunFavicon} from './favicon-status.ts';

test('buildStatusFaviconSvg uses action status icons', () => {
  const success = buildStatusFaviconSvg('success');
  expect(success).toContain('viewBox="0 0 640 640"');
  expect(success).toContain('fill:#609926');
  expect(success).toContain('M8 16A8 8 0 1 1 8 0');

  const running = buildStatusFaviconSvg('running');
  expect(running).toContain('M3.05 3.05');

  const failure = buildStatusFaviconSvg('failure');
  expect(failure).toContain('M2.343 13.657');
});

test('syncActionRunFavicon updates favicon links', () => {
  document.head.innerHTML = `
    <link rel="icon" href="/assets/img/favicon.svg" type="image/svg+xml">
    <link rel="alternate icon" href="/assets/img/favicon.png" type="image/png">
  `;
  const links = Array.from(document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]'));
  syncActionRunFavicon('running');
  for (const link of links) {
    expect(link.href).toMatch(/^data:image\/svg\+xml,/);
    expect(decodeURIComponent(link.href)).toContain('M3.05 3.05');
  }
  resetActionFavicon();
  expect(links[0].href).toContain('favicon.svg');
});
