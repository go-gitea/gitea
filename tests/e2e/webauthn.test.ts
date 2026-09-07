import {test, expect, type Page} from '@playwright/test';
import {apiCreateUser, loginUser, randomString, testUserPassword} from './utils.ts';

const signedIn = /^(?!.*\/user\/(login|webauthn))/; // the target of a finished login varies

async function registerKey(page: Page, nickname: string) {
  await page.goto('/user/settings/security');
  await page.getByLabel('Nickname').fill(nickname);
  await page.getByRole('button', {name: 'Add Security Key'}).click();
}

async function signInWithPassword(page: Page, username: string) {
  await page.context().clearCookies();
  await page.goto('/user/login');
  await page.getByLabel('Username or Email Address').fill(username);
  await page.getByLabel('Password').fill(testUserPassword);
  await page.getByRole('button', {name: 'Sign In'}).click();
}

// regression: credProtect level 3 hid the credential from the second-factor login
test('security key survives credProtect', async ({page, request, browserName}) => {
  test.skip(browserName !== 'chromium', 'only the CDP authenticator emulates credProtect'); // eslint-disable-line playwright/no-skipped-test -- conditional skip, the reason is in the message

  const username = `e2e-credprotect-${randomString(8)}`;
  await apiCreateUser(request, username);

  const cdp = await page.context().newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  await cdp.send('WebAuthn.addVirtualAuthenticator', {options: {
    protocol: 'ctap2',
    ctap2Version: 'ctap2_1',
    transport: 'usb',
    hasResidentKey: true,
    hasUserVerification: true,
    hasCredBlob: true, // CDP only emulates credProtect together with credBlob
    isUserVerified: true,
  }});

  await loginUser(page, username);
  await registerKey(page, 'e2e-key');
  await expect(page.getByText('e2e-key')).toBeVisible();

  await registerKey(page, 'e2e-key-again');
  await expect(page.locator('#webauthn-error-msg')).toContainText('already registered');

  await signInWithPassword(page, username);
  await expect(page).toHaveURL(signedIn);
});

// this authenticator has no credProtect, so it cannot replace the test above
test('security key signs in as second factor and as passkey', async ({page, request}) => {
  const username = `e2e-passkey-${randomString(8)}`;
  await apiCreateUser(request, username);
  await page.context().credentials.install();

  await loginUser(page, username);
  await registerKey(page, 'e2e-key');
  await expect(page.getByText('e2e-key')).toBeVisible();

  await signInWithPassword(page, username);
  await expect(page).toHaveURL(signedIn);

  await page.context().clearCookies();
  await page.goto('/user/login');
  await page.getByText('Sign in with a passkey').click();
  await expect(page).toHaveURL(signedIn);
});
