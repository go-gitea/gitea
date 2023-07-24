import $ from 'jquery';

const {csrfToken} = window.config;

export function initMirrorRepoSyncUpdate() {
  $('.mirror-sync-update-button').on('click', () => {
    const mirrorSync = $('.mirror-sync-update-button');
    const mirrorAddress = mirrorSync.attr('data-mirror-address');
    const mirrorId = mirrorSync.attr('data-mirror-push-id');
    const syncTime = mirrorSync.attr('data-sync-time');
    $('#mirror-sync-update #mirror-address').val(mirrorAddress);
    $('#mirror-sync-update #mirror-id').val(mirrorId);
    $('#mirror-sync-update #push_mirror_interval').val(syncTime);
  });
}
