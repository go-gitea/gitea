import {isDarkTheme} from '../utils.ts';
import {registerGlobalEventFunc} from '../modules/observer.ts';

export async function initCaptcha() {
  registerGlobalEventFunc('click', 'onImageCaptchaReload', (btn: HTMLElement) => {
    const img = btn.closest('.image-captcha')!.querySelector('img')!;
    const url = new URL(img.src);
    url.searchParams.set('reload', String(Date.now()));
    img.src = url.href;
  });

  const captchaEl = document.querySelector('#captcha');
  if (!captchaEl) return;

  const siteKey = captchaEl.getAttribute('data-sitekey')!;
  const isDark = isDarkTheme();

  const params = {
    sitekey: siteKey,
    theme: isDark ? 'dark' : 'light',
  };

  switch (captchaEl.getAttribute('data-captcha-type') ?? '') {
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
      // ref: https://github.com/mCaptcha/glue/blob/master/packages/vanilla/README.md
      // sample: https://github.com/mCaptcha/glue/blob/master/packages/vanilla/static/embeded.html
      // @mcaptcha/vanilla-glue 0.1.0-rc2 auto-runs on module load, use the existing elements to render.
      await import('@mcaptcha/vanilla-glue');
      break;
    }
    default:
  }
}
