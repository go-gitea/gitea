export default async function initProject(csrf) {
  if (!window.config || !window.config.PageIsProjects) {
    return;
  }

  const {Sortable} = await import(
    /* webpackChunkName: "sortable" */ 'sortablejs'
  );

  const boardColumns = document.getElementsByClassName('board-column');

  for (let i = 0; i < boardColumns.length; i++) {
    new Sortable(
      document.getElementById(
        boardColumns[i].getElementsByClassName('board')[0].id
      ),
      {
        group: 'shared',
        animation: 150,
        onAdd: (e) => {
          $.ajax(`${e.to.dataset.url}/${e.item.dataset.issue}`, {
            headers: {
              'X-Csrf-Token': csrf,
              'X-Remote': true,
            },
            contentType: 'application/json',
            type: 'POST',
            success: () => {
              // setTimeout(reload(),3000)
            },
          });
        },
      }
    );
  }
}
