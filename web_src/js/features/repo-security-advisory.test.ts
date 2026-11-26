import {composeCvssVector, initSecurityAdvisoryForm, parseCvssVector} from './repo-security-advisory.ts';

test('CVSS vector composition orders base metrics and preserves optional metrics', () => {
  const prefix = 'CVSS:3.1/';
  const metrics = parseCvssVector(' CVSS:3.1/AV:N/AC:L/E:P ', prefix);
  expect(parseCvssVector('CVSS:4.0/AV:N', prefix).size).toBe(0);
  expect(composeCvssVector(prefix, ['AC', 'AV'], metrics)).toBe('CVSS:3.1/AC:L/AV:N/E:P');
  expect(composeCvssVector(prefix, ['AV', 'AC', 'PR'], metrics)).toBe('');
});

test('severity selection shows and submits only the active CVSS vector', () => {
  const form = document.createElement('form');
  form.innerHTML = `
    <input type="hidden" name="severity" value="cvss:3.1">
    <div data-cvss-version="3.1"><input name="cvss_vector"><span data-cvss-result></span></div>
    <div data-cvss-version="4.0"><input name="cvss_vector"><span data-cvss-result></span></div>`;
  initSecurityAdvisoryForm(form);
  const severity = form.querySelector<HTMLInputElement>('[name="severity"]')!;
  const [v31, v40] = form.querySelectorAll<HTMLInputElement>('[name="cvss_vector"]');
  v31.value = 'CVSS:3.1/AV:N';
  v40.value = 'CVSS:4.0/AV:N';
  expect(new FormData(form).getAll('cvss_vector')).toEqual([v31.value]);

  severity.value = 'cvss:4.0';
  severity.dispatchEvent(new Event('change'));
  expect(v31.parentElement!.classList.contains('tw-hidden')).toBe(true);
  expect(v40.parentElement!.classList.contains('tw-hidden')).toBe(false);
  expect(new FormData(form).getAll('cvss_vector')).toEqual([v40.value]);

  severity.value = 'low';
  severity.dispatchEvent(new Event('change'));
  expect(new FormData(form).has('cvss_vector')).toBe(false);
});
