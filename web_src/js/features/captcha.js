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
    default:
  }
}
