import {weakRefClass} from './polyfills.ts';

test('polyfillWeakRef', () => {
  const WeakRef = weakRefClass();
  const r = new WeakRef(123);
  expect(r.deref()).toEqual(123);
});
