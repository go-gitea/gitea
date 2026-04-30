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
      // @mcaptcha/vanilla-glue is published as a UMD/CJS bundle. Vite's interop sometimes
      // exposes the Widget constructor at `module.default` and sometimes (when the whole
      // CJS exports object gets wrapped) at `module.default.default`, so probe both.
      // INPUT_NAME is a read-only ES module binding and cannot be reassigned, so we let
      // the widget create its input and then rename it to match Gitea's expected field.
      const mCaptcha = await import('@mcaptcha/vanilla-glue') as any;
      const Widget: new (config: unknown) => {inputElement: HTMLInputElement} =
        typeof mCaptcha.default === 'function' ? mCaptcha.default : mCaptcha.default.default;
      const instanceURL = captchaEl.getAttribute('data-instance-url')!;

      const widget = new Widget({
        siteKey: {
          instanceUrl: new URL(instanceURL),
          key: siteKey,
        },
      });
      widget.inputElement.name = 'm-captcha-response';
      widget.inputElement.id = 'm-captcha-response';
      break;
    }
    default:
  }
}
