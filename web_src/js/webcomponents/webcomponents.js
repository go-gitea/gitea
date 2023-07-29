// do not use dynamic `import()` in any of these files because the script tag does not
// have type=module, it need to load synchronously to avoid content flickering
import '@webcomponents/custom-elements'; // polyfill for some browsers like Pale Moon
import '@github/relative-time-element';
import './GiteaOriginUrl.js';
