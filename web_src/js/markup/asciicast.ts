import {queryElems} from '../utils/dom.ts';

export async function initMarkupRenderAsciicast(elMarkup: HTMLElement): Promise<void> {
  queryElems(elMarkup, '.asciinema-player-container', async (el) => {
    const [player] = await Promise.all([
      // @ts-expect-error: module exports no types
      import(/* webpackChunkName: "asciinema-player" */'asciinema-player'),
      import(/* webpackChunkName: "asciinema-player" */'asciinema-player/dist/bundle/asciinema-player.css'),
    ]);

    player.create(el.getAttribute('data-asciinema-player-src'), el, {
      // poster (a preview frame) to display until the playback is started.
      // Set it to 1 hour (also means the end if the video is shorter) to make the preview frame show more.
      poster: 'npt:1:0:0',
    });
  });
}
