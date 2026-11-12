import {composeCvssVector, parseCvssVector} from './repo-security-advisory.ts';

test('parseCvssVector', () => {
  expect(parseCvssVector('CVSS:3.1/AV:N/AC:L', 'CVSS:4.0/').size).toBe(0);
  expect(Array.from(parseCvssVector(' CVSS:3.1/AV:N/AC:L/E:P ', 'CVSS:3.1/'))).toEqual([['AV', 'N'], ['AC', 'L'], ['E', 'P']]);
});

test('composeCvssVector', () => {
  const metrics = new Map([['E', 'P'], ['AC', 'L'], ['AV', 'N']]);
  expect(composeCvssVector('CVSS:3.1/', ['AV', 'AC'], metrics)).toBe('CVSS:3.1/AV:N/AC:L/E:P');
  expect(composeCvssVector('CVSS:3.1/', ['AV', 'AC', 'PR'], metrics)).toBe('');
});
