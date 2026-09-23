import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, randomString} from './utils.ts';

test('create a bot and manage its access token', async ({page, request}) => {
  const botName = `e2e-bot-${randomString(8)}`;
  await login(page);
  await page.goto('/-/admin/users/new');
  await page.getByLabel('Username').fill(botName);
  await page.getByLabel('Username').press('Tab');
  await page.getByRole('option', {name: 'Bot'}).click();
  await page.getByLabel('Email Address').fill(`${botName}@${env.GITEA_TEST_E2E_DOMAIN}`);
  await page.getByRole('button', {name: 'Create User Account'}).click();
  await page.waitForURL(/\/-\/admin\/users\/\d+$/);

  await page.getByLabel('Token Name').fill('e2e-token');
  await page.getByRole('row', {name: /^user /}).getByRole('radio', {name: 'Read', exact: true}).check();
  await page.getByRole('button', {name: 'Generate Token'}).click();
  const token = await page.getByRole('code').textContent();
  const response = await request.get('/api/v1/user', {headers: {Authorization: `token ${token}`}});
  expect(await response.json()).toMatchObject({login: botName, type: 'Bot'});

  await page.getByRole('button', {name: 'Delete'}).click();
  await page.getByRole('button', {name: 'Yes'}).click();
  await expect(page.getByText('The token has been deleted.')).toBeVisible();
});
