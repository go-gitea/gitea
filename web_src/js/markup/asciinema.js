export async function renderAsciinemaPlayer() {
  const els = document.querySelectorAll('.asciinema-player-container');
  if (!els.length) return;

  const player = await import(/* webpackChunkName: "asciinema" */'asciinema-player');

  for (const el of els) {
    player.create(el.getAttribute('data-asciinema-player-src'), el, {
      poster: 'npt:1:0:0',
    });
  }
}
