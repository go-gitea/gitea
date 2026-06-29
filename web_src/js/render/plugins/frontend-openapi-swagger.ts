import type {FrontendRenderFunc} from '../plugin.ts';
import {initSwaggerUI} from '../swagger.ts';

// HINT: SWAGGER-CSS-IMPORT: these styles are for the render only.
import '../../../css/swagger-render.css';

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    await initSwaggerUI(opts.container, {specText: opts.contentString()});
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
