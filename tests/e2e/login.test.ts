import {createHmac} from 'node:crypto';
import {env} from 'node:process';
import {test, expect, type Page} from '@playwright/test';
import {apiCreateUser, loginUser, logout, randomString, signInWithPassword} from './utils.ts';

test('homepage', async ({page}) => {
  await page.goto('/');
  await expect(page.getByRole('img', {name: 'Logo'})).toHaveAttribute('src', '/assets/img/logo.svg');
});

test('login form and logout', async ({page}) => {
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(env.GITEA_TEST_E2E_USER);
  await page.getByLabel('Password').fill(env.GITEA_TEST_E2E_PASSWORD);
  await page.getByRole('button', {name: 'Sign In'}).click();
  await expect(page.getByRole('link', {name: 'Sign In'})).toBeHidden();
  await logout(page);
});

function totpCode(secret: string, stepOffset: number) {
  const bits = Array.from(secret, (char) => 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'.indexOf(char).toString(2).padStart(5, '0')).join('');
  const key = Buffer.from(bits.match(/.{8}/g)!.map((byte) => parseInt(byte, 2)));
  const counter = Buffer.alloc(8);
  counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000) + stepOffset));
  const hmac = createHmac('sha1', key).update(counter).digest();
  return String((hmac.readUInt32BE(hmac[19] & 15) & 0x7fffffff) % 1000000).padStart(6, '0');
}

async function submitPasscode(page: Page, passcode: string) {
  await page.getByLabel('Passcode').fill(passcode);
  await page.getByRole('button', {name: 'Verify'}).click();
}

test('totp sign-in and attempt limit', async ({page, request}) => {
  const username = `e2e-totp-${randomString(8)}`;
  await apiCreateUser(request, username);
  await loginUser(page, username);
  await page.goto('/user/settings/security/two_factor/enroll');
  const secret = (await page.locator('.user-setting-content img').getAttribute('alt'))!;
  await submitPasscode(page, totpCode(secret, 0));
  await expect(page.getByText('successfully enrolled')).toBeVisible();

  await signInWithPassword(page, username);
  await page.waitForURL('**/user/two_factor');
  await submitPasscode(page, totpCode(secret, 1));
  await expect(page).not.toHaveURL(/\/user\/(login|two_factor)/);

  await signInWithPassword(page, username);
  await page.waitForURL('**/user/two_factor');
  for (let attempt = 0; attempt < 5; attempt++) {
    await submitPasscode(page, '0000000');
    await expect(page.getByLabel('Passcode')).toHaveValue('');
  }
  await expect(page.getByText('Your passcode is incorrect')).toBeVisible();
  await submitPasscode(page, '0000000');
  await expect(page.getByText('Too many incorrect passcodes')).toBeVisible();
});
