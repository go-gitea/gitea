const {csrf} = window.config;

export function initCompReactionSelector(parent) {
  let reactions = '';
  if (!parent) {
    parent = $(document);
    reactions = '.reactions > ';
  }

  parent.find(`${reactions}a.label`).popup({position: 'bottom left', metadata: {content: 'title', title: 'none'}});

  parent.find(`.select-reaction > .menu > .item, ${reactions}a.label`).on('click', function (e) {
    e.preventDefault();

    if ($(this).hasClass('disabled')) return;

    const actionURL = $(this).hasClass('item') ? $(this).closest('.select-reaction').data('action-url') : $(this).data('action-url');
    const url = `${actionURL}/${$(this).hasClass('blue') ? 'unreact' : 'react'}`;
    $.ajax({
      type: 'POST',
      url,
      data: {
        _csrf: csrf,
        content: $(this).data('content')
      }
    }).done((resp) => {
      if (resp && (resp.html || resp.empty)) {
        const content = $(this).closest('.content');
        let react = content.find('.segment.reactions');
        if ((!resp.empty || resp.html === '') && react.length > 0) {
          react.remove();
        }
        if (!resp.empty) {
          react = $('<div class="ui attached segment reactions"></div>');
          const attachments = content.find('.segment.bottom:first');
          if (attachments.length > 0) {
            react.insertBefore(attachments);
          } else {
            react.appendTo(content);
          }
          react.html(resp.html);
          react.find('.dropdown').dropdown();
          initCompReactionSelector(react);
        }
      }
    });
  });
}
