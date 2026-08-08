/**
 * APIA v2 — Playwright frontend measurement.
 * Hits representative pages on both environments, collects Core Web Vitals
 * via the browser's native PerformanceObserver API, and writes results to
 * OUTPUT_FILE as JSON.
 *
 * Outputs p75 across RUNS_PER_PAGE cold-start runs per page.
 * Uses p75 to match Google's CWV measurement methodology.
 */

const { chromium } = require('playwright');
const fs = require('fs');

const BASELINE_URL   = process.env.BASELINE_URL  || 'http://localhost:30301';
const STAGING_URL    = process.env.STAGING_URL   || 'http://localhost:30302';
const OUTPUT_FILE    = process.env.OUTPUT_FILE   || '/tmp/playwright-results.json';
const RUNS_PER_PAGE  = 3;

const PAGES = ['/', '/explore/repos', '/explore/users', '/user/login'];

async function measurePage(browser, baseURL, path) {
  const context = await browser.newContext({
    // Disable cache for cold-start measurement
    bypassCSP: true,
  });
  const page = await context.newPage();

  // Inject observers before navigation so they catch all events
  await page.addInitScript(() => {
    window.__vitals = { lcp: null, fcp: null, cls: 0 };

    try {
      new PerformanceObserver(list => {
        const entries = list.getEntries();
        if (entries.length) {
          window.__vitals.lcp = entries[entries.length - 1].startTime;
        }
      }).observe({ type: 'largest-contentful-paint', buffered: true });
    } catch (_) {}

    try {
      new PerformanceObserver(list => {
        for (const e of list.getEntries()) {
          if (e.name === 'first-contentful-paint' && window.__vitals.fcp === null) {
            window.__vitals.fcp = e.startTime;
          }
        }
      }).observe({ type: 'paint', buffered: true });
    } catch (_) {}

    try {
      new PerformanceObserver(list => {
        for (const e of list.getEntries()) {
          if (!e.hadRecentInput) window.__vitals.cls += e.value;
        }
      }).observe({ type: 'layout-shift', buffered: true });
    } catch (_) {}
  });

  let ttfb = null;
  try {
    const response = await page.goto(baseURL + path, {
      waitUntil: 'networkidle',
      timeout: 15000,
    });
    if (response) {
      const timing = response.timing();
      ttfb = Math.round(timing.responseStart - timing.requestStart);
    }
  } catch (e) {
    await context.close();
    return null;
  }

  // Wait for LCP to stabilise (LCP can update as late content loads)
  await page.waitForTimeout(1500);

  const vitals = await page.evaluate(() => {
    const nav = performance.getEntriesByType('navigation')[0];
    const navTTFB = nav ? Math.round(nav.responseStart - nav.requestStart) : null;
    return {
      lcp:  window.__vitals.lcp  ? Math.round(window.__vitals.lcp)  : null,
      fcp:  window.__vitals.fcp  ? Math.round(window.__vitals.fcp)  : null,
      cls:  Math.round(window.__vitals.cls * 1000) / 1000,
      ttfb: navTTFB,
    };
  });

  await context.close();
  return { ...vitals, ttfb: vitals.ttfb ?? ttfb };
}

function p75(arr) {
  const clean = arr.filter(v => v !== null && v !== undefined);
  if (!clean.length) return null;
  const sorted = [...clean].sort((a, b) => a - b);
  const idx = Math.ceil(0.75 * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

async function measureEnvironment(label, baseURL) {
  console.log(`[playwright] measuring ${label} (${baseURL})...`);
  const browser = await chromium.launch({ args: ['--no-sandbox', '--disable-dev-shm-usage'] });

  const allLcp = [], allFcp = [], allTtfb = [], allCls = [];
  const pageBreakdown = {};

  for (const path of PAGES) {
    const runLcp = [], runFcp = [], runTtfb = [], runCls = [];

    for (let run = 0; run < RUNS_PER_PAGE; run++) {
      const m = await measurePage(browser, baseURL, path);
      if (!m) continue;
      if (m.lcp  !== null) runLcp.push(m.lcp);
      if (m.fcp  !== null) runFcp.push(m.fcp);
      if (m.ttfb !== null) runTtfb.push(m.ttfb);
      runCls.push(m.cls ?? 0);
    }

    pageBreakdown[path] = {
      lcp_p75_ms:  p75(runLcp),
      fcp_p75_ms:  p75(runFcp),
      ttfb_p75_ms: p75(runTtfb),
      cls_p75:     p75(runCls),
    };

    allLcp.push(...runLcp);
    allFcp.push(...runFcp);
    allTtfb.push(...runTtfb);
    allCls.push(...runCls);

    console.log(
      `  [${label}] ${path}: LCP=${pageBreakdown[path].lcp_p75_ms}ms ` +
      `FCP=${pageBreakdown[path].fcp_p75_ms}ms TTFB=${pageBreakdown[path].ttfb_p75_ms}ms`
    );
  }

  await browser.close();

  return {
    lcp_p75_ms:  p75(allLcp),
    fcp_p75_ms:  p75(allFcp),
    ttfb_p75_ms: p75(allTtfb),
    cls_p75:     p75(allCls),
    pages:       pageBreakdown,
  };
}

async function main() {
  const baseline = await measureEnvironment('baseline', BASELINE_URL);
  const staging  = await measureEnvironment('staging',  STAGING_URL);

  const output = { baseline, staging };
  fs.writeFileSync(OUTPUT_FILE, JSON.stringify(output, null, 2));

  console.log(`[playwright] results written to ${OUTPUT_FILE}`);
  console.log(`[playwright] baseline LCP p75=${baseline.lcp_p75_ms}ms  staging LCP p75=${staging.lcp_p75_ms}ms`);
}

main().catch(e => {
  console.error('[playwright] error:', e.message);
  process.exit(1);
});
