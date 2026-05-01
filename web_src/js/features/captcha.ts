import {isDarkTheme} from '../utils.ts';

export async function initCaptcha() {
  const captchaEl = document.querySelector('#captcha');
  if (!captchaEl) return;

  const siteKey = captchaEl.getAttribute('data-sitekey')!;
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
      // @mcaptcha/vanilla-glue 0.1.0-rc2 auto-runs on module evaluation: it reads the
      // widget URL from #mcaptcha__token-label's data-mcaptcha_url attribute and binds
      // to the existing #mcaptcha__token input (both rendered by the captcha template).
      // The widget names the input "mcaptcha__token"; rename it after import so the form
      // submits the field name that services/context/captcha.go reads.
      const instanceURL = captchaEl.getAttribute('data-instance-url')!;
      const widgetURL = new URL(instanceURL);
      widgetURL.pathname = '/widget/';
      widgetURL.search = `?sitekey=${siteKey}`;

      const label = document.querySelector('#mcaptcha__token-label')!;
      label.setAttribute('data-mcaptcha_url', widgetURL.toString());

      await import('@mcaptcha/vanilla-glue');

      const input = document.querySelector<HTMLInputElement>('#mcaptcha__token')!;
      input.name = 'm-captcha-response';
      input.id = 'm-captcha-response';
      break;
    }
    default:
  }
}
