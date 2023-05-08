export async function renderAsciicast() {
  const els = document.querySelectorAll('.asciinema-player-container');
  if (!els.length) return;

  const [player] = await Promise.all([
    import(/* webpackChunkName: "asciinema-player" */'asciinema-player'),
    import(/* webpackChunkName: "asciinema-player" */'asciinema-player/dist/bundle/asciinema-player.css'),
  ]);

  for (const el of els) {
    player.create(el.getAttribute('data-asciinema-player-src'), el, {
      // poster (a preview frame) to display until the playback is started.
      // Set it to 1 hour (also means the end if the video is shorter) to make the preview frame show more.
      poster: 'npt:1:0:0',
    });
  }
}
