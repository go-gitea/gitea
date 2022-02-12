import $ from 'jquery';
import 'jquery.are-you-sure';
import {mqBinarySearch} from '../utils.js';
import createDropzone from './dropzone.js';
import {initCompColorPicker} from './comp/ColorPicker.js';

const {csrfToken} = window.config;

export function initGlobalFormDirtyLeaveConfirm() {
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if ($('.user.signin').length === 0) {
    $('form:not(.ignore-dirty)').areYouSure();
  }
}

export function initHeadNavbarContentToggle() {
  const content = $('#navbar');
  const toggle = $('#navbar-expand-toggle');
  let isExpanded = false;
  toggle.on('click', () => {
    isExpanded = !isExpanded;
    if (isExpanded) {
      content.addClass('shown');
      toggle.addClass('active');
    } else {
      content.removeClass('shown');
      toggle.removeClass('active');
    }
  });
}

export function initFootLanguageMenu() {
  function linkLanguageAction() {
    const $this = $(this);
    $.post($this.data('url')).always(() => {
      window.location.reload();
    });
  }

  $('.language-menu a[lang]').on('click', linkLanguageAction);
}


export function initGlobalEnterQuickSubmit() {
  $('.js-quick-submit').on('keydown', function (e) {
    if (((e.ctrlKey && !e.altKey) || e.metaKey) && (e.keyCode === 13 || e.keyCode === 10)) {
      $(this).closest('form').trigger('submit');
    }
  });
}

export function initGlobalButtonClickOnEnter() {
  $(document).on('keypress', '.ui.button', (e) => {
    if (e.keyCode === 13 || e.keyCode === 32) { // enter key or space bar
      $(e.target).trigger('click');
    }
  });
}

export function initGlobalCommon() {
  // Show exact time
  $('.time-since').each(function () {
    $(this)
      .addClass('tooltip')
      .attr('data-content', $(this).attr('title'))
      .attr('title', '');
  });

  // Undo Safari emoji glitch fix at high enough zoom levels
  if (navigator.userAgent.match('Safari')) {
    $(window).resize(() => {
      const px = mqBinarySearch('width', 0, 4096, 1, 'px');
      const em = mqBinarySearch('width', 0, 1024, 0.01, 'em');
      if (em * 16 * 1.25 - px <= -1) {
        $('body').addClass('safari-above125');
      } else {
        $('body').removeClass('safari-above125');
      }
    });
  }

  // Semantic UI modules.
  $('.dropdown:not(.custom)').dropdown({
    fullTextSearch: 'exact'
  });
  $('.jump.dropdown').dropdown({
    action: 'hide',
    onShow() {
      $('.tooltip').popup('hide');
    },
    fullTextSearch: 'exact'
  });
  $('.slide.up.dropdown').dropdown({
    transition: 'slide up',
    fullTextSearch: 'exact'
  });
  $('.upward.dropdown').dropdown({
    direction: 'upward',
    fullTextSearch: 'exact'
  });
  $('.ui.checkbox').checkbox();
  $('.ui.progress').progress({
    showActivity: false
  });

  // init popups
  $('.tooltip').each((_, el) => {
    const $el = $(el);
    const attr = $el.attr('data-variation');
    const attrs = attr ? attr.split(' ') : [];
    const variations = new Set([...attrs, 'inverted', 'tiny']);
    $el.attr('data-variation', [...variations].join(' ')).popup();
  });

  $('.top.menu .tooltip').popup({
    onShow() {
      if ($('.top.menu .menu.transition').hasClass('visible')) {
        return false;
      }
    }
  });
  $('.tabular.menu .item').tab();
  $('.tabable.menu .item').tab();

  $('.toggle.button').on('click', function () {
    $($(this).data('target')).slideToggle(100);
  });

  // make table <tr> and <td> elements clickable like a link
  $('tr[data-href], td[data-href]').on('click', function (e) {
    const href = $(this).data('href');
    if (e.target.nodeName === 'A') {
      // if a user clicks on <a>, then the <tr> or <td> should not act as a link.
      return;
    }
    if (e.ctrlKey || e.metaKey) {
      // ctrl+click or meta+click opens a new window in modern browsers
      window.open(href);
    } else {
      window.location = href;
    }
  });
}

export function initGlobalDropzone() {
  // Dropzone
  for (const el of document.querySelectorAll('.dropzone')) {
    const $dropzone = $(el);
    const _promise = createDropzone(el, {
      url: $dropzone.data('upload-url'),
      headers: {'X-Csrf-Token': csrfToken},
      maxFiles: $dropzone.data('max-file'),
      maxFilesize: $dropzone.data('max-size'),
      acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
      addRemoveLinks: true,
      dictDefaultMessage: $dropzone.data('default-message'),
      dictInvalidFileType: $dropzone.data('invalid-input-type'),
      dictFileTooBig: $dropzone.data('file-too-big'),
      dictRemoveFile: $dropzone.data('remove-file'),
      timeout: 0,
      thumbnailMethod: 'contain',
      thumbnailWidth: 480,
      thumbnailHeight: 480,
      init() {
        this.on('success', (_file, data) => {
          const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
          $dropzone.find('.files').append(input);
        });
        this.on('removedfile', (file) => {
          $(`#${file.uuid}`).remove();
          if ($dropzone.data('remove-url')) {
            $.post($dropzone.data('remove-url'), {
              file: file.uuid,
              _csrf: csrfToken,
            });
          }
        });
      },
    });
  }
}

export function initGlobalLinkActions() {
  function showDeletePopup() {
    const $this = $(this);
    const dataArray = $this.data();
    let filter = '';
    if ($this.data('modal-id')) {
      filter += `#${$this.data('modal-id')}`;
    }

    const dialog = $(`.delete.modal${filter}`);
    dialog.find('.name').text($this.data('name'));
    for (const [key, value] of Object.entries(dataArray)) {
      if (key && key.startsWith('data')) {
        dialog.find(`.${key}`).text(value);
      }
    }

    dialog.modal({
      closable: false,
      onApprove() {
        if ($this.data('type') === 'form') {
          $($this.data('form')).trigger('submit');
          return;
        }

        const postData = {
          _csrf: csrfToken,
        };
        for (const [key, value] of Object.entries(dataArray)) {
          if (key && key.startsWith('data')) {
            postData[key.substr(4)] = value;
          }
          if (key === 'id') {
            postData['id'] = value;
          }
        }

        $.post($this.data('url'), postData).done((data) => {
          window.location.href = data.redirect;
        });
      }
    }).modal('show');
    return false;
  }

  function showAddAllPopup() {
    const $this = $(this);
    let filter = '';
    if ($this.attr('id')) {
      filter += `#${$this.attr('id')}`;
    }

    const dialog = $(`.addall.modal${filter}`);
    dialog.find('.name').text($this.data('name'));

    dialog.modal({
      closable: false,
      onApprove() {
        if ($this.data('type') === 'form') {
          $($this.data('form')).trigger('submit');
          return;
        }

        $.post($this.data('url'), {
          _csrf: csrfToken,
          id: $this.data('id')
        }).done((data) => {
          window.location.href = data.redirect;
        });
      }
    }).modal('show');
    return false;
  }

  function linkAction(e) {
    e.preventDefault();
    const $this = $(this);
    const redirect = $this.data('redirect');
    $.post($this.data('url'), {
      _csrf: csrfToken
    }).done((data) => {
      if (data.redirect) {
        window.location.href = data.redirect;
      } else if (redirect) {
        window.location.href = redirect;
      } else {
        window.location.reload();
      }
    });
  }

  // Helpers.
  $('.delete-button').on('click', showDeletePopup);
  $('.link-action').on('click', linkAction);

  // FIXME: this function is only used once, and not common, not well designed. should be refactored later
  $('.add-all-button').on('click', showAddAllPopup);

  // FIXME: this is only used once, and should be replace with `link-action` instead
  $('.undo-button').on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrfToken,
      id: $this.data('id')
    }).done((data) => {
      window.location.href = data.redirect;
    });
  });
}

export function initGlobalButtons() {
  $('.show-panel.button').on('click', function () {
    $($(this).data('panel')).show();
  });

  $('.hide-panel.button').on('click', function (event) {
    // a `.hide-panel.button` can hide a panel, by `data-panel="selector"` or `data-panel-closest="selector"`
    event.preventDefault();
    let sel = $(this).attr('data-panel');
    if (sel) {
      $(sel).hide();
      return;
    }
    sel = $(this).attr('data-panel-closest');
    if (sel) {
      $(this).closest(sel).hide();
      return;
    }
    // should never happen, otherwise there is a bug in code
    alert('Nothing to hide');
  });

  $('.show-modal').on('click', function () {
    const modalDiv = $($(this).attr('data-modal'));
    for (const attrib of this.attributes) {
      if (!attrib.name.startsWith('data-modal-')) {
        continue;
      }
      const id = attrib.name.substring(11);
      const target = modalDiv.find(`#${id}`);
      if (target.is('input')) {
        target.val(attrib.value);
      } else {
        target.text(attrib.value);
      }
    }
    modalDiv.modal('show');
    const colorPickers = $($(this).attr('data-modal')).find('.color-picker');
    if (colorPickers.length > 0) {
      initCompColorPicker();
    }
  });

  $('.delete-post.button').on('click', function () {
    const $this = $(this);
    $.post($this.attr('data-request-url'), {
      _csrf: csrfToken
    }).done(() => {
      window.location.href = $this.attr('data-done-url');
    });
  });
}
