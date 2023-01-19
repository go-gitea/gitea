import {isDarkTheme} from '../utils.js';

export function initCaptcha() {
  const captchaEl = document.querySelector('#captcha');
  if (!captchaEl) return;

  const siteKey = captchaEl.getAttribute('data-sitekey');
  const isDark = isDarkTheme();

  const params = {
    sitekey: siteKey,
    theme: isDark ? 'dark' : 'light'
  };

  switch (captchaEl.getAttribute('captcha-type')) {
    case 'g-recaptcha': {
      // eslint-disable-next-line no-undef
      if (grecaptcha) {
        // eslint-disable-next-line no-undef
        grecaptcha.ready(() => {
          // eslint-disable-next-line no-undef
          grecaptcha.render(captchaEl, params);
        });
      }
      break;
    }
    case 'cf-turnstile': {
      // eslint-disable-next-line no-undef
      if (turnstile) {
        // eslint-disable-next-line no-undef
        turnstile.render(captchaEl, params);
      }
      break;
    }
    case 'h-captcha': {
      // eslint-disable-next-line no-undef
      hcaptcha.render(captchaEl, params);
      break;
    }
    default:
  }
}
