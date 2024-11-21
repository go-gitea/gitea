import {isDarkTheme} from '../utils.ts';

export async function initCaptcha() {
  const captchaEl = document.querySelector('#captcha');
  if (!captchaEl) return;

  const siteKey = captchaEl.getAttribute('data-sitekey');
  const isDark = isDarkTheme();

  const params = {
    sitekey: siteKey,
    theme: isDark ? 'dark' : 'light',
  };

  switch (captchaEl.getAttribute('data-captcha-type')) {
    case 'g-recaptcha': {
      if (window.grecaptcha) {
        window.grecaptcha.ready(() => {
          window.grecaptcha.render(captchaEl, params);
        });
      }
      break;
    }
    case 'cf-turnstile': {
      if (window.turnstile) {
        window.turnstile.render(captchaEl, params);
      }
      break;
    }
    case 'h-captcha': {
      if (window.hcaptcha) {
        window.hcaptcha.render(captchaEl, params);
      }
      break;
    }
    case 'm-captcha': {
      const {default: mCaptcha} = await import(/* webpackChunkName: "mcaptcha-vanilla-glue" */'@mcaptcha/vanilla-glue');
      // @ts-expect-error
      mCaptcha.INPUT_NAME = 'm-captcha-response';
      const instanceURL = captchaEl.getAttribute('data-instance-url');

      // @ts-expect-error
      mCaptcha.default({
        siteKey: {
          instanceUrl: new URL(instanceURL),
          key: siteKey,
        },
      });
      break;
    }
    default:
  }
}
