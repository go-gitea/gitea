export function initGiteaAppPermSettingConfig() {
  const $form = document.querySelector<HTMLFormElement>('form#gita-app-create');
  if ($form === null) {
    return
  }

  const $permListInput = $form.querySelector<HTMLInputElement>('input[name="perm_list"]');
  if ($permListInput === null) {
    return
  }

  for (const el of $form.querySelectorAll<HTMLSelectElement>('select.gitea-app-perm-item')) {
    el.addEventListener('change', (e : Event) => {
        if (!(e.target instanceof HTMLSelectElement)) {
            return;
        }

        let $permList = $permListInput.value === ""? []: $permListInput.value.split(',');
        const $permItem = e.target.getAttribute('data-perm-item');
        if ($permItem === null) {
            return;
        }

        const $value = e.target.value;
        if ($value === null || $value === "none") {
            $permList = $permList.filter(item => !item.startsWith($permItem + ':'));
        }
        else {
            const newPerm = $permItem + ':' + $value;
            const foundIndex = $permList.findIndex(item => item.startsWith($permItem + ':'));
            if (foundIndex !== -1) {
                $permList[foundIndex] = newPerm;
            } else {
                $permList.push(newPerm);
            }
        }

        $permListInput.value = $permList.join(',');
    });
  }
}
