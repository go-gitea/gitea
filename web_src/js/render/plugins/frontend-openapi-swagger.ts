import type {FrontendRenderFunc} from '../plugin.ts';

// HINT: SWAGGER-CSS-IMPORT: this import is also necessary when swagger is used as a frontend external render
await import('../../../css/swagger.css');

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  const {initSwaggerUI} = await import('../swagger.ts');
  try {
    await initSwaggerUI(opts.container, {specText: opts.contentString()});
    return true;
  } catch {
    return false;
  }
};
