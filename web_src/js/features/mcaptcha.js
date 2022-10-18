export async function initMcaptcha() {
  const mCaptchaEl = document.querySelector('.m-captcha');
  if (!mCaptchaEl) return;

  const {default: mCaptcha} = await import(/* webpackChunkName: "mcaptcha-vanilla-glue" */'@mcaptcha/vanilla-glue');
  mCaptcha.INPUT_NAME = 'm-captcha-response';
  const siteKey = mCaptchaEl.getAttribute('data-sitekey');
  const instanceURL = mCaptchaEl.getAttribute('data-instance-url');

  mCaptcha.default({
    siteKey: {
      instanceUrl: new URL(instanceURL),
      key: siteKey,
    }
  });
}
