import {findUrlAtPosition} from './utils.ts';

test('findUrlAtPosition', () => {
  const doc = 'visit https://example.com for info';
  expect(findUrlAtPosition(doc, 0)).toBeNull();
  expect(findUrlAtPosition(doc, 6)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 15)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 24)).toEqual('https://example.com');
  expect(findUrlAtPosition(doc, 25)).toBeNull();
});
