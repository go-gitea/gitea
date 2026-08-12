import {buildStatusFaviconSvg, resetActionFavicon, syncActionRunFavicon} from './favicon-status.ts';

test('buildStatusFaviconSvg uses action status icons', () => {
  const success = buildStatusFaviconSvg('success');
  expect(success).toContain('viewBox="0 0 640 640"');
  expect(success).toContain('fill:#609926');
  expect(success).toContain('data-actions-status-name="success"');

  const running = buildStatusFaviconSvg('running');
  expect(running).toContain('data-actions-status-name="running"');

  const failure = buildStatusFaviconSvg('failure');
  expect(failure).toContain('data-actions-status-name="failure"');
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
    expect(decodeURIComponent(link.href)).toContain('data-actions-status-name="running"');
  }
  resetActionFavicon();
  expect(links[0].href).toContain('favicon.svg');
});
