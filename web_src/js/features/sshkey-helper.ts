export function initSshKeyFormParser() {
  // Parse SSH Key
  document.querySelector<HTMLTextAreaElement>('#ssh-key-content')?.addEventListener('input', function () {
    const arrays = this.value.split(' ');
    const title = document.querySelector<HTMLInputElement>('#ssh-key-title');
    if (!title.value && arrays.length === 3 && arrays[2] !== '') {
      title.value = arrays[2];
    }
  });
}
