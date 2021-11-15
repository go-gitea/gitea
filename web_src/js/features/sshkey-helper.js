export function initSshKeyFormParser() {
// Parse SSH Key
  $('#ssh-key-content').on('change paste keyup', function () {
    const arrays = $(this).val().split(' ');
    const $title = $('#ssh-key-title');
    if ($title.val() === '' && arrays.length === 3 && arrays[2] !== '') {
      $title.val(arrays[2]);
    }
  });
}
