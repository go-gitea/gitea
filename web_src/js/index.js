/* exported timeAddManual, toggleStopwatch, cancelStopwatch */
/* exported toggleDeadlineForm, setDeadline, updateDeadline, deleteDependencyModal, cancelCodeComment, onOAuthLoginClick */

import './publicpath.js';

import Vue from 'vue';
import {htmlEscape} from 'escape-goat';
import 'jquery.are-you-sure';
import './vendor/semanticdropdown.js';

import initMigration from './features/migration.js';
import initContextPopups from './features/contextpopup.js';
import initGitGraph from './features/gitgraph.js';
import initClipboard from './features/clipboard.js';
import initUserHeatmap from './features/userheatmap.js';
import initProject from './features/projects.js';
import initServiceWorker from './features/serviceworker.js';
import initMarkdownAnchors from './markdown/anchors.js';
import renderMarkdownContent from './markdown/content.js';
import attachTribute from './features/tribute.js';
import initTableSort from './features/tablesort.js';
import ActivityTopAuthors from './components/ActivityTopAuthors.vue';
import {initNotificationsTable, initNotificationCount} from './features/notification.js';
import {createCodeEditor, createMonaco} from './features/codeeditor.js';
import {svg, svgs} from './svg.js';
import {stripTags} from './utils.js';
import createDropzone from './features/dropzone.js';
import {initCommentForm, updateIssuesMeta, initRepoStatusChecker, initIssueList, reload, initRepository, setCommentSimpleMDE, initLabelEdit, initCommentPreviewTab, initEditPreviewTab, buttonsClickOnEnter, initEditor} from './features/issuesutil.js';

const {AppSubUrl, StaticUrlPrefix, csrf} = window.config;

// Silence fomantic's error logging when tabs are used without a target content element
$.fn.tab.settings.silent = true;


function initEditDiffTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  $tabMenu.find(`.item[data-tab="${$tabMenu.data('diff')}"]`).on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrf,
      context: $this.data('context'),
      content: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val()
    }, (data) => {
      const $diffPreviewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('diff')}"]`);
      $diffPreviewPanel.html(data);
    });
  });
}

function initEditForm() {
  if ($('.edit.form').length === 0) {
    return;
  }

  initEditPreviewTab($('.edit.form'));
  initEditDiffTab($('.edit.form'));
}

function initInstall() {
  if ($('.install').length === 0) {
    return;
  }

  if ($('#db_host').val() === '') {
    $('#db_host').val('127.0.0.1:3306');
    $('#db_user').val('gitea');
    $('#db_name').val('gitea');
  }

  // Database type change detection.
  $('#db_type').on('change', function () {
    const sqliteDefault = 'data/gitea.db';
    const tidbDefault = 'data/gitea_tidb';

    const dbType = $(this).val();
    if (dbType === 'SQLite3') {
      $('#sql_settings').hide();
      $('#pgsql_settings').hide();
      $('#mysql_settings').hide();
      $('#sqlite_settings').show();

      if (dbType === 'SQLite3' && $('#db_path').val() === tidbDefault) {
        $('#db_path').val(sqliteDefault);
      }
      return;
    }

    const dbDefaults = {
      MySQL: '127.0.0.1:3306',
      PostgreSQL: '127.0.0.1:5432',
      MSSQL: '127.0.0.1:1433'
    };

    $('#sqlite_settings').hide();
    $('#sql_settings').show();

    $('#pgsql_settings').toggle(dbType === 'PostgreSQL');
    $('#mysql_settings').toggle(dbType === 'MySQL');
    $.each(dbDefaults, (_type, defaultHost) => {
      if ($('#db_host').val() === defaultHost) {
        $('#db_host').val(dbDefaults[dbType]);
        return false;
      }
    });
  });

  // TODO: better handling of exclusive relations.
  $('#offline-mode input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-gravatar').checkbox('check');
      $('#federated-avatar-lookup').checkbox('uncheck');
    }
  });
  $('#disable-gravatar input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#federated-avatar-lookup').checkbox('uncheck');
    } else {
      $('#offline-mode').checkbox('uncheck');
    }
  });
  $('#federated-avatar-lookup input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-gravatar').checkbox('uncheck');
      $('#offline-mode').checkbox('uncheck');
    }
  });
  $('#enable-openid-signin input').on('change', function () {
    if ($(this).is(':checked')) {
      if (!$('#disable-registration input').is(':checked')) {
        $('#enable-openid-signup').checkbox('check');
      }
    } else {
      $('#enable-openid-signup').checkbox('uncheck');
    }
  });
  $('#disable-registration input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#enable-captcha').checkbox('uncheck');
      $('#enable-openid-signup').checkbox('uncheck');
    } else {
      $('#enable-openid-signup').checkbox('check');
    }
  });
  $('#enable-captcha input').on('change', function () {
    if ($(this).is(':checked')) {
      $('#disable-registration').checkbox('uncheck');
    }
  });
}

function getArchive($target, url, first) {
  $.ajax({
    url,
    type: 'POST',
    data: {
      _csrf: csrf,
    },
    complete(xhr) {
      if (xhr.status === 200) {
        if (!xhr.responseJSON) {
          // XXX Shouldn't happen?
          $target.closest('.dropdown').children('i').removeClass('loading');
          return;
        }

        if (!xhr.responseJSON.complete) {
          $target.closest('.dropdown').children('i').addClass('loading');
          // Wait for only three quarters of a second initially, in case it's
          // quickly archived.
          setTimeout(() => {
            getArchive($target, url, false);
          }, first ? 750 : 2000);
        } else {
          // We don't need to continue checking.
          $target.closest('.dropdown').children('i').removeClass('loading');
          window.location.href = url;
        }
      }
    }
  });
}

function initArchiveLinks() {
  if ($('.archive-link').length === 0) {
    return;
  }

  $('.archive-link').on('click', function (event) {
    const url = $(this).data('url');
    if (typeof url === 'undefined') {
      return;
    }

    event.preventDefault();
    getArchive($(event.target), url, true);
  });
}

function initPullRequestReview() {
  if (window.location.hash && window.location.hash.startsWith('#issuecomment-')) {
    const commentDiv = $(window.location.hash);
    if (commentDiv) {
      // get the name of the parent id
      const groupID = commentDiv.closest('div[id^="code-comments-"]').attr('id');
      if (groupID && groupID.startsWith('code-comments-')) {
        const id = groupID.substr(14);
        $(`#show-outdated-${id}`).addClass('hide');
        $(`#code-comments-${id}`).removeClass('hide');
        $(`#code-preview-${id}`).removeClass('hide');
        $(`#hide-outdated-${id}`).removeClass('hide');
        $(window).scrollTop(commentDiv.offset().top);
      }
    }
  }

  $('.show-outdated').on('click', function (e) {
    e.preventDefault();
    const id = $(this).data('comment');
    $(this).addClass('hide');
    $(`#code-comments-${id}`).removeClass('hide');
    $(`#code-preview-${id}`).removeClass('hide');
    $(`#hide-outdated-${id}`).removeClass('hide');
  });

  $('.hide-outdated').on('click', function (e) {
    e.preventDefault();
    const id = $(this).data('comment');
    $(this).addClass('hide');
    $(`#code-comments-${id}`).addClass('hide');
    $(`#code-preview-${id}`).addClass('hide');
    $(`#show-outdated-${id}`).removeClass('hide');
  });

  $('button.comment-form-reply').on('click', function (e) {
    e.preventDefault();
    $(this).hide();
    const form = $(this).parent().find('.comment-form');
    form.removeClass('hide');
    const $textarea = form.find('textarea');
    let $simplemde;
    if ($textarea.data('simplemde')) {
      $simplemde = $textarea.data('simplemde');
    } else {
      attachTribute($textarea.get(), {mentions: true, emoji: true});
      $simplemde = setCommentSimpleMDE($textarea);
      $textarea.data('simplemde', $simplemde);
    }
    $textarea.focus();
    $simplemde.codemirror.focus();
    assingMenuAttributes(form.find('.menu'));
  });
  // The following part is only for diff views
  if ($('.repository.pull.diff').length === 0) {
    return;
  }

  $('.btn-review').on('click', function (e) {
    e.preventDefault();
    $(this).closest('.dropdown').find('.menu').toggle('visible');
  }).closest('.dropdown').find('.link.close')
    .on('click', function (e) {
      e.preventDefault();
      $(this).closest('.menu').toggle('visible');
    });

  $('.add-code-comment').on('click', function (e) {
    if ($(e.target).hasClass('btn-add-single')) return; // https://github.com/go-gitea/gitea/issues/4745
    e.preventDefault();

    const isSplit = $(this).closest('.code-diff').hasClass('code-diff-split');
    const side = $(this).data('side');
    const idx = $(this).data('idx');
    const path = $(this).data('path');
    const form = $('#pull_review_add_comment').html();
    const tr = $(this).closest('tr');

    const oldLineNum = tr.find('.lines-num-old').data('line-num');
    const newLineNum = tr.find('.lines-num-new').data('line-num');
    const addCommentKey = `${oldLineNum}|${newLineNum}`;
    if (document.querySelector(`[data-add-comment-key="${addCommentKey}"]`)) return; // don't add same comment box twice

    let ntr = tr.next();
    if (!ntr.hasClass('add-comment')) {
      ntr = $(`
        <tr class="add-comment" data-add-comment-key="${addCommentKey}">
          ${isSplit ? `
            <td class="lines-num"></td>
            <td class="lines-type-marker"></td>
            <td class="add-comment-left"></td>
            <td class="lines-num"></td>
            <td class="lines-type-marker"></td>
            <td class="add-comment-right"></td>
          ` : `
            <td class="lines-num"></td>
            <td class="lines-num"></td>
            <td class="add-comment-left add-comment-right" colspan="2"></td>
          `}
        </tr>`);
      tr.after(ntr);
    }

    const td = ntr.find(`.add-comment-${side}`);
    let commentCloud = td.find('.comment-code-cloud');
    if (commentCloud.length === 0) {
      td.html(form);
      commentCloud = td.find('.comment-code-cloud');
      assingMenuAttributes(commentCloud.find('.menu'));

      td.find("input[name='line']").val(idx);
      td.find("input[name='side']").val(side === 'left' ? 'previous' : 'proposed');
      td.find("input[name='path']").val(path);
    }
    const $textarea = commentCloud.find('textarea');
    attachTribute($textarea.get(), {mentions: true, emoji: true});

    const $simplemde = setCommentSimpleMDE($textarea);
    $textarea.focus();
    $simplemde.codemirror.focus();
  });
}

function assingMenuAttributes(menu) {
  const id = Math.floor(Math.random() * Math.floor(1000000));
  menu.attr('data-write', menu.attr('data-write') + id);
  menu.attr('data-preview', menu.attr('data-preview') + id);
  menu.find('.item').each(function () {
    const tab = $(this).attr('data-tab') + id;
    $(this).attr('data-tab', tab);
  });
  menu.parent().find("*[data-tab='write']").attr('data-tab', `write${id}`);
  menu.parent().find("*[data-tab='preview']").attr('data-tab', `preview${id}`);
  initCommentPreviewTab(menu.parent('.form'));
  return id;
}

function initRepositoryCollaboration() {
  // Change collaborator access mode
  $('.access-mode.menu .item').on('click', function () {
    const $menu = $(this).parent();
    $.post($menu.data('url'), {
      _csrf: csrf,
      uid: $menu.data('uid'),
      mode: $(this).data('value')
    });
  });
}

function initTeamSettings() {
  // Change team access mode
  $('.organization.new.team input[name=permission]').on('change', () => {
    const val = $('input[name=permission]:checked', '.organization.new.team').val();
    if (val === 'admin') {
      $('.organization.new.team .team-units').hide();
    } else {
      $('.organization.new.team .team-units').show();
    }
  });
}

function initWikiForm() {
  const $editArea = $('.repository.wiki textarea#edit_area');
  let sideBySideChanges = 0;
  let sideBySideTimeout = null;
  if ($editArea.length > 0) {
    const simplemde = new SimpleMDE({
      autoDownloadFontAwesome: false,
      element: $editArea[0],
      forceSync: true,
      previewRender(plainText, preview) { // Async method
        // FIXME: still send render request when return back to edit mode
        const render = function () {
          sideBySideChanges = 0;
          if (sideBySideTimeout !== null) {
            clearTimeout(sideBySideTimeout);
            sideBySideTimeout = null;
          }
          $.post($editArea.data('url'), {
            _csrf: csrf,
            mode: 'gfm',
            context: $editArea.data('context'),
            text: plainText,
            wiki: true
          }, (data) => {
            preview.innerHTML = `<div class="markdown ui segment">${data}</div>`;
            renderMarkdownContent();
          });
        };

        setTimeout(() => {
          if (!simplemde.isSideBySideActive()) {
            render();
          } else {
            // delay preview by keystroke counting
            sideBySideChanges++;
            if (sideBySideChanges > 10) {
              render();
            }
            // or delay preview by timeout
            if (sideBySideTimeout !== null) {
              clearTimeout(sideBySideTimeout);
              sideBySideTimeout = null;
            }
            sideBySideTimeout = setTimeout(render, 600);
          }
        }, 0);
        if (!simplemde.isSideBySideActive()) {
          return 'Loading...';
        }
        return preview.innerHTML;
      },
      renderingConfig: {
        singleLineBreaks: false
      },
      indentWithTabs: false,
      tabSize: 4,
      spellChecker: false,
      toolbar: ['bold', 'italic', 'strikethrough', '|',
        'heading-1', 'heading-2', 'heading-3', 'heading-bigger', 'heading-smaller', '|',
        {
          name: 'code-inline',
          action(e) {
            const cm = e.codemirror;
            const selection = cm.getSelection();
            cm.replaceSelection(`\`${selection}\``);
            if (!selection) {
              const cursorPos = cm.getCursor();
              cm.setCursor(cursorPos.line, cursorPos.ch - 1);
            }
            cm.focus();
          },
          className: 'fa fa-angle-right',
          title: 'Add Inline Code',
        }, 'code', 'quote', '|', {
          name: 'checkbox-empty',
          action(e) {
            const cm = e.codemirror;
            cm.replaceSelection(`\n- [ ] ${cm.getSelection()}`);
            cm.focus();
          },
          className: 'fa fa-square-o',
          title: 'Add Checkbox (empty)',
        },
        {
          name: 'checkbox-checked',
          action(e) {
            const cm = e.codemirror;
            cm.replaceSelection(`\n- [x] ${cm.getSelection()}`);
            cm.focus();
          },
          className: 'fa fa-check-square-o',
          title: 'Add Checkbox (checked)',
        }, '|',
        'unordered-list', 'ordered-list', '|',
        'link', 'image', 'table', 'horizontal-rule', '|',
        'clean-block', 'preview', 'fullscreen', 'side-by-side', '|',
        {
          name: 'revert-to-textarea',
          action(e) {
            e.toTextArea();
          },
          className: 'fa fa-file',
          title: 'Revert to simple textarea',
        },
      ]
    });
    $(simplemde.codemirror.getInputField()).addClass('js-quick-submit');

    setTimeout(() => {
      const $bEdit = $('.repository.wiki.new .previewtabs a[data-tab="write"]');
      const $bPrev = $('.repository.wiki.new .previewtabs a[data-tab="preview"]');
      const $toolbar = $('.editor-toolbar');
      const $bPreview = $('.editor-toolbar a.fa-eye');
      const $bSideBySide = $('.editor-toolbar a.fa-columns');
      $bEdit.on('click', () => {
        if ($toolbar.hasClass('disabled-for-preview')) {
          $bPreview.trigger('click');
        }
      });
      $bPrev.on('click', () => {
        if (!$toolbar.hasClass('disabled-for-preview')) {
          $bPreview.trigger('click');
        }
      });
      $bPreview.on('click', () => {
        setTimeout(() => {
          if ($toolbar.hasClass('disabled-for-preview')) {
            if ($bEdit.hasClass('active')) {
              $bEdit.removeClass('active');
            }
            if (!$bPrev.hasClass('active')) {
              $bPrev.addClass('active');
            }
          } else {
            if (!$bEdit.hasClass('active')) {
              $bEdit.addClass('active');
            }
            if ($bPrev.hasClass('active')) {
              $bPrev.removeClass('active');
            }
          }
        }, 0);
      });
      $bSideBySide.on('click', () => {
        sideBySideChanges = 10;
      });
    }, 0);
  }
}

// Adding function to get the cursor position in a text field to jQuery object.
$.fn.getCursorPosition = function () {
  const el = $(this).get(0);
  let pos = 0;
  if ('selectionStart' in el) {
    pos = el.selectionStart;
  } else if ('selection' in document) {
    el.focus();
    const Sel = document.selection.createRange();
    const SelLength = document.selection.createRange().text.length;
    Sel.moveStart('character', -el.value.length);
    pos = Sel.text.length - SelLength;
  }
  return pos;
};

function initOrganization() {
  if ($('.organization').length === 0) {
    return;
  }

  // Options
  if ($('.organization.settings.options').length > 0) {
    $('#org_name').on('keyup', function () {
      const $prompt = $('#org-name-change-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('org-name').toString().toLowerCase()) {
        $prompt.show();
      } else {
        $prompt.hide();
      }
    });
  }

  // Labels
  if ($('.organization.settings.labels').length > 0) {
    initLabelEdit();
  }
}

function initUserSettings() {
  // Options
  if ($('.user.settings.profile').length > 0) {
    $('#username').on('keyup', function () {
      const $prompt = $('#name-change-prompt');
      if ($(this).val().toString().toLowerCase() !== $(this).data('name').toString().toLowerCase()) {
        $prompt.show();
      } else {
        $prompt.hide();
      }
    });
  }
}

async function initGithook() {
  if ($('.edit.githook').length === 0) return;
  const filename = document.querySelector('.hook-filename').textContent;
  await createMonaco($('#content')[0], filename, {language: 'shell'});
}

function initWebhook() {
  if ($('.new.webhook').length === 0) {
    return;
  }

  $('.events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      $('.events.fields').show();
    }
  });
  $('.non-events.checkbox input').on('change', function () {
    if ($(this).is(':checked')) {
      $('.events.fields').hide();
    }
  });

  const updateContentType = function () {
    const visible = $('#http_method').val() === 'POST';
    $('#content_type').parent().parent()[visible ? 'show' : 'hide']();
  };
  updateContentType();
  $('#http_method').on('change', () => {
    updateContentType();
  });

  // Test delivery
  $('#test-delivery').on('click', function () {
    const $this = $(this);
    $this.addClass('loading disabled');
    $.post($this.data('link'), {
      _csrf: csrf
    }).done(
      setTimeout(() => {
        window.location.href = $this.data('redirect');
      }, 5000)
    );
  });
}

function initAdmin() {
  if ($('.admin').length === 0) {
    return;
  }

  // New user
  if ($('.admin.new.user').length > 0 || $('.admin.edit.user').length > 0) {
    $('#login_type').on('change', function () {
      if ($(this).val().substring(0, 1) === '0') {
        $('#login_name').removeAttr('required');
        $('.non-local').hide();
        $('.local').show();
        $('#user_name').focus();

        if ($(this).data('password') === 'required') {
          $('#password').attr('required', 'required');
        }
      } else {
        $('#login_name').attr('required', 'required');
        $('.non-local').show();
        $('.local').hide();
        $('#login_name').focus();

        $('#password').removeAttr('required');
      }
    });
  }

  function onSecurityProtocolChange() {
    if ($('#security_protocol').val() > 0) {
      $('.has-tls').show();
    } else {
      $('.has-tls').hide();
    }
  }

  function onUsePagedSearchChange() {
    if ($('#use_paged_search').prop('checked')) {
      $('.search-page-size').show()
        .find('input').attr('required', 'required');
    } else {
      $('.search-page-size').hide()
        .find('input').removeAttr('required');
    }
  }

  function onOAuth2Change() {
    $('.open_id_connect_auto_discovery_url, .oauth2_use_custom_url').hide();
    $('.open_id_connect_auto_discovery_url input[required]').removeAttr('required');

    const provider = $('#oauth2_provider').val();
    switch (provider) {
      case 'github':
      case 'gitlab':
      case 'gitea':
      case 'nextcloud':
      case 'mastodon':
        $('.oauth2_use_custom_url').show();
        break;
      case 'openidConnect':
        $('.open_id_connect_auto_discovery_url input').attr('required', 'required');
        $('.open_id_connect_auto_discovery_url').show();
        break;
    }
    onOAuth2UseCustomURLChange();
  }

  function onOAuth2UseCustomURLChange() {
    const provider = $('#oauth2_provider').val();
    $('.oauth2_use_custom_url_field').hide();
    $('.oauth2_use_custom_url_field input[required]').removeAttr('required');

    if ($('#oauth2_use_custom_url').is(':checked')) {
      $('#oauth2_token_url').val($(`#${provider}_token_url`).val());
      $('#oauth2_auth_url').val($(`#${provider}_auth_url`).val());
      $('#oauth2_profile_url').val($(`#${provider}_profile_url`).val());
      $('#oauth2_email_url').val($(`#${provider}_email_url`).val());

      switch (provider) {
        case 'github':
          $('.oauth2_token_url input, .oauth2_auth_url input, .oauth2_profile_url input, .oauth2_email_url input').attr('required', 'required');
          $('.oauth2_token_url, .oauth2_auth_url, .oauth2_profile_url, .oauth2_email_url').show();
          break;
        case 'nextcloud':
        case 'gitea':
        case 'gitlab':
          $('.oauth2_token_url input, .oauth2_auth_url input, .oauth2_profile_url input').attr('required', 'required');
          $('.oauth2_token_url, .oauth2_auth_url, .oauth2_profile_url').show();
          $('#oauth2_email_url').val('');
          break;
        case 'mastodon':
          $('.oauth2_auth_url input').attr('required', 'required');
          $('.oauth2_auth_url').show();
          break;
      }
    }
  }

  function onVerifyGroupMembershipChange() {
    if ($('#groups_enabled').is(':checked')) {
      $('#groups_enabled_change').show();
    } else {
      $('#groups_enabled_change').hide();
    }
  }

  // New authentication
  if ($('.admin.new.authentication').length > 0) {
    $('#auth_type').on('change', function () {
      $('.ldap, .dldap, .smtp, .pam, .oauth2, .has-tls, .search-page-size, .sspi').hide();

      $('.ldap input[required], .binddnrequired input[required], .dldap input[required], .smtp input[required], .pam input[required], .oauth2 input[required], .has-tls input[required], .sspi input[required]').removeAttr('required');
      $('.binddnrequired').removeClass('required');

      const authType = $(this).val();
      switch (authType) {
        case '2': // LDAP
          $('.ldap').show();
          $('.binddnrequired input, .ldap div.required:not(.dldap) input').attr('required', 'required');
          $('.binddnrequired').addClass('required');
          break;
        case '3': // SMTP
          $('.smtp').show();
          $('.has-tls').show();
          $('.smtp div.required input, .has-tls').attr('required', 'required');
          break;
        case '4': // PAM
          $('.pam').show();
          $('.pam input').attr('required', 'required');
          break;
        case '5': // LDAP
          $('.dldap').show();
          $('.dldap div.required:not(.ldap) input').attr('required', 'required');
          break;
        case '6': // OAuth2
          $('.oauth2').show();
          $('.oauth2 div.required:not(.oauth2_use_custom_url,.oauth2_use_custom_url_field,.open_id_connect_auto_discovery_url) input').attr('required', 'required');
          onOAuth2Change();
          break;
        case '7': // SSPI
          $('.sspi').show();
          $('.sspi div.required input').attr('required', 'required');
          break;
      }
      if (authType === '2' || authType === '5') {
        onSecurityProtocolChange();
        onVerifyGroupMembershipChange();
      }
      if (authType === '2') {
        onUsePagedSearchChange();
      }
    });
    $('#auth_type').trigger('change');
    $('#security_protocol').on('change', onSecurityProtocolChange);
    $('#use_paged_search').on('change', onUsePagedSearchChange);
    $('#oauth2_provider').on('change', onOAuth2Change);
    $('#oauth2_use_custom_url').on('change', onOAuth2UseCustomURLChange);
    $('#groups_enabled').on('change', onVerifyGroupMembershipChange);
  }
  // Edit authentication
  if ($('.admin.edit.authentication').length > 0) {
    const authType = $('#auth_type').val();
    if (authType === '2' || authType === '5') {
      $('#security_protocol').on('change', onSecurityProtocolChange);
      $('#groups_enabled').on('change', onVerifyGroupMembershipChange);
      onVerifyGroupMembershipChange();
      if (authType === '2') {
        $('#use_paged_search').on('change', onUsePagedSearchChange);
      }
    } else if (authType === '6') {
      $('#oauth2_provider').on('change', onOAuth2Change);
      $('#oauth2_use_custom_url').on('change', onOAuth2UseCustomURLChange);
      onOAuth2Change();
    }
  }

  // Notice
  if ($('.admin.notice')) {
    const $detailModal = $('#detail-modal');

    // Attach view detail modals
    $('.view-detail').on('click', function () {
      $detailModal.find('.content pre').text($(this).parents('tr').find('.notice-description').text());
      $detailModal.find('.sub.header').text($(this).parents('tr').find('.notice-created-time').text());
      $detailModal.modal('show');
      return false;
    });

    // Select actions
    const $checkboxes = $('.select.table .ui.checkbox');
    $('.select.action').on('click', function () {
      switch ($(this).data('action')) {
        case 'select-all':
          $checkboxes.checkbox('check');
          break;
        case 'deselect-all':
          $checkboxes.checkbox('uncheck');
          break;
        case 'inverse':
          $checkboxes.checkbox('toggle');
          break;
      }
    });
    $('#delete-selection').on('click', function () {
      const $this = $(this);
      $this.addClass('loading disabled');
      const ids = [];
      $checkboxes.each(function () {
        if ($(this).checkbox('is checked')) {
          ids.push($(this).data('id'));
        }
      });
      $.post($this.data('link'), {
        _csrf: csrf,
        ids
      }).done(() => {
        window.location.href = $this.data('redirect');
      });
    });
  }
}

function searchUsers() {
  const $searchUserBox = $('#search-user-box');
  $searchUserBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${AppSubUrl}/api/v1/users/search?q={query}`,
      onResponse(response) {
        const items = [];
        $.each(response.data, (_i, item) => {
          let title = item.login;
          if (item.full_name && item.full_name.length > 0) {
            title += ` (${htmlEscape(item.full_name)})`;
          }
          items.push({
            title,
            image: item.avatar_url
          });
        });

        return {results: items};
      }
    },
    searchFields: ['login', 'full_name'],
    showNoResults: false
  });
}

function searchTeams() {
  const $searchTeamBox = $('#search-team-box');
  $searchTeamBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${AppSubUrl}/api/v1/orgs/${$searchTeamBox.data('org')}/teams/search?q={query}`,
      headers: {'X-Csrf-Token': csrf},
      onResponse(response) {
        const items = [];
        $.each(response.data, (_i, item) => {
          const title = `${item.name} (${item.permission} access)`;
          items.push({
            title,
          });
        });

        return {results: items};
      }
    },
    searchFields: ['name', 'description'],
    showNoResults: false
  });
}

function searchRepositories() {
  const $searchRepoBox = $('#search-repo-box');
  $searchRepoBox.search({
    minCharacters: 2,
    apiSettings: {
      url: `${AppSubUrl}/api/v1/repos/search?q={query}&uid=${$searchRepoBox.data('uid')}`,
      onResponse(response) {
        const items = [];
        $.each(response.data, (_i, item) => {
          items.push({
            title: item.full_name.split('/')[1],
            description: item.full_name
          });
        });

        return {results: items};
      }
    },
    searchFields: ['full_name'],
    showNoResults: false
  });
}

function initCodeView() {
  if ($('.code-view .lines-num').length > 0) {
    $(document).on('click', '.lines-num span', function (e) {
      const $select = $(this);
      let $list;
      if ($('div.blame').length) {
        $list = $('.code-view td.lines-code li');
      } else {
        $list = $('.code-view td.lines-code');
      }
      selectRange($list, $list.filter(`[rel=${$select.attr('id')}]`), (e.shiftKey ? $list.filter('.active').eq(0) : null));
      deSelect();
    });

    $(window).on('hashchange', () => {
      let m = window.location.hash.match(/^#(L\d+)-(L\d+)$/);
      let $list;
      if ($('div.blame').length) {
        $list = $('.code-view td.lines-code li');
      } else {
        $list = $('.code-view td.lines-code');
      }
      let $first;
      if (m) {
        $first = $list.filter(`[rel=${m[1]}]`);
        selectRange($list, $first, $list.filter(`[rel=${m[2]}]`));
        $('html, body').scrollTop($first.offset().top - 200);
        return;
      }
      m = window.location.hash.match(/^#(L|n)(\d+)$/);
      if (m) {
        $first = $list.filter(`[rel=L${m[2]}]`);
        selectRange($list, $first);
        $('html, body').scrollTop($first.offset().top - 200);
      }
    }).trigger('hashchange');
  }
  $(document).on('click', '.fold-file', ({currentTarget}) => {
    const box = currentTarget.closest('.file-content');
    const folded = box.dataset.folded !== 'true';
    currentTarget.innerHTML = svg(`octicon-chevron-${folded ? 'right' : 'down'}`, 18);
    box.dataset.folded = String(folded);
  });
  $(document).on('click', '.blob-excerpt', async ({currentTarget}) => {
    const {url, query, anchor} = currentTarget.dataset;
    const blob = await $.get(`${url}?${query}&anchor=${anchor}`);
    currentTarget.closest('tr').outerHTML = blob;
  });
}

function initU2FAuth() {
  if ($('#wait-for-key').length === 0) {
    return;
  }
  u2fApi.ensureSupport()
    .then(() => {
      $.getJSON(`${AppSubUrl}/user/u2f/challenge`).done((req) => {
        u2fApi.sign(req.appId, req.challenge, req.registeredKeys, 30)
          .then(u2fSigned)
          .catch((err) => {
            if (err === undefined) {
              u2fError(1);
              return;
            }
            u2fError(err.metaData.code);
          });
      });
    }).catch(() => {
      // Fallback in case browser do not support U2F
      window.location.href = `${AppSubUrl}/user/two_factor`;
    });
}
function u2fSigned(resp) {
  $.ajax({
    url: `${AppSubUrl}/user/u2f/sign`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrf},
    data: JSON.stringify(resp),
    contentType: 'application/json; charset=utf-8',
  }).done((res) => {
    window.location.replace(res);
  }).fail(() => {
    u2fError(1);
  });
}

function u2fRegistered(resp) {
  if (checkError(resp)) {
    return;
  }
  $.ajax({
    url: `${AppSubUrl}/user/settings/security/u2f/register`,
    type: 'POST',
    headers: {'X-Csrf-Token': csrf},
    data: JSON.stringify(resp),
    contentType: 'application/json; charset=utf-8',
    success() {
      reload();
    },
    fail() {
      u2fError(1);
    }
  });
}

function checkError(resp) {
  if (!('errorCode' in resp)) {
    return false;
  }
  if (resp.errorCode === 0) {
    return false;
  }
  u2fError(resp.errorCode);
  return true;
}

function u2fError(errorType) {
  const u2fErrors = {
    browser: $('#unsupported-browser'),
    1: $('#u2f-error-1'),
    2: $('#u2f-error-2'),
    3: $('#u2f-error-3'),
    4: $('#u2f-error-4'),
    5: $('.u2f-error-5')
  };
  u2fErrors[errorType].removeClass('hide');

  Object.keys(u2fErrors).forEach((type) => {
    if (type !== errorType) {
      u2fErrors[type].addClass('hide');
    }
  });
  $('#u2f-error').modal('show');
}

function initU2FRegister() {
  $('#register-device').modal({allowMultiple: false});
  $('#u2f-error').modal({allowMultiple: false});
  $('#register-security-key').on('click', (e) => {
    e.preventDefault();
    u2fApi.ensureSupport()
      .then(u2fRegisterRequest)
      .catch(() => {
        u2fError('browser');
      });
  });
}

function u2fRegisterRequest() {
  $.post(`${AppSubUrl}/user/settings/security/u2f/request_register`, {
    _csrf: csrf,
    name: $('#nickname').val()
  }).done((req) => {
    $('#nickname').closest('div.field').removeClass('error');
    $('#register-device').modal('show');
    if (req.registeredKeys === null) {
      req.registeredKeys = [];
    }
    u2fApi.register(req.appId, req.registerRequests, req.registeredKeys, 30)
      .then(u2fRegistered)
      .catch((reason) => {
        if (reason === undefined) {
          u2fError(1);
          return;
        }
        u2fError(reason.metaData.code);
      });
  }).fail((xhr) => {
    if (xhr.status === 409) {
      $('#nickname').closest('div.field').addClass('error');
    }
  });
}

function initWipTitle() {
  $('.title_wip_desc > a').on('click', (e) => {
    e.preventDefault();

    const $issueTitle = $('#issue_title');
    $issueTitle.focus();
    const value = $issueTitle.val().trim().toUpperCase();

    const wipPrefixes = $('.title_wip_desc').data('wip-prefixes');
    for (const prefix of wipPrefixes) {
      if (value.startsWith(prefix.toUpperCase())) {
        return;
      }
    }

    $issueTitle.val(`${wipPrefixes[0]} ${$issueTitle.val()}`);
  });
}

function initTemplateSearch() {
  const $repoTemplate = $('#repo_template');
  const checkTemplate = function () {
    const $templateUnits = $('#template_units');
    const $nonTemplate = $('#non_template');
    if ($repoTemplate.val() !== '' && $repoTemplate.val() !== '0') {
      $templateUnits.show();
      $nonTemplate.hide();
    } else {
      $templateUnits.hide();
      $nonTemplate.show();
    }
  };
  $repoTemplate.on('change', checkTemplate);
  checkTemplate();

  const changeOwner = function () {
    $('#repo_template_search')
      .dropdown({
        apiSettings: {
          url: `${AppSubUrl}/api/v1/repos/search?q={query}&template=true&priority_owner_id=${$('#uid').val()}`,
          onResponse(response) {
            const filteredResponse = {success: true, results: []};
            filteredResponse.results.push({
              name: '',
              value: ''
            });
            // Parse the response from the api to work with our dropdown
            $.each(response.data, (_r, repo) => {
              filteredResponse.results.push({
                name: htmlEscape(repo.full_name),
                value: repo.id
              });
            });
            return filteredResponse;
          },
          cache: false,
        },

        fullTextSearch: true
      });
  };
  $('#uid').on('change', changeOwner);
  changeOwner();
}

$(document).ready(async () => {
  // Show exact time
  $('.time-since').each(function () {
    $(this)
      .addClass('poping up')
      .attr('data-content', $(this).attr('title'))
      .attr('data-variation', 'inverted tiny')
      .attr('title', '');
  });

  // Semantic UI modules.
  $('.dropdown:not(.custom)').dropdown();
  $('.jump.dropdown').dropdown({
    action: 'hide',
    onShow() {
      $('.poping.up').popup('hide');
    }
  });
  $('.slide.up.dropdown').dropdown({
    transition: 'slide up'
  });
  $('.upward.dropdown').dropdown({
    direction: 'upward'
  });
  $('.ui.accordion').accordion();
  $('.ui.checkbox').checkbox();
  $('.ui.progress').progress({
    showActivity: false
  });
  $('.poping.up').popup();
  $('.top.menu .poping.up').popup({
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

  // make table <tr> element clickable like a link
  $('tr[data-href]').on('click', function () {
    window.location = $(this).data('href');
  });

  // make table <td> element clickable like a link
  $('td[data-href]').click(function () {
    window.location = $(this).data('href');
  });

  // Dropzone
  const $dropzone = $('#dropzone');
  if ($dropzone.length > 0) {
    const filenameDict = {};

    await createDropzone('#dropzone', {
      url: $dropzone.data('upload-url'),
      headers: {'X-Csrf-Token': csrf},
      maxFiles: $dropzone.data('max-file'),
      maxFilesize: $dropzone.data('max-size'),
      acceptedFiles: (['*/*', ''].includes($dropzone.data('accepts'))) ? null : $dropzone.data('accepts'),
      addRemoveLinks: true,
      dictDefaultMessage: $dropzone.data('default-message'),
      dictInvalidFileType: $dropzone.data('invalid-input-type'),
      dictFileTooBig: $dropzone.data('file-too-big'),
      dictRemoveFile: $dropzone.data('remove-file'),
      timeout: 0,
      init() {
        this.on('success', (file, data) => {
          filenameDict[file.name] = data.uuid;
          const input = $(`<input id="${data.uuid}" name="files" type="hidden">`).val(data.uuid);
          $('.files').append(input);
        });
        this.on('removedfile', (file) => {
          if (file.name in filenameDict) {
            $(`#${filenameDict[file.name]}`).remove();
          }
          if ($dropzone.data('remove-url')) {
            $.post($dropzone.data('remove-url'), {
              file: filenameDict[file.name],
              _csrf: csrf
            });
          }
        });
      },
    });
  }

  // Helpers.
  $('.delete-button').on('click', showDeletePopup);
  $('.add-all-button').on('click', showAddAllPopup);
  $('.link-action').on('click', linkAction);
  $('.language-menu a[lang]').on('click', linkLanguageAction);
  $('.link-email-action').on('click', linkEmailAction);

  $('.delete-branch-button').on('click', showDeletePopup);

  $('.undo-button').on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrf,
      id: $this.data('id')
    }).done((data) => {
      window.location.href = data.redirect;
    });
  });
  $('.show-panel.button').on('click', function () {
    $($(this).data('panel')).show();
  });
  $('.show-modal.button').on('click', function () {
    $($(this).data('modal')).modal('show');
  });
  $('.delete-post.button').on('click', function () {
    const $this = $(this);
    $.post($this.data('request-url'), {
      _csrf: csrf
    }).done(() => {
      window.location.href = $this.data('done-url');
    });
  });

  $('.issue-checkbox').on('click', () => {
    const numChecked = $('.issue-checkbox').children('input:checked').length;
    if (numChecked > 0) {
      $('#issue-filters').addClass('hide');
      $('#issue-actions').removeClass('hide');
    } else {
      $('#issue-filters').removeClass('hide');
      $('#issue-actions').addClass('hide');
    }
  });

  $('.issue-action').on('click', function () {
    let {action} = this.dataset;
    let {elementId} = this.dataset;
    const issueIDs = $('.issue-checkbox').children('input:checked').map(function () {
      return this.dataset.issueId;
    }).get().join();
    const {url} = this.dataset;
    if (elementId === '0' && url.substr(-9) === '/assignee') {
      elementId = '';
      action = 'clear';
    }
    updateIssuesMeta(url, action, issueIDs, elementId, '').then(() => {
      // NOTICE: This reset of checkbox state targets Firefox caching behaviour, as the checkboxes stay checked after reload
      if (action === 'close' || action === 'open') {
        // uncheck all checkboxes
        $('.issue-checkbox input[type="checkbox"]').each((_, e) => { e.checked = false });
      }
      reload();
    });
  });

  // NOTICE: This event trigger targets Firefox caching behaviour, as the checkboxes stay checked after reload
  // trigger ckecked event, if checkboxes are checked on load
  $('.issue-checkbox input[type="checkbox"]:checked').first().each((_, e) => {
    e.checked = false;
    $(e).trigger('click');
  });

  $('.resolve-conversation').on('click', function (e) {
    e.preventDefault();
    const id = $(this).data('comment-id');
    const action = $(this).data('action');
    const url = $(this).data('update-url');

    $.post(url, {
      _csrf: csrf,
      action,
      comment_id: id,
    }).then(reload);
  });

  buttonsClickOnEnter();
  searchUsers();
  searchTeams();
  searchRepositories();

  initMarkdownAnchors();
  initCommentForm();
  initInstall();
  initArchiveLinks();
  initRepository();
  initMigration();
  initWikiForm();
  initEditForm();
  initEditor();
  initOrganization();
  initWebhook();
  initAdmin();
  initCodeView();
  initVueApp();
  initTeamSettings();
  initCtrlEnterSubmit();
  initNavbarContentToggle();
  initTopicbar();
  initU2FAuth();
  initU2FRegister();
  initIssueList();
  initWipTitle();
  initPullRequestReview();
  initRepoStatusChecker();
  initTemplateSearch();
  initContextPopups();
  initTableSort();
  initNotificationsTable();

  // Repo clone url.
  if ($('#repo-clone-url').length > 0) {
    switch (localStorage.getItem('repo-clone-protocol')) {
      case 'ssh':
        if ($('#repo-clone-ssh').length > 0) {
          $('#repo-clone-ssh').trigger('click');
        } else {
          $('#repo-clone-https').trigger('click');
        }
        break;
      default:
        $('#repo-clone-https').trigger('click');
        break;
    }
  }

  const routes = {
    'div.user.settings': initUserSettings,
    'div.repository.settings.collaboration': initRepositoryCollaboration
  };

  for (const [selector, fn] of Object.entries(routes)) {
    if ($(selector).length > 0) {
      fn();
      break;
    }
  }

  // parallel init of async loaded features
  await Promise.all([
    attachTribute(document.querySelectorAll('#content, .emoji-input')),
    initGitGraph(),
    initClipboard(),
    initUserHeatmap(),
    initProject(),
    initServiceWorker(),
    initNotificationCount(),
    renderMarkdownContent(),
    initGithook(),
  ]);
});

function changeHash(hash) {
  if (window.history.pushState) {
    window.history.pushState(null, null, hash);
  } else {
    window.location.hash = hash;
  }
}

function deSelect() {
  if (window.getSelection) {
    window.getSelection().removeAllRanges();
  } else {
    document.selection.empty();
  }
}

function selectRange($list, $select, $from) {
  $list.removeClass('active');
  if ($from) {
    let a = parseInt($select.attr('rel').substr(1));
    let b = parseInt($from.attr('rel').substr(1));
    let c;
    if (a !== b) {
      if (a > b) {
        c = a;
        a = b;
        b = c;
      }
      const classes = [];
      for (let i = a; i <= b; i++) {
        classes.push(`[rel=L${i}]`);
      }
      $list.filter(classes.join(',')).addClass('active');
      changeHash(`#L${a}-L${b}`);
      return;
    }
  }
  $select.addClass('active');
  changeHash(`#${$select.attr('rel')}`);
}

$(() => {
  // Warn users that try to leave a page after entering data into a form.
  // Except on sign-in pages, and for forms marked as 'ignore-dirty'.
  if ($('.user.signin').length === 0) {
    $('form:not(.ignore-dirty)').areYouSure();
  }

  // Parse SSH Key
  $('#ssh-key-content').on('change paste keyup', function () {
    const arrays = $(this).val().split(' ');
    const $title = $('#ssh-key-title');
    if ($title.val() === '' && arrays.length === 3 && arrays[2] !== '') {
      $title.val(arrays[2]);
    }
  });
});

function showDeletePopup() {
  const $this = $(this);
  let filter = '';
  if ($this.attr('id')) {
    filter += `#${$this.attr('id')}`;
  }

  const dialog = $(`.delete.modal${filter}`);
  dialog.find('.name').text($this.data('name'));

  dialog.modal({
    closable: false,
    onApprove() {
      if ($this.data('type') === 'form') {
        $($this.data('form')).trigger('submit');
        return;
      }

      $.post($this.data('url'), {
        _csrf: csrf,
        id: $this.data('id')
      }).done((data) => {
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
        _csrf: csrf,
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
    _csrf: csrf
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

function linkLanguageAction() {
  const $this = $(this);
  $.post($this.data('url')).always(() => {
    window.location.reload();
  });
}

function linkEmailAction(e) {
  const $this = $(this);
  $('#form-uid').val($this.data('uid'));
  $('#form-email').val($this.data('email'));
  $('#form-primary').val($this.data('primary'));
  $('#form-activate').val($this.data('activate'));
  $('#form-uid').val($this.data('uid'));
  $('#change-email-modal').modal('show');
  e.preventDefault();
}

function initVueComponents() {
  // register svg icon vue components, e.g. <octicon-repo size="16"/>
  for (const [name, htmlString] of Object.entries(svgs)) {
    const template = htmlString
      .replace(/height="[0-9]+"/, 'v-bind:height="size"')
      .replace(/width="[0-9]+"/, 'v-bind:width="size"');

    Vue.component(name, {
      props: {
        size: {
          type: String,
          default: '16',
        },
      },
      template,
    });
  }

  const vueDelimeters = ['${', '}'];

  Vue.component('repo-search', {
    delimiters: vueDelimeters,

    props: {
      searchLimit: {
        type: Number,
        default: 10
      },
      suburl: {
        type: String,
        required: true
      },
      uid: {
        type: Number,
        required: true
      },
      organizations: {
        type: Array,
        default: () => [],
      },
      isOrganization: {
        type: Boolean,
        default: true
      },
      canCreateOrganization: {
        type: Boolean,
        default: false
      },
      organizationsTotalCount: {
        type: Number,
        default: 0
      },
      moreReposLink: {
        type: String,
        default: ''
      }
    },

    data() {
      const params = new URLSearchParams(window.location.search);

      let tab = params.get('repo-search-tab');
      if (!tab) {
        tab = 'repos';
      }

      let reposFilter = params.get('repo-search-filter');
      if (!reposFilter) {
        reposFilter = 'all';
      }

      let privateFilter = params.get('repo-search-private');
      if (!privateFilter) {
        privateFilter = 'both';
      }

      let archivedFilter = params.get('repo-search-archived');
      if (!archivedFilter) {
        archivedFilter = 'unarchived';
      }

      let searchQuery = params.get('repo-search-query');
      if (!searchQuery) {
        searchQuery = '';
      }

      let page = 1;
      try {
        page = parseInt(params.get('repo-search-page'));
      } catch {
        // noop
      }
      if (!page) {
        page = 1;
      }

      return {
        tab,
        repos: [],
        reposTotalCount: 0,
        reposFilter,
        archivedFilter,
        privateFilter,
        page,
        finalPage: 1,
        searchQuery,
        isLoading: false,
        staticPrefix: StaticUrlPrefix,
        counts: {},
        repoTypes: {
          all: {
            searchMode: '',
          },
          forks: {
            searchMode: 'fork',
          },
          mirrors: {
            searchMode: 'mirror',
          },
          sources: {
            searchMode: 'source',
          },
          collaborative: {
            searchMode: 'collaborative',
          },
        }
      };
    },

    computed: {
      showMoreReposLink() {
        return this.repos.length > 0 && this.repos.length < this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
      },
      searchURL() {
        return `${this.suburl}/api/v1/repos/search?sort=updated&order=desc&uid=${this.uid}&q=${this.searchQuery
        }&page=${this.page}&limit=${this.searchLimit}&mode=${this.repoTypes[this.reposFilter].searchMode
        }${this.reposFilter !== 'all' ? '&exclusive=1' : ''
        }${this.archivedFilter === 'archived' ? '&archived=true' : ''}${this.archivedFilter === 'unarchived' ? '&archived=false' : ''
        }${this.privateFilter === 'private' ? '&is_private=true' : ''}${this.privateFilter === 'public' ? '&is_private=false' : ''
        }`;
      },
      repoTypeCount() {
        return this.counts[`${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`];
      }
    },

    mounted() {
      this.searchRepos(this.reposFilter);
      $(this.$el).find('.poping.up').popup();
      $(this.$el).find('.dropdown').dropdown();
      this.setCheckboxes();
      const self = this;
      Vue.nextTick(() => {
        self.$refs.search.focus();
      });
    },

    methods: {
      changeTab(t) {
        this.tab = t;
        this.updateHistory();
      },

      setCheckboxes() {
        switch (this.archivedFilter) {
          case 'unarchived':
            $('#archivedFilterCheckbox').checkbox('set unchecked');
            break;
          case 'archived':
            $('#archivedFilterCheckbox').checkbox('set checked');
            break;
          case 'both':
            $('#archivedFilterCheckbox').checkbox('set indeterminate');
            break;
          default:
            this.archivedFilter = 'unarchived';
            $('#archivedFilterCheckbox').checkbox('set unchecked');
            break;
        }
        switch (this.privateFilter) {
          case 'public':
            $('#privateFilterCheckbox').checkbox('set unchecked');
            break;
          case 'private':
            $('#privateFilterCheckbox').checkbox('set checked');
            break;
          case 'both':
            $('#privateFilterCheckbox').checkbox('set indeterminate');
            break;
          default:
            this.privateFilter = 'both';
            $('#privateFilterCheckbox').checkbox('set indeterminate');
            break;
        }
      },

      changeReposFilter(filter) {
        this.reposFilter = filter;
        this.repos = [];
        this.page = 1;
        Vue.set(this.counts, `${filter}:${this.archivedFilter}:${this.privateFilter}`, 0);
        this.searchRepos();
      },

      updateHistory() {
        const params = new URLSearchParams(window.location.search);

        if (this.tab === 'repos') {
          params.delete('repo-search-tab');
        } else {
          params.set('repo-search-tab', this.tab);
        }

        if (this.reposFilter === 'all') {
          params.delete('repo-search-filter');
        } else {
          params.set('repo-search-filter', this.reposFilter);
        }

        if (this.privateFilter === 'both') {
          params.delete('repo-search-private');
        } else {
          params.set('repo-search-private', this.privateFilter);
        }

        if (this.archivedFilter === 'unarchived') {
          params.delete('repo-search-archived');
        } else {
          params.set('repo-search-archived', this.archivedFilter);
        }

        if (this.searchQuery === '') {
          params.delete('repo-search-query');
        } else {
          params.set('repo-search-query', this.searchQuery);
        }

        if (this.page === 1) {
          params.delete('repo-search-page');
        } else {
          params.set('repo-search-page', `${this.page}`);
        }

        const queryString = params.toString();
        if (queryString) {
          window.history.replaceState({}, '', `?${queryString}`);
        } else {
          window.history.replaceState({}, '', window.location.pathname);
        }
      },

      toggleArchivedFilter() {
        switch (this.archivedFilter) {
          case 'both':
            this.archivedFilter = 'unarchived';
            break;
          case 'unarchived':
            this.archivedFilter = 'archived';
            break;
          case 'archived':
            this.archivedFilter = 'both';
            break;
          default:
            this.archivedFilter = 'unarchived';
            break;
        }
        this.page = 1;
        this.repos = [];
        this.setCheckboxes();
        Vue.set(this.counts, `${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`, 0);
        this.searchRepos();
      },

      togglePrivateFilter() {
        switch (this.privateFilter) {
          case 'both':
            this.privateFilter = 'public';
            break;
          case 'public':
            this.privateFilter = 'private';
            break;
          case 'private':
            this.privateFilter = 'both';
            break;
          default:
            this.privateFilter = 'both';
            break;
        }
        this.page = 1;
        this.repos = [];
        this.setCheckboxes();
        Vue.set(this.counts, `${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`, 0);
        this.searchRepos();
      },


      changePage(page) {
        this.page = page;
        if (this.page > this.finalPage) {
          this.page = this.finalPage;
        }
        if (this.page < 1) {
          this.page = 1;
        }
        this.repos = [];
        Vue.set(this.counts, `${this.reposFilter}:${this.archivedFilter}:${this.privateFilter}`, 0);
        this.searchRepos();
      },

      searchRepos() {
        const self = this;

        this.isLoading = true;

        if (!this.reposTotalCount) {
          const totalCountSearchURL = `${this.suburl}/api/v1/repos/search?sort=updated&order=desc&uid=${this.uid}&q=&page=1&mode=`;
          $.getJSON(totalCountSearchURL, (_result, _textStatus, request) => {
            self.reposTotalCount = request.getResponseHeader('X-Total-Count');
          });
        }

        const searchedMode = this.repoTypes[this.reposFilter].searchMode;
        const searchedURL = this.searchURL;
        const searchedQuery = this.searchQuery;

        $.getJSON(searchedURL, (result, _textStatus, request) => {
          if (searchedURL === self.searchURL) {
            self.repos = result.data;
            const count = request.getResponseHeader('X-Total-Count');
            if (searchedQuery === '' && searchedMode === '' && self.archivedFilter === 'both') {
              self.reposTotalCount = count;
            }
            Vue.set(self.counts, `${self.reposFilter}:${self.archivedFilter}:${self.privateFilter}`, count);
            self.finalPage = Math.floor(count / self.searchLimit) + 1;
            self.updateHistory();
          }
        }).always(() => {
          if (searchedURL === self.searchURL) {
            self.isLoading = false;
          }
        });
      },

      repoIcon(repo) {
        if (repo.fork) {
          return 'octicon-repo-forked';
        } else if (repo.mirror) {
          return 'octicon-mirror';
        } else if (repo.template) {
          return `octicon-repo-template`;
        } else if (repo.private) {
          return 'octicon-lock';
        } else if (repo.internal) {
          return 'octicon-repo';
        }
        return 'octicon-repo';
      }
    }
  });
}

function initCtrlEnterSubmit() {
  $('.js-quick-submit').on('keydown', function (e) {
    if (((e.ctrlKey && !e.altKey) || e.metaKey) && (e.keyCode === 13 || e.keyCode === 10)) {
      $(this).closest('form').trigger('submit');
    }
  });
}

function initVueApp() {
  const el = document.getElementById('app');
  if (!el) {
    return;
  }

  initVueComponents();

  new Vue({
    el,
    delimiters: ['${', '}'],
    components: {
      ActivityTopAuthors,
    },
    data: () => {
      return {
        searchLimit: Number((document.querySelector('meta[name=_search_limit]') || {}).content),
        suburl: AppSubUrl,
        uid: Number((document.querySelector('meta[name=_context_uid]') || {}).content),
        activityTopAuthors: window.ActivityTopAuthors || [],
      };
    },
  });
}

window.timeAddManual = function () {
  $('.mini.modal')
    .modal({
      duration: 200,
      onApprove() {
        $('#add_time_manual_form').trigger('submit');
      }
    }).modal('show');
};

window.toggleStopwatch = function () {
  $('#toggle_stopwatch_form').trigger('submit');
};
window.cancelStopwatch = function () {
  $('#cancel_stopwatch_form').trigger('submit');
};

$('.commit-button').on('click', function (e) {
  e.preventDefault();
  $(this).parent().find('.commit-body').toggle();
});

function initNavbarContentToggle() {
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

function initTopicbar() {
  const mgrBtn = $('#manage_topic');
  const editDiv = $('#topic_edit');
  const viewDiv = $('#repo-topics');
  const saveBtn = $('#save_topic');
  const topicDropdown = $('#topic_edit .dropdown');
  const topicForm = $('#topic_edit.ui.form');
  const topicPrompts = getPrompts();

  mgrBtn.on('click', () => {
    viewDiv.hide();
    editDiv.css('display', ''); // show Semantic UI Grid
  });

  function getPrompts() {
    const hidePrompt = $('div.hide#validate_prompt');
    const prompts = {
      countPrompt: hidePrompt.children('#count_prompt').text(),
      formatPrompt: hidePrompt.children('#format_prompt').text()
    };
    hidePrompt.remove();
    return prompts;
  }

  saveBtn.on('click', () => {
    const topics = $('input[name=topics]').val();

    $.post(saveBtn.data('link'), {
      _csrf: csrf,
      topics
    }, (_data, _textStatus, xhr) => {
      if (xhr.responseJSON.status === 'ok') {
        viewDiv.children('.topic').remove();
        if (topics.length) {
          const topicArray = topics.split(',');

          const last = viewDiv.children('a').last();
          for (let i = 0; i < topicArray.length; i++) {
            const link = $('<a class="ui repo-topic small label topic"></a>');
            link.attr('href', `${AppSubUrl}/explore/repos?q=${encodeURIComponent(topicArray[i])}&topic=1`);
            link.text(topicArray[i]);
            link.insertBefore(last);
          }
        }
        editDiv.css('display', 'none');
        viewDiv.show();
      }
    }).fail((xhr) => {
      if (xhr.status === 422) {
        if (xhr.responseJSON.invalidTopics.length > 0) {
          topicPrompts.formatPrompt = xhr.responseJSON.message;

          const {invalidTopics} = xhr.responseJSON;
          const topicLables = topicDropdown.children('a.ui.label');

          topics.split(',').forEach((value, index) => {
            for (let i = 0; i < invalidTopics.length; i++) {
              if (invalidTopics[i] === value) {
                topicLables.eq(index).removeClass('green').addClass('red');
              }
            }
          });
        } else {
          topicPrompts.countPrompt = xhr.responseJSON.message;
        }
      }
    }).always(() => {
      topicForm.form('validate form');
    });
  });

  topicDropdown.dropdown({
    allowAdditions: true,
    forceSelection: false,
    fields: {name: 'description', value: 'data-value'},
    saveRemoteData: false,
    label: {
      transition: 'horizontal flip',
      duration: 200,
      variation: false,
      blue: true,
      basic: true,
    },
    className: {
      label: 'ui small label'
    },
    apiSettings: {
      url: `${AppSubUrl}/api/v1/topics/search?q={query}`,
      throttle: 500,
      cache: false,
      onResponse(res) {
        const formattedResponse = {
          success: false,
          results: [],
        };
        const query = stripTags(this.urlData.query.trim());
        let found_query = false;
        const current_topics = [];
        topicDropdown.find('div.label.visible.topic,a.label.visible').each((_, e) => { current_topics.push(e.dataset.value) });

        if (res.topics) {
          let found = false;
          for (let i = 0; i < res.topics.length; i++) {
            // skip currently added tags
            if (current_topics.includes(res.topics[i].topic_name)) {
              continue;
            }

            if (res.topics[i].topic_name.toLowerCase() === query.toLowerCase()) {
              found_query = true;
            }
            formattedResponse.results.push({description: res.topics[i].topic_name, 'data-value': res.topics[i].topic_name});
            found = true;
          }
          formattedResponse.success = found;
        }

        if (query.length > 0 && !found_query) {
          formattedResponse.success = true;
          formattedResponse.results.unshift({description: query, 'data-value': query});
        } else if (query.length > 0 && found_query) {
          formattedResponse.results.sort((a, b) => {
            if (a.description.toLowerCase() === query.toLowerCase()) return -1;
            if (b.description.toLowerCase() === query.toLowerCase()) return 1;
            if (a.description > b.description) return -1;
            if (a.description < b.description) return 1;
            return 0;
          });
        }

        return formattedResponse;
      },
    },
    onLabelCreate(value) {
      value = value.toLowerCase().trim();
      this.attr('data-value', value).contents().first().replaceWith(value);
      return $(this);
    },
    onAdd(addedValue, _addedText, $addedChoice) {
      addedValue = addedValue.toLowerCase().trim();
      $($addedChoice).attr('data-value', addedValue);
      $($addedChoice).attr('data-text', addedValue);
    }
  });

  $.fn.form.settings.rules.validateTopic = function (_values, regExp) {
    const topics = topicDropdown.children('a.ui.label');
    const status = topics.length === 0 || topics.last().attr('data-value').match(regExp);
    if (!status) {
      topics.last().removeClass('green').addClass('red');
    }
    return status && topicDropdown.children('a.ui.label.red').length === 0;
  };

  topicForm.form({
    on: 'change',
    inline: true,
    fields: {
      topics: {
        identifier: 'topics',
        rules: [
          {
            type: 'validateTopic',
            value: /^[a-z0-9][a-z0-9-]{0,35}$/,
            prompt: topicPrompts.formatPrompt
          },
          {
            type: 'maxCount[25]',
            prompt: topicPrompts.countPrompt
          }
        ]
      },
    }
  });
}

window.toggleDeadlineForm = function () {
  $('#deadlineForm').fadeToggle(150);
};

window.setDeadline = function () {
  const deadline = $('#deadlineDate').val();
  window.updateDeadline(deadline);
};

window.updateDeadline = function (deadlineString) {
  $('#deadline-err-invalid-date').hide();
  $('#deadline-loader').addClass('loading');

  let realDeadline = null;
  if (deadlineString !== '') {
    const newDate = Date.parse(deadlineString);

    if (Number.isNaN(newDate)) {
      $('#deadline-loader').removeClass('loading');
      $('#deadline-err-invalid-date').show();
      return false;
    }
    realDeadline = new Date(newDate);
  }

  $.ajax(`${$('#update-issue-deadline-form').attr('action')}/deadline`, {
    data: JSON.stringify({
      due_date: realDeadline,
    }),
    headers: {
      'X-Csrf-Token': csrf,
      'X-Remote': true,
    },
    contentType: 'application/json',
    type: 'POST',
    success() {
      reload();
    },
    error() {
      $('#deadline-loader').removeClass('loading');
      $('#deadline-err-invalid-date').show();
    }
  });
};

window.deleteDependencyModal = function (id, type) {
  $('.remove-dependency')
    .modal({
      closable: false,
      duration: 200,
      onApprove() {
        $('#removeDependencyID').val(id);
        $('#dependencyType').val(type);
        $('#removeDependencyForm').trigger('submit');
      }
    }).modal('show');
};

window.cancelCodeComment = function (btn) {
  const form = $(btn).closest('form');
  if (form.length > 0 && form.hasClass('comment-form')) {
    form.addClass('hide');
    form.parent().find('button.comment-form-reply').show();
  } else {
    form.closest('.comment-code-cloud').remove();
  }
};

window.submitReply = function (btn) {
  const form = $(btn).closest('form');
  if (form.length > 0 && form.hasClass('comment-form')) {
    form.trigger('submit');
  }
};

window.onOAuthLoginClick = function () {
  const oauthLoader = $('#oauth2-login-loader');
  const oauthNav = $('#oauth2-login-navigator');

  oauthNav.hide();
  oauthLoader.removeClass('disabled');

  setTimeout(() => {
    // recover previous content to let user try again
    // usually redirection will be performed before this action
    oauthLoader.addClass('disabled');
    oauthNav.show();
  }, 5000);
};
