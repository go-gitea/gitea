import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {
  login,
  randomString,
  apiCreateRepo,
  apiCreateFile,
  apiDeleteRepo,
} from './utils.ts';

/**
 * Regression test for: Indexer will not search for all-numeric search terms
 * https://github.com/go-gitea/gitea/issues/37221
 *
 * The bleve repoIndexerAnalyzer is configured with `tokenizer: letter.Name`.
 * The "letter" tokenizer only produces tokens from sequences of Unicode letters.
 * Pure digit sequences (e.g. "699") produce zero tokens at both index time and
 * query time, so they can never match any document in the bleve index.
 *
 * Root-Cause: modules/indexer/code/bleve/bleve.go — generateBleveIndexMapping()
 *   mapping.AddCustomAnalyzer(repoIndexerAnalyzer, map[string]any{
 *     "tokenizer": letter.Name,  // ← drops all digit sequences
 *   })
 *   mapping.DefaultAnalyzer = repoIndexerAnalyzer  // applied to Content field
 *
 * Scenario:
 *   1. Create a repo and commit a file containing "interface vlan 699"
 *   2. Wait for the bleve indexer to index the file (poll using "vlan" as control)
 *   3. Search for the pure-numeric term "699"
 *   → BUG: "699" returns 0 results ("No matching results found.")
 *   → FIX: "699" returns 1 result (the file containing "interface vlan 699")
 *
 * Commit convention: tests(e2e): {description} (#{issue})
 */
test.fixme('bleve indexer returns no results for all-numeric search terms', async ({page, request}) => {
  const owner = env.GITEA_TEST_E2E_USER;
  const repo = `e2e-bleve-numeric-${randomString(6)}`;

  // File content with both letter words AND pure-numeric strings on the same lines.
  // "vlan" is the control (letter tokenizer handles it correctly).
  // "699" is the bug probe (letter tokenizer drops it, producing 0 results).
  const fileContent = [
    '! Network configuration - automated regression test fixture for #37221',
    'interface GigabitEthernet0/0',
    ' description WAN uplink',
    ' ip address 192.168.1.1 255.255.255.0',
    '!',
    'interface vlan 699',
    ' description Finance VLAN',
    ' ip address 10.20.30.1 255.255.255.0',
    '!',
    'interface vlan 700',
    ' description HR VLAN',
    '!',
    'router bgp 65001',
    ' neighbor 10.0.0.1 remote-as 65002',
    '!',
    '! Config version 20240101',
  ].join('\n');

  await apiCreateRepo(request, {name: repo, autoInit: true});

  try {
    await apiCreateFile(request, owner, repo, 'network.cfg', fileContent);

    await login(page);

    // Wait for the bleve indexer to process the file.
    // The indexer runs asynchronously, so we poll using "vlan" as the control query.
    // Once "vlan" returns results, the file is indexed and we can probe "699".
    const searchUrl = `/${owner}/${repo}/search`;
    let controlIndexed = false;
    for (let attempt = 0; attempt < 40; attempt++) {
      await page.goto(`${searchUrl}?q=vlan&type=code`);
      await page.waitForLoadState('load');
      const resultCount = await page.locator('.repo-search-result').count();
      if (resultCount > 0) {
        controlIndexed = true;
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 3000));
    }

    // Screenshot: control search — proves indexer is running and the file is indexed
    // BUG and FIX: "vlan" should return results in both versions
    await page.screenshot({path: 'test-results/01-control-search-vlan.png', fullPage: true});

    expect(controlIndexed, 'Control query "vlan" must return results within 120s — indexer must be running').toBe(true);

    const vlanCount = await page.locator('.repo-search-result').count();
    expect(vlanCount, '"vlan" must return at least 1 result to confirm the file is indexed').toBeGreaterThan(0);

    // Now probe the pure-numeric term "699" — this is the bug assertion.
    await page.goto(`${searchUrl}?q=699&type=code`);
    await page.waitForLoadState('load');

    // Screenshot immediately before assertion — proves the page is in the correct state
    // BUG (v1.25.5):  Shows "No matching results found." — letter tokenizer drops "699" tokens
    // FIX (expected): Shows network.cfg with the matching line "interface vlan 699" highlighted
    await page.screenshot({path: 'test-results/02-bug-search-numeric-699.png', fullPage: true});

    const numericCount = await page.locator('.repo-search-result').count();

    // This assertion FAILS on v1.25.5 (letter tokenizer discards all digit sequences)
    // This assertion PASSES on a fixed version (e.g. using unicode or whitespace tokenizer)
    expect(
      numericCount,
      'Searching for "699" must return the file "network.cfg" which contains ' +
      '"interface vlan 699". On v1.25.5 the bleve repoIndexerAnalyzer uses letter.Name ' +
      'tokenizer which produces zero tokens for digit-only strings.',
    ).toBeGreaterThan(0);
  } finally {
    await apiDeleteRepo(request, owner, repo);
  }
});
