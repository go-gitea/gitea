import type {FrontendRenderFunc} from '../plugin.ts';

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    const [player] = await Promise.all([
      import('asciinema-player'),
      import('asciinema-player/dist/bundle/asciinema-player.css'),
      import('./frontend-asciicast.css'),
    ]);
    player.create({data: opts.contentString()}, opts.container, {
      // poster (a preview frame) to display until the playback is started.
      // Set it to 1 hour (also means the end if the video is shorter) to make the preview frame show more.
      poster: 'npt:1:0:0',
    });
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
