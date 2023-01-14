import * as AsciinemaPlayer from 'asciinema-player';

export function initAsciinemaPlayer() {
  const players = document.getElementsByClassName('asciinema-player-container');
  for (const player of players) {
    AsciinemaPlayer.create(player.getAttribute('data-asciinema-player-src'), player, {
      poster: 'npt:1:0:0',
    });
  }
}
