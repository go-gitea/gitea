import type {FrontendRenderFunc} from '../plugin.ts';
import {initSwaggerUI} from '../swagger.ts';

// HINT: SWAGGER-CSS-IMPORT: this import is also necessary when swagger is used as a frontend external render
// But it can't share the same CSS file with the standalone page: it triggers our Vite manifest parser's bug
// Although single top-level "await import(css)" can work, it requires es2022.
// Otherwise, single function-level "await import(css)" can't work due to Vite's dependency analysis and bundling.
import '../../../css/swagger-render.css';

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    await import('../../../css/swagger-render.css');
    await initSwaggerUI(opts.container, {specText: opts.contentString()});
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
