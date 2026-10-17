import type {FrontendRenderFunc} from '../plugin.ts';

export const frontendRender: FrontendRenderFunc = async (opts): Promise<boolean> => {
  try {
    const [player] = await Promise.all([
      import('asciinema-player'),
      import('asciinema-player/dist/bundle/asciinema-player.css'),
    ]);
    player.create({data: opts.contentString()}, opts.container, {
      // poster (a preview frame) to display until the playback is started.
      // Set it to 1 hour (also means the end if the video is shorter) to make the preview frame show more.
      poster: 'npt:1:0:0',
    });
    // Related: https://github.com/asciinema/asciinema-player/blob/develop/src/components/Terminal.js : <div class="ap-term" ...>
    // Old PR: Fix UI regression of asciinema player https://github.com/go-gitea/gitea/pull/26159
    opts.container.querySelector<HTMLElement>('.ap-term')!.style.overflow = 'hidden';
    opts.container.querySelector<HTMLElement>('.ap-player')!.style.borderRadius = '0';
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
