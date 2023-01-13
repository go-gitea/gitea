import * as AsciinemaPlayer from 'asciinema-player';

export function initAsciinemaPlayer() {
  const players = document.getElementsByClassName('asciinema-player-container');
  console.log('start');
  for (const player of players) {
    console.log('init');
    AsciinemaPlayer.create(player.getAttribute('data-asciinema-player-src'), player, {
      fit: 'both',
      preload: true,
      poster: 'npt:100:0:0',
    });
  }
}
