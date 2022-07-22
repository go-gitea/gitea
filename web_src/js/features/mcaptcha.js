export async function initMcaptcha() {
  const siteKeyEl = document.querySelector('.m-captcha');
  if (!siteKeyEl) {
    return;
  }

  const {default: mCaptcha} = await import(/* webpackChunkName: "mcaptcha-vanilla-glue" */'@mcaptcha/vanilla-glue');
  mCaptcha.INPUT_NAME = 'm-captcha-response';
  const siteKey = siteKeyEl.getAttribute('data-sitekey');

  mCaptcha.default({
    siteKey: {
      instanceUrl: new URL('http://localhost:7000'),
      key: siteKey,
    }
  });
}
