import {pathEscapeSegments, isUrl, toOriginUrl} from './url.ts';

test('pathEscapeSegments', () => {
  expect(pathEscapeSegments('a/b/c')).toEqual('a/b/c');
  expect(pathEscapeSegments('a/b/ c')).toEqual('a/b/%20c');
});

test('isUrl', () => {
  expect(isUrl('https://example.com')).toEqual(true);
  expect(isUrl('https://example.com/')).toEqual(true);
  expect(isUrl('https://example.com/index.html')).toEqual(true);
  expect(isUrl('/index.html')).toEqual(false);
});

test('toOriginUrl', () => {
  const oldLocation = String(window.location);
  for (const origin of ['https://example.com', 'https://example.com:3000']) {
    window.location.assign(`${origin}/`);
    expect(toOriginUrl('/')).toEqual(`${origin}/`);
    expect(toOriginUrl('/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com:4000')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/org/repo.git')).toEqual(`${origin}/org/repo.git`);
  }
  window.location.assign(oldLocation);
});
