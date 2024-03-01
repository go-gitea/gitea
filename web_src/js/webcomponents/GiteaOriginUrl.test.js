import {toOriginUrl} from './GiteaOriginUrl.js';

test('toOriginUrl', () => {
  const oldLocation = window.location;
  for (const origin of ['https://example.com', 'https://example.com:3000']) {
    window.location = new URL(`${origin}/`);
    expect(toOriginUrl('/')).toEqual(`${origin}/`);
    expect(toOriginUrl('/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com/org/repo.git')).toEqual(`${origin}/org/repo.git`);
    expect(toOriginUrl('https://another.com:4000')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/')).toEqual(`${origin}/`);
    expect(toOriginUrl('https://another.com:4000/org/repo.git')).toEqual(`${origin}/org/repo.git`);
  }
  window.location = oldLocation;
});
