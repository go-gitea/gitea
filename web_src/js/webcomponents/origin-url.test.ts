import {toOriginUrl} from './origin-url.ts';

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
