import $ from 'jquery';

export function initMirrorRepoSyncUpdate() {
  $('.mirror-sync-update-button').on('click', (event) => {
    const mirrorAddress = $(event.currentTarget).data('mirror-address');
    const mirrorId = $(event.currentTarget).data('id');
    const syncTime = $(event.currentTarget).data('interval');
    $('#mirror-sync-update #mirror-address').val(mirrorAddress);
    $('#mirror-sync-update #push_mirror_id').val(mirrorId);
    $('#mirror-sync-update #push_mirror_interval').val(syncTime);
    $('#mirror-sync-update .cancel').on('click', (event) => {
      event.preventDefault();
    });
  });
}
