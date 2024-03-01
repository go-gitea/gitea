export function initSshKeyFormParser() {
  // Parse SSH Key
  document.getElementById('ssh-key-content')?.addEventListener('input', function () {
    const arrays = this.value.split(' ');
    const title = document.getElementById('ssh-key-title');
    if (!title.value && arrays.length === 3 && arrays[2] !== '') {
      title.value = arrays[2];
    }
  });
}
