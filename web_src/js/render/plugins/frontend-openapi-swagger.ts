import type {FrontendRenderFunc} from '../plugin.ts';
import {initSwaggerUI} from '../swagger.ts';

// HINT: SWAGGER-CSS-IMPORT: this import is also necessary when swagger is used as a frontend external render
// It must be on top-level, doesn't work in a function
// Static import doesn't work (it needs to use manifest.json to manually add the CSS file)
await import('../../../css/swagger.css');

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    await initSwaggerUI(opts.container, {specText: opts.contentString()});
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
