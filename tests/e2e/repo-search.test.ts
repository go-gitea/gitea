import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, randomString, apiCreateRepo, apiCreateFile, apiDeleteRepo} from './utils.ts';

test('code search returns results for all-numeric search terms', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-repo-search-${randomString(6)}`;

  // "vlan" is the control query (letter tokenizer handles letters correctly).
  // "699" is the bug probe (letter tokenizer produces zero tokens for digit-only strings).
  const fileContent = [
    'interface GigabitEthernet0/0',
    ' description WAN uplink',
    'interface vlan 699',
    ' description Finance VLAN',
    ' ip address 10.20.30.1 255.255.255.0',
  ].join('\n');

  await apiCreateRepo(request, {name: repo, autoInit: true});

  try {
    await Promise.all([
      apiCreateFile(request, owner, repo, 'network.cfg', fileContent),
      login(page),
    ]);

    // Poll until the bleve indexer has processed the file, using "vlan" as control.
    const searchUrl = `/${owner}/${repo}/search`;
    let controlIndexed = false;
    for (let attempt = 0; attempt < 40; attempt++) {
      await page.goto(`${searchUrl}?q=vlan&type=code`);
      await page.waitForLoadState('load');
      if (await page.locator('.repo-search-result').count() > 0) {
        controlIndexed = true;
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 3000));
    }
    expect(controlIndexed, 'control query "vlan" must return results within 120s — indexer must be running').toBe(true);

    await page.goto(`${searchUrl}?q=699&type=code`);
    await page.waitForLoadState('load');

    // Fails on v1.25.5: repoIndexerAnalyzer uses tokenizer: letter.Name which drops digit sequences.
    await expect(
      page.locator('.repo-search-result').first(),
      'searching for "699" must find "network.cfg" (contains "interface vlan 699")',
    ).toBeVisible();
  } finally {
    await apiDeleteRepo(request, owner, repo);
  }
});
