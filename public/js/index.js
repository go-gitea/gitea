'use strict';

var csrf;
var suburl;

function initCommentPreviewTab($form) {
    var $tabMenu = $form.find('.tabular.menu');
    $tabMenu.find('.item').tab();
    $tabMenu.find('.item[data-tab="' + $tabMenu.data('preview') + '"]').click(function () {
        var $this = $(this);
        $.post($this.data('url'), {
                "_csrf": csrf,
                "mode": "gfm",
                "context": $this.data('context'),
                "text": $form.find('.tab.segment[data-tab="' + $tabMenu.data('write') + '"] textarea').val()
            },
            function (data) {
                var $previewPanel = $form.find('.tab.segment[data-tab="' + $tabMenu.data('preview') + '"]');
                $previewPanel.html(data);
                emojify.run($previewPanel[0]);
                $('pre code', $previewPanel[0]).each(function (i, block) {
                    hljs.highlightBlock(block);
                });
            }
        );
    });

    buttonsClickOnEnter();
}

var previewFileModes;

function initEditPreviewTab($form) {
    var $tabMenu = $form.find('.tabular.menu');
    $tabMenu.find('.item').tab();
    var $previewTab = $tabMenu.find('.item[data-tab="' + $tabMenu.data('preview') + '"]');
    if ($previewTab.length) {
        previewFileModes = $previewTab.data('preview-file-modes').split(',');
        $previewTab.click(function () {
            var $this = $(this);
            $.post($this.data('url'), {
                    "_csrf": csrf,
                    "mode": "gfm",
                    "context": $this.data('context'),
                    "text": $form.find('.tab.segment[data-tab="' + $tabMenu.data('write') + '"] textarea').val()
                },
                function (data) {
                    var $previewPanel = $form.find('.tab.segment[data-tab="' + $tabMenu.data('preview') + '"]');
                    $previewPanel.html(data);
                    emojify.run($previewPanel[0]);
                    $('pre code', $previewPanel[0]).each(function (i, block) {
                        hljs.highlightBlock(block);
                    });
                }
            );
        });
    }
}

function initEditDiffTab($form) {
    var $tabMenu = $form.find('.tabular.menu');
    $tabMenu.find('.item').tab();
    $tabMenu.find('.item[data-tab="' + $tabMenu.data('diff') + '"]').click(function () {
        var $this = $(this);
        $.post($this.data('url'), {
                "_csrf": csrf,
                "context": $this.data('context'),
                "content": $form.find('.tab.segment[data-tab="' + $tabMenu.data('write') + '"] textarea').val()
            },
            function (data) {
                var $diffPreviewPanel = $form.find('.tab.segment[data-tab="' + $tabMenu.data('diff') + '"]');
                $diffPreviewPanel.html(data);
                emojify.run($diffPreviewPanel[0]);
            }
        );
    });
}


function initEditForm() {
    if ($('.edit.form').length == 0) {
        return;
    }

    initEditPreviewTab($('.edit.form'));
    initEditDiffTab($('.edit.form'));
}


function updateIssuesMeta(url, action, issueIds, elementId, afterSuccess) {
    $.ajax({
        type: "POST",
        url: url,
        data: {
            "_csrf": csrf,
            "action": action,
            "issue_ids": issueIds,
            "id": elementId
        },
        success: afterSuccess
    })
}

function initCommentForm() {
    if ($('.comment.form').length == 0) {
        return
    }

    initCommentPreviewTab($('.comment.form'));

    // Labels
    var $list = $('.ui.labels.list');
    var $noSelect = $list.find('.no-select');
    var $labelMenu = $('.select-label .menu');
    var hasLabelUpdateAction = $labelMenu.data('action') == 'update';

    $('.select-label').dropdown('setting', 'onHide', function(){
        if (hasLabelUpdateAction) {
            location.reload();
        }
    });

    $labelMenu.find('.item:not(.no-select)').click(function () {
        if ($(this).hasClass('checked')) {
            $(this).removeClass('checked');
            $(this).find('.octicon').removeClass('octicon-check');
            if (hasLabelUpdateAction) {
                updateIssuesMeta(
                    $labelMenu.data('update-url'),
                    "detach",
                    $labelMenu.data('issue-id'),
                    $(this).data('id')
                );
            }
        } else {
            $(this).addClass('checked');
            $(this).find('.octicon').addClass('octicon-check');
            if (hasLabelUpdateAction) {
                updateIssuesMeta(
                    $labelMenu.data('update-url'),
                    "attach",
                    $labelMenu.data('issue-id'),
                    $(this).data('id')
                );
            }
        }

        var labelIds = [];
        $(this).parent().find('.item').each(function () {
            if ($(this).hasClass('checked')) {
                labelIds.push($(this).data('id'));
                $($(this).data('id-selector')).removeClass('hide');
            } else {
                $($(this).data('id-selector')).addClass('hide');
            }
        });
        if (labelIds.length == 0) {
            $noSelect.removeClass('hide');
        } else {
            $noSelect.addClass('hide');
        }
        $($(this).parent().data('id')).val(labelIds.join(","));
        return false;
    });
    $labelMenu.find('.no-select.item').click(function () {
        if (hasLabelUpdateAction) {
            updateIssuesMeta(
                $labelMenu.data('update-url'),
                "clear",
                $labelMenu.data('issue-id'),
                ""
            );
        }

        $(this).parent().find('.item').each(function () {
            $(this).removeClass('checked');
            $(this).find('.octicon').removeClass('octicon-check');
        });

        $list.find('.item').each(function () {
            $(this).addClass('hide');
        });
        $noSelect.removeClass('hide');
        $($(this).parent().data('id')).val('');
    });

    function selectItem(select_id, input_id) {
        var $menu = $(select_id + ' .menu');
        var $list = $('.ui' + select_id + '.list');
        var hasUpdateAction = $menu.data('action') == 'update';

        $menu.find('.item:not(.no-select)').click(function () {
            $(this).parent().find('.item').each(function () {
                $(this).removeClass('selected active')
            });

            $(this).addClass('selected active');
            if (hasUpdateAction) {
                updateIssuesMeta(
                    $menu.data('update-url'),
                    "",
                    $menu.data('issue-id'),
                    $(this).data('id'),
                    function() { location.reload(); }
                );
            }
            switch (input_id) {
                case '#milestone_id':
                    $list.find('.selected').html('<a class="item" href=' + $(this).data('href') + '>' +
                        $(this).text() + '</a>');
                    break;
                case '#assignee_id':
                    $list.find('.selected').html('<a class="item" href=' + $(this).data('href') + '>' +
                        '<img class="ui avatar image" src=' + $(this).data('avatar') + '>' +
                        $(this).text() + '</a>');
            }
            $('.ui' + select_id + '.list .no-select').addClass('hide');
            $(input_id).val($(this).data('id'));
        });
        $menu.find('.no-select.item').click(function () {
            $(this).parent().find('.item:not(.no-select)').each(function () {
                $(this).removeClass('selected active')
            });

            if (hasUpdateAction) {
                updateIssuesMeta(
                    $menu.data('update-url'),
                    "",
                    $menu.data('issue-id'),
                    $(this).data('id'),
                    function() { location.reload(); }
                );
            }

            $list.find('.selected').html('');
            $list.find('.no-select').removeClass('hide');
            $(input_id).val('');
        });
    }

    // Milestone and assignee
    selectItem('.select-milestone', '#milestone_id');
    selectItem('.select-assignee', '#assignee_id');
}

function initInstall() {
    if ($('.install').length == 0) {
        return;
    }

    if ($('#db_host').val()=="") {
        $('#db_host').val("127.0.0.1:3306");
        $('#db_user').val("gitea");
        $('#db_name').val("gitea");
    }

    // Database type change detection.
    $("#db_type").change(function () {
        var sqliteDefault = 'data/gitea.db';
        var tidbDefault = 'data/gitea_tidb';

        var dbType = $(this).val();
        if (dbType === "SQLite3" || dbType === "TiDB") {
            $('#sql_settings').hide();
            $('#pgsql_settings').hide();
            $('#sqlite_settings').show();

            if (dbType === "SQLite3" && $('#db_path').val() == tidbDefault) {
                $('#db_path').val(sqliteDefault);
            } else if (dbType === "TiDB" && $('#db_path').val() == sqliteDefault) {
                $('#db_path').val(tidbDefault);
            }
            return;
        }

        var dbDefaults = {
            "MySQL": "127.0.0.1:3306",
            "PostgreSQL": "127.0.0.1:5432",
            "MSSQL": "127.0.0.1:1433"
        };

        $('#sqlite_settings').hide();
        $('#sql_settings').show();

        $('#pgsql_settings').toggle(dbType === "PostgreSQL");
        $.each(dbDefaults, function(type, defaultHost) {
            if ($('#db_host').val() == defaultHost) {
                $('#db_host').val(dbDefaults[dbType]);
                return false;
            }
        });
    });

    // TODO: better handling of exclusive relations.
    $('#offline-mode input').change(function () {
        if ($(this).is(':checked')) {
            $('#disable-gravatar').checkbox('check');
            $('#federated-avatar-lookup').checkbox('uncheck');
        }
    });
    $('#disable-gravatar input').change(function () {
        if ($(this).is(':checked')) {
            $('#federated-avatar-lookup').checkbox('uncheck');
        } else {
            $('#offline-mode').checkbox('uncheck');
        }
    });
    $('#federated-avatar-lookup input').change(function () {
        if ($(this).is(':checked')) {
            $('#disable-gravatar').checkbox('uncheck');
            $('#offline-mode').checkbox('uncheck');
        }
    });
    $('#disable-registration input').change(function () {
        if ($(this).is(':checked')) {
            $('#enable-captcha').checkbox('uncheck');
        }
    });
    $('#enable-captcha input').change(function () {
        if ($(this).is(':checked')) {
            $('#disable-registration').checkbox('uncheck');
        }
    });
}

function initRepository() {
    if ($('.repository').length == 0) {
        return;
    }

    function initFilterSearchDropdown(selector) {
        var $dropdown = $(selector);
        $dropdown.dropdown({
            fullTextSearch: true,
            onChange: function (text, value, $choice) {
                window.location.href = $choice.data('url');
                console.log($choice.data('url'))
            },
            message: {noResults: $dropdown.data('no-results')}
        });
    }

    // File list and commits
    if ($('.repository.file.list').length > 0 ||
        ('.repository.commits').length > 0) {
        initFilterSearchDropdown('.choose.reference .dropdown');

        $('.reference.column').click(function () {
            $('.choose.reference .scrolling.menu').css('display', 'none');
            $('.choose.reference .text').removeClass('black');
            $($(this).data('target')).css('display', 'block');
            $(this).find('.text').addClass('black');
            return false;
        });
    }

    // Wiki
    if ($('.repository.wiki.view').length > 0) {
        initFilterSearchDropdown('.choose.page .dropdown');
    }

    // Options
    if ($('.repository.settings.options').length > 0) {
        $('#repo_name').keyup(function () {
            var $prompt = $('#repo-name-change-prompt');
            if ($(this).val().toString().toLowerCase() != $(this).data('repo-name').toString().toLowerCase()) {
                $prompt.show();
            } else {
                $prompt.hide();
            }
        });

        // Enable or select internal/external wiki system and issue tracker.
        $('.enable-system').change(function () {
            if (this.checked) {
                $($(this).data('target')).removeClass('disabled');
            } else {
                $($(this).data('target')).addClass('disabled');
            }
        });
        $('.enable-system-radio').change(function () {
            if (this.value == 'false') {
                $($(this).data('target')).addClass('disabled');
            } else if (this.value == 'true') {
                $($(this).data('target')).removeClass('disabled');
            }
        });
    }

    // Labels
    if ($('.repository.labels').length > 0) {
        // Create label
        var $newLabelPanel = $('.new-label.segment');
        $('.new-label.button').click(function () {
            $newLabelPanel.show();
        });
        $('.new-label.segment .cancel').click(function () {
            $newLabelPanel.hide();
        });

        $('.color-picker').each(function () {
            $(this).minicolors();
        });
        $('.precolors .color').click(function () {
            var color_hex = $(this).data('color-hex');
            $('.color-picker').val(color_hex);
            $('.minicolors-swatch-color').css("background-color", color_hex);
        });
        $('.edit-label-button').click(function () {
            $('#label-modal-id').val($(this).data('id'));
            $('.edit-label .new-label-input').val($(this).data('title'));
            $('.edit-label .color-picker').val($(this).data('color'));
            $('.minicolors-swatch-color').css("background-color", $(this).data('color'));
            $('.edit-label.modal').modal({
                onApprove: function () {
                    $('.edit-label.form').submit();
                }
            }).modal('show');
            return false;
        });
    }

    // Milestones
    if ($('.repository.milestones').length > 0) {

    }
    if ($('.repository.new.milestone').length > 0) {
        var $datepicker = $('.milestone.datepicker');
        $datepicker.datetimepicker({
            lang: $datepicker.data('lang'),
            inline: true,
            timepicker: false,
            startDate: $datepicker.data('start-date'),
            formatDate: 'Y-m-d',
            onSelectDate: function (ct) {
                $('#deadline').val(ct.dateFormat('Y-m-d'));
            }
        });
        $('#clear-date').click(function () {
            $('#deadline').val('');
            return false;
        });
    }

    // Issues
    if ($('.repository.view.issue').length > 0) {
        // Edit issue title
        var $issueTitle = $('#issue-title');
        var $editInput = $('#edit-title-input input');
        var editTitleToggle = function () {
            $issueTitle.toggle();
            $('.not-in-edit').toggle();
            $('#edit-title-input').toggle();
            $('.in-edit').toggle();
            $editInput.focus();
            return false;
        };
        $('#edit-title').click(editTitleToggle);
        $('#cancel-edit-title').click(editTitleToggle);
        $('#save-edit-title').click(editTitleToggle).click(function () {
            if ($editInput.val().length == 0 ||
                $editInput.val() == $issueTitle.text()) {
                $editInput.val($issueTitle.text());
                return false;
            }

            $.post($(this).data('update-url'), {
                    "_csrf": csrf,
                    "title": $editInput.val()
                },
                function (data) {
                    $editInput.val(data.title);
                    $issueTitle.text(data.title);
                    location.reload();
                });
            return false;
        });

        // Edit issue or comment content
        $('.edit-content').click(function () {
            var $segment = $(this).parent().parent().parent().next();
            var $editContentZone = $segment.find('.edit-content-zone');
            var $renderContent = $segment.find('.render-content');
            var $rawContent = $segment.find('.raw-content');
            var $textarea;

            // Setup new form
            if ($editContentZone.html().length == 0) {
                $editContentZone.html($('#edit-content-form').html());
                $textarea = $segment.find('textarea');

                // Give new write/preview data-tab name to distinguish from others
                var $editContentForm = $editContentZone.find('.ui.comment.form');
                var $tabMenu = $editContentForm.find('.tabular.menu');
                $tabMenu.attr('data-write', $editContentZone.data('write'));
                $tabMenu.attr('data-preview', $editContentZone.data('preview'));
                $tabMenu.find('.write.item').attr('data-tab', $editContentZone.data('write'));
                $tabMenu.find('.preview.item').attr('data-tab', $editContentZone.data('preview'));
                $editContentForm.find('.write.segment').attr('data-tab', $editContentZone.data('write'));
                $editContentForm.find('.preview.segment').attr('data-tab', $editContentZone.data('preview'));

                initCommentPreviewTab($editContentForm);

                $editContentZone.find('.cancel.button').click(function () {
                    $renderContent.show();
                    $editContentZone.hide();
                });
                $editContentZone.find('.save.button').click(function () {
                    $renderContent.show();
                    $editContentZone.hide();

                    $.post($editContentZone.data('update-url'), {
                            "_csrf": csrf,
                            "content": $textarea.val(),
                            "context": $editContentZone.data('context')
                        },
                        function (data) {
                            if (data.length == 0) {
                                $renderContent.html($('#no-content').html());
                            } else {
                                $renderContent.html(data.content);
                                emojify.run($renderContent[0]);
                                $('pre code', $renderContent[0]).each(function (i, block) {
                                    hljs.highlightBlock(block);
                                });
                            }
                        });
                });
            } else {
                $textarea = $segment.find('textarea');
            }

            // Show write/preview tab and copy raw content as needed
            $editContentZone.show();
            $renderContent.hide();
            if ($textarea.val().length == 0) {
                $textarea.val($rawContent.text());
            }
            $textarea.focus();
            return false;
        });

        // Delete comment
        $('.delete-comment').click(function () {
            var $this = $(this);
            if (confirm($this.data('locale'))) {
                $.post($this.data('url'), {
                    "_csrf": csrf
                }).success(function () {
                    $('#' + $this.data('comment-id')).remove();
                });
            }
            return false;
        });

        // Change status
        var $statusButton = $('#status-button');
        $('#comment-form .edit_area').keyup(function () {
            if ($(this).val().length == 0) {
                $statusButton.text($statusButton.data('status'))
            } else {
                $statusButton.text($statusButton.data('status-and-comment'))
            }
        });
        $statusButton.click(function () {
            $('#status').val($statusButton.data('status-val'));
            $('#comment-form').submit();
        });
    }

    // Diff
    if ($('.repository.diff').length > 0) {
        var $counter = $('.diff-counter');
        if ($counter.length >= 1) {
            $counter.each(function (i, item) {
                var $item = $(item);
                var addLine = $item.find('span[data-line].add').data("line");
                var delLine = $item.find('span[data-line].del').data("line");
                var addPercent = parseFloat(addLine) / (parseFloat(addLine) + parseFloat(delLine)) * 100;
                $item.find(".bar .add").css("width", addPercent + "%");
            });
        }
    }

    // Quick start and repository home
    $('#repo-clone-ssh').click(function () {
        $('.clone-url').text($(this).data('link'));
        $('#repo-clone-url').val($(this).data('link'));
        $(this).addClass('blue');
        $('#repo-clone-https').removeClass('blue');
        localStorage.setItem('repo-clone-protocol', 'ssh');
    });
    $('#repo-clone-https').click(function () {
        $('.clone-url').text($(this).data('link'));
        $('#repo-clone-url').val($(this).data('link'));
        $(this).addClass('blue');
        $('#repo-clone-ssh').removeClass('blue');
        localStorage.setItem('repo-clone-protocol', 'https');
    });
    $('#repo-clone-url').click(function () {
        $(this).select();
    });

    // Pull request
    if ($('.repository.compare.pull').length > 0) {
        initFilterSearchDropdown('.choose.branch .dropdown');
    }
}

function initProtectedBranch() {
    $('#protectedBranch').change(function () {
        var $this = $(this);
        $.post($this.data('url'), {
                "_csrf": csrf,
                "canPush": true,
                "branchName": $this.val(),
            },
            function (data) {
                if (data.redirect) {
                    window.location.href = data.redirect;
                } else {
                    location.reload();
                }
            }
        );
    });

    $('.rm').click(function () {
        var $this = $(this);
        $.post($this.data('url'), {
                "_csrf": csrf,
                "canPush": false,
                "branchName": $this.data('val'),
            },
            function (data) {
                if (data.redirect) {
                    window.location.href = data.redirect;
                } else {
                    location.reload();
                }
            }
        );
    });
}

function initRepositoryCollaboration() {
    console.log('initRepositoryCollaboration');

    // Change collaborator access mode
    $('.access-mode.menu .item').click(function () {
        var $menu = $(this).parent();
        $.post($menu.data('url'), {
            "_csrf": csrf,
            "uid": $menu.data('uid'),
            "mode": $(this).data('value')
        })
    });
}

function initTeamSettings() {
    // Change team access mode
    $('.organization.new.team input[name=permission]').change(function () {
        var val = $('input[name=permission]:checked', '.organization.new.team').val()
        if (val === 'admin') {
            $('.organization.new.team .team-units').hide();
        } else {
            $('.organization.new.team .team-units').show();
        }
    });
}

function initWikiForm() {
    var $editArea = $('.repository.wiki textarea#edit_area');
    if ($editArea.length > 0) {
        new SimpleMDE({
            autoDownloadFontAwesome: false,
            element: $editArea[0],
            forceSync: true,
            previewRender: function (plainText, preview) { // Async method
                setTimeout(function () {
                    // FIXME: still send render request when return back to edit mode
                    $.post($editArea.data('url'), {
                            "_csrf": csrf,
                            "mode": "gfm",
                            "context": $editArea.data('context'),
                            "text": plainText
                        },
                        function (data) {
                            preview.innerHTML = '<div class="markdown">' + data + '</div>';
                            emojify.run($('.editor-preview')[0]);
                            $('.editor-preview').autolink();
                        }
                    );
                }, 0);

                return "Loading...";
            },
            renderingConfig: {
                singleLineBreaks: false
            },
            indentWithTabs: false,
            tabSize: 4,
            spellChecker: false,
            toolbar: ["bold", "italic", "strikethrough", "|",
                "heading-1", "heading-2", "heading-3", "heading-bigger", "heading-smaller", "|",
                "code", "quote", "|",
                "unordered-list", "ordered-list", "|",
                "link", "image", "table", "horizontal-rule", "|",
                "clean-block", "preview", "fullscreen"]
        })
    }
}

var simpleMDEditor;
var codeMirrorEditor;

// For IE
String.prototype.endsWith = function (pattern) {
    var d = this.length - pattern.length;
    return d >= 0 && this.lastIndexOf(pattern) === d;
};

// Adding function to get the cursor position in a text field to jQuery object.
(function ($, undefined) {
    $.fn.getCursorPosition = function () {
        var el = $(this).get(0);
        var pos = 0;
        if ('selectionStart' in el) {
            pos = el.selectionStart;
        } else if ('selection' in document) {
            el.focus();
            var Sel = document.selection.createRange();
            var SelLength = document.selection.createRange().text.length;
            Sel.moveStart('character', -el.value.length);
            pos = Sel.text.length - SelLength;
        }
        return pos;
    }
})(jQuery);


function setSimpleMDE($editArea) {
    if (codeMirrorEditor) {
        codeMirrorEditor.toTextArea();
        codeMirrorEditor = null;
    }

    if (simpleMDEditor) {
        return true;
    }

    simpleMDEditor = new SimpleMDE({
        autoDownloadFontAwesome: false,
        element: $editArea[0],
        forceSync: true,
        renderingConfig: {
            singleLineBreaks: false
        },
        indentWithTabs: false,
        tabSize: 4,
        spellChecker: false,
        previewRender: function (plainText, preview) { // Async method
            setTimeout(function () {
                // FIXME: still send render request when return back to edit mode
                $.post($editArea.data('url'), {
                        "_csrf": csrf,
                        "mode": "gfm",
                        "context": $editArea.data('context'),
                        "text": plainText
                    },
                    function (data) {
                        preview.innerHTML = '<div class="markdown">' + data + '</div>';
                        emojify.run($('.editor-preview')[0]);
                    }
                );
            }, 0);

            return "Loading...";
        },
        toolbar: ["bold", "italic", "strikethrough", "|",
            "heading-1", "heading-2", "heading-3", "heading-bigger", "heading-smaller", "|",
            "code", "quote", "|",
            "unordered-list", "ordered-list", "|",
            "link", "image", "table", "horizontal-rule", "|",
            "clean-block", "preview", "fullscreen", "side-by-side"]
    });

    return true;
}

function setCodeMirror($editArea) {
    if (simpleMDEditor) {
        simpleMDEditor.toTextArea();
        simpleMDEditor = null;
    }

    if (codeMirrorEditor) {
        return true;
    }

    codeMirrorEditor = CodeMirror.fromTextArea($editArea[0], {
        lineNumbers: true
    });
    codeMirrorEditor.on("change", function (cm, change) {
        $editArea.val(cm.getValue());
    });

    return true;
}

function initEditor() {
    $('.js-quick-pull-choice-option').change(function () {
        if ($(this).val() == 'commit-to-new-branch') {
            $('.quick-pull-branch-name').show();
            $('.quick-pull-branch-name input').prop('required',true);
        } else {
            $('.quick-pull-branch-name').hide();
            $('.quick-pull-branch-name input').prop('required',false);
        }
    });

    var $editFilename = $("#file-name");
    $editFilename.keyup(function (e) {
        var $section = $('.breadcrumb span.section');
        var $divider = $('.breadcrumb div.divider');
        if (e.keyCode == 8) {
            if ($(this).getCursorPosition() == 0) {
                if ($section.length > 0) {
                    var value = $section.last().find('a').text();
                    $(this).val(value + $(this).val());
                    $(this)[0].setSelectionRange(value.length, value.length);
                    $section.last().remove();
                    $divider.last().remove();
                }
            }
        }
        if (e.keyCode == 191) {
            var parts = $(this).val().split('/');
            for (var i = 0; i < parts.length; ++i) {
                var value = parts[i];
                if (i < parts.length - 1) {
                    if (value.length) {
                        $('<span class="section"><a href="#">' + value + '</a></span>').insertBefore($(this));
                        $('<div class="divider"> / </div>').insertBefore($(this));
                    }
                }
                else {
                    $(this).val(value);
                }
                $(this)[0].setSelectionRange(0, 0);
            }
        }
        var parts = [];
        $('.breadcrumb span.section').each(function (i, element) {
            element = $(element);
            if (element.find('a').length) {
                parts.push(element.find('a').text());
            } else {
                parts.push(element.text());
            }
        });
        if ($(this).val())
            parts.push($(this).val());
        $('#tree_path').val(parts.join('/'));
    }).trigger('keyup');

    var $editArea = $('.repository.editor textarea#edit_area');
    if (!$editArea.length)
        return;

    var markdownFileExts = $editArea.data("markdown-file-exts").split(",");
    var lineWrapExtensions = $editArea.data("line-wrap-extensions").split(",");

    $editFilename.on("keyup", function (e) {
        var val = $editFilename.val(), m, mode, spec, extension, extWithDot, previewLink, dataUrl, apiCall;
        extension = extWithDot = "";
        if (m = /.+\.([^.]+)$/.exec(val)) {
            extension = m[1];
            extWithDot = "." + extension;
        }

        var info = CodeMirror.findModeByExtension(extension);
        previewLink = $('a[data-tab=preview]');
        if (info) {
            mode = info.mode;
            spec = info.mime;
            apiCall = mode;
        }
        else {
            apiCall = extension
        }

        if (previewLink.length && apiCall && previewFileModes && previewFileModes.length && previewFileModes.indexOf(apiCall) >= 0) {
            dataUrl = previewLink.data('url');
            previewLink.data('url', dataUrl.replace(/(.*)\/.*/i, '$1/' + mode));
            previewLink.show();
        }
        else {
            previewLink.hide();
        }

        // If this file is a Markdown extensions, we will load that editor and return
        if (markdownFileExts.indexOf(extWithDot) >= 0) {
            if (setSimpleMDE($editArea)) {
                return;
            }
        }

        // Else we are going to use CodeMirror
        if (!codeMirrorEditor && !setCodeMirror($editArea)) {
            return;
        }

        if (mode) {
            codeMirrorEditor.setOption("mode", spec);
            CodeMirror.autoLoadMode(codeMirrorEditor, mode);
        }

        if (lineWrapExtensions.indexOf(extWithDot) >= 0) {
            codeMirrorEditor.setOption("lineWrapping", true);
        }
        else {
            codeMirrorEditor.setOption("lineWrapping", false);
        }

        // get the filename without any folder
        var value = $editFilename.val();
        if (value.length === 0) {
            return;
        }
        value = value.split('/');
        value = value[value.length - 1];

        $.getJSON($editFilename.data('ec-url-prefix')+value, function(editorconfig) {
            if (editorconfig.indent_style === 'tab') {
                codeMirrorEditor.setOption("indentWithTabs", true);
                codeMirrorEditor.setOption('extraKeys', {});
            } else {
                codeMirrorEditor.setOption("indentWithTabs", false);
                // required because CodeMirror doesn't seems to use spaces correctly for {"indentWithTabs": false}:
                // - https://github.com/codemirror/CodeMirror/issues/988
                // - https://codemirror.net/doc/manual.html#keymaps
                codeMirrorEditor.setOption('extraKeys', {
                    Tab: function(cm) {
                        var spaces = Array(parseInt(cm.getOption("indentUnit")) + 1).join(" ");
                        cm.replaceSelection(spaces);
                    }
                });
            }
            codeMirrorEditor.setOption("indentUnit", editorconfig.indent_size || 4);
            codeMirrorEditor.setOption("tabSize", editorconfig.tab_width || 4);
        });
    }).trigger('keyup');
}

function initOrganization() {
    if ($('.organization').length == 0) {
        return;
    }

    // Options
    if ($('.organization.settings.options').length > 0) {
        $('#org_name').keyup(function () {
            var $prompt = $('#org-name-change-prompt');
            if ($(this).val().toString().toLowerCase() != $(this).data('org-name').toString().toLowerCase()) {
                $prompt.show();
            } else {
                $prompt.hide();
            }
        });
    }
}

function initUserSettings() {
    console.log('initUserSettings');

    // Options
    if ($('.user.settings.profile').length > 0) {
        $('#username').keyup(function () {
            var $prompt = $('#name-change-prompt');
            if ($(this).val().toString().toLowerCase() != $(this).data('name').toString().toLowerCase()) {
                $prompt.show();
            } else {
                $prompt.hide();
            }
        });
    }
}

function initWebhook() {
    if ($('.new.webhook').length == 0) {
        return;
    }

    $('.events.checkbox input').change(function () {
        if ($(this).is(':checked')) {
            $('.events.fields').show();
        }
    });
    $('.non-events.checkbox input').change(function () {
        if ($(this).is(':checked')) {
            $('.events.fields').hide();
        }
    });

    // Test delivery
    $('#test-delivery').click(function () {
        var $this = $(this);
        $this.addClass('loading disabled');
        $.post($this.data('link'), {
            "_csrf": csrf
        }).done(
            setTimeout(function () {
                window.location.href = $this.data('redirect');
            }, 5000)
        )
    });
}

function initAdmin() {
    if ($('.admin').length == 0) {
        return;
    }

    // New user
    if ($('.admin.new.user').length > 0 ||
        $('.admin.edit.user').length > 0) {
        $('#login_type').change(function () {
            if ($(this).val().substring(0, 1) == '0') {
                $('#login_name').removeAttr('required');
                $('.non-local').hide();
                $('.local').show();
                $('#user_name').focus();

                if ($(this).data('password') == "required") {
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

    function onOAuth2Change() {
        $('.open_id_connect_auto_discovery_url, .oauth2_use_custom_url').hide();
        $('.open_id_connect_auto_discovery_url input[required]').removeAttr('required');

        var provider = $('#oauth2_provider').val();
        switch (provider) {
            case 'github':
            case 'gitlab':
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
        var provider = $('#oauth2_provider').val();
        $('.oauth2_use_custom_url_field').hide();
        $('.oauth2_use_custom_url_field input[required]').removeAttr('required');

        if ($('#oauth2_use_custom_url').is(':checked')) {
            if (!$('#oauth2_token_url').val()) {
                $('#oauth2_token_url').val($('#' + provider + '_token_url').val());
            }
            if (!$('#oauth2_auth_url').val()) {
                $('#oauth2_auth_url').val($('#' + provider + '_auth_url').val());
            }
            if (!$('#oauth2_profile_url').val()) {
                $('#oauth2_profile_url').val($('#' + provider + '_profile_url').val());
            }
            if (!$('#oauth2_email_url').val()) {
                $('#oauth2_email_url').val($('#' + provider + '_email_url').val());
            }
            switch (provider) {
                case 'github':
                    $('.oauth2_token_url input, .oauth2_auth_url input, .oauth2_profile_url input, .oauth2_email_url input').attr('required', 'required');
                    $('.oauth2_token_url, .oauth2_auth_url, .oauth2_profile_url, .oauth2_email_url').show();
                    break;
                case 'gitlab':
                    $('.oauth2_token_url input, .oauth2_auth_url input, .oauth2_profile_url input').attr('required', 'required');
                    $('.oauth2_token_url, .oauth2_auth_url, .oauth2_profile_url').show();
                    $('#oauth2_email_url').val('');
                    break;
            }
        }
    }

    // New authentication
    if ($('.admin.new.authentication').length > 0) {
        $('#auth_type').change(function () {
            $('.ldap, .dldap, .smtp, .pam, .oauth2, .has-tls').hide();

            $('.ldap input[required], .dldap input[required], .smtp input[required], .pam input[required], .oauth2 input[required] .has-tls input[required]').removeAttr('required');

            var authType = $(this).val();
            switch (authType) {
                case '2':     // LDAP
                    $('.ldap').show();
                    $('.ldap div.required:not(.dldap) input').attr('required', 'required');
                    break;
                case '3':     // SMTP
                    $('.smtp').show();
                    $('.has-tls').show();
                    $('.smtp div.required input, .has-tls').attr('required', 'required');
                    break;
                case '4':     // PAM
                    $('.pam').show();
                    $('.pam input').attr('required', 'required');
                    break;
                case '5':     // LDAP
                    $('.dldap').show();
                    $('.dldap div.required:not(.ldap) input').attr('required', 'required');
                    break;
                case '6':     // OAuth2
                    $('.oauth2').show();
                    $('.oauth2 div.required:not(.oauth2_use_custom_url,.oauth2_use_custom_url_field,.open_id_connect_auto_discovery_url) input').attr('required', 'required');
                    onOAuth2Change();
                    break;
            }
            if (authType == '2' || authType == '5') {
                onSecurityProtocolChange()
            }
        });
        $('#auth_type').change();
        $('#security_protocol').change(onSecurityProtocolChange);
        $('#oauth2_provider').change(onOAuth2Change);
        $('#oauth2_use_custom_url').change(onOAuth2UseCustomURLChange);
    }
    // Edit authentication
    if ($('.admin.edit.authentication').length > 0) {
        var authType = $('#auth_type').val();
        if (authType == '2' || authType == '5') {
            $('#security_protocol').change(onSecurityProtocolChange);
        } else if (authType == '6') {
            $('#oauth2_provider').change(onOAuth2Change);
            $('#oauth2_use_custom_url').change(onOAuth2UseCustomURLChange);
            onOAuth2Change();
        }
    }

    // Notice
    if ($('.admin.notice')) {
        var $detailModal = $('#detail-modal');

        // Attach view detail modals
        $('.view-detail').click(function () {
            $detailModal.find('.content p').text($(this).data('content'));
            $detailModal.modal('show');
            return false;
        });

        // Select actions
        var $checkboxes = $('.select.table .ui.checkbox');
        $('.select.action').click(function () {
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
        $('#delete-selection').click(function () {
            var $this = $(this);
            $this.addClass("loading disabled");
            var ids = [];
            $checkboxes.each(function () {
                if ($(this).checkbox('is checked')) {
                    ids.push($(this).data('id'));
                }
            });
            $.post($this.data('link'), {
                "_csrf": csrf,
                "ids": ids
            }).done(function () {
                window.location.href = $this.data('redirect');
            });
        });
    }
}

function buttonsClickOnEnter() {
    $('.ui.button').keypress(function (e) {
        if (e.keyCode == 13 || e.keyCode == 32) // enter key or space bar
            $(this).click();
    });
}

function hideWhenLostFocus(body, parent) {
    $(document).click(function (e) {
        var target = e.target;
        if (!$(target).is(body) && !$(target).parents().is(parent)) {
            $(body).hide();
        }
    });
}

function searchUsers() {
    if (!$('#search-user-box .results').length) {
        return;
    }

    var $searchUserBox = $('#search-user-box');
    var $results = $searchUserBox.find('.results');
    $searchUserBox.keyup(function () {
        var $this = $(this);
        var keyword = $this.find('input').val();
        if (keyword.length < 2) {
            $results.hide();
            return;
        }

        $.ajax({
            url: suburl + '/api/v1/users/search?q=' + keyword,
            dataType: "json",
            success: function (response) {
                var notEmpty = function (str) {
                    return str && str.length > 0;
                };

                $results.html('');

                if (response.ok && response.data.length) {
                    var html = '';
                    $.each(response.data, function (i, item) {
                        html += '<div class="item"><img class="ui avatar image" src="' + item.avatar_url + '"><span class="username">' + item.login + '</span>';
                        if (notEmpty(item.full_name)) {
                            html += ' (' + item.full_name + ')';
                        }
                        html += '</div>';
                    });
                    $results.html(html);
                    $this.find('.results .item').click(function () {
                        $this.find('input').val($(this).find('.username').text());
                        $results.hide();
                    });
                    $results.show();
                } else {
                    $results.hide();
                }
            }
        });
    });
    $searchUserBox.find('input').focus(function () {
        $searchUserBox.keyup();
    });
    hideWhenLostFocus('#search-user-box .results', '#search-user-box');
}

// FIXME: merge common parts in two functions
function searchRepositories() {
    if (!$('#search-repo-box .results').length) {
        return;
    }

    var $searchRepoBox = $('#search-repo-box');
    var $results = $searchRepoBox.find('.results');
    $searchRepoBox.keyup(function () {
        var $this = $(this);
        var keyword = $this.find('input').val();
        if (keyword.length < 2) {
            $results.hide();
            return;
        }

        $.ajax({
            url: suburl + '/api/v1/repos/search?q=' + keyword + "&uid=" + $searchRepoBox.data('uid'),
            dataType: "json",
            success: function (response) {
                var notEmpty = function (str) {
                    return str && str.length > 0;
                };

                $results.html('');

                if (response.ok && response.data.length) {
                    var html = '';
                    $.each(response.data, function (i, item) {
                        html += '<div class="item"><i class="icon octicon octicon-repo"></i> <span class="fullname">' + item.full_name + '</span></div>';
                    });
                    $results.html(html);
                    $this.find('.results .item').click(function () {
                        $this.find('input').val($(this).find('.fullname').text().split("/")[1]);
                        $results.hide();
                    });
                    $results.show();
                } else {
                    $results.hide();
                }
            }
        });
    });
    $searchRepoBox.find('input').focus(function () {
        $searchRepoBox.keyup();
    });
    hideWhenLostFocus('#search-repo-box .results', '#search-repo-box');
}

function initCodeView() {
    if ($('.code-view .linenums').length > 0) {
        $(document).on('click', '.lines-num span', function (e) {
            var $select = $(this);
            var $list = $select.parent().siblings('.lines-code').find('ol.linenums > li');
            selectRange($list, $list.filter('[rel=' + $select.attr('id') + ']'), (e.shiftKey ? $list.filter('.active').eq(0) : null));
            deSelect();
        });

        $(window).on('hashchange', function (e) {
            var m = window.location.hash.match(/^#(L\d+)\-(L\d+)$/);
            var $list = $('.code-view ol.linenums > li');
            var $first;
            if (m) {
                $first = $list.filter('.' + m[1]);
                selectRange($list, $first, $list.filter('.' + m[2]));
                $("html, body").scrollTop($first.offset().top - 200);
                return;
            }
            m = window.location.hash.match(/^#(L\d+)$/);
            if (m) {
                $first = $list.filter('.' + m[1]);
                selectRange($list, $first);
                $("html, body").scrollTop($first.offset().top - 200);
            }
        }).trigger('hashchange');
    }
}

$(document).ready(function () {
    csrf = $('meta[name=_csrf]').attr("content");
    suburl = $('meta[name=_suburl]').attr("content");

    // Show exact time
    $('.time-since').each(function () {
        $(this).addClass('poping up').attr('data-content', $(this).attr('title')).attr('data-variation', 'inverted tiny').attr('title', '');
    });

    // Semantic UI modules.
    $('.dropdown').dropdown();
    $('.jump.dropdown').dropdown({
        action: 'hide',
        onShow: function () {
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
        onShow: function () {
            if ($('.top.menu .menu.transition').hasClass('visible')) {
                return false;
            }
        }
    });
    $('.tabular.menu .item').tab();
    $('.tabable.menu .item').tab();

    $('.toggle.button').click(function () {
        $($(this).data('target')).slideToggle(100);
    });

    // make table <tr> element clickable like a link
    $('tr[data-href]').click(function(event) {
        window.location = $(this).data('href');
    });

    // Highlight JS
    if (typeof hljs != 'undefined') {
        hljs.initHighlightingOnLoad();
    }

    // Dropzone
    var $dropzone = $('#dropzone');
    if ($dropzone.length > 0) {
        // Disable auto discover for all elements:
        Dropzone.autoDiscover = false;

        var filenameDict = {};
        $dropzone.dropzone({
            url: $dropzone.data('upload-url'),
            headers: {"X-Csrf-Token": csrf},
            maxFiles: $dropzone.data('max-file'),
            maxFilesize: $dropzone.data('max-size'),
            acceptedFiles: ($dropzone.data('accepts') === '*/*') ? null : $dropzone.data('accepts'),
            addRemoveLinks: true,
            dictDefaultMessage: $dropzone.data('default-message'),
            dictInvalidFileType: $dropzone.data('invalid-input-type'),
            dictFileTooBig: $dropzone.data('file-too-big'),
            dictRemoveFile: $dropzone.data('remove-file'),
            init: function () {
                this.on("success", function (file, data) {
                    filenameDict[file.name] = data.uuid;
                    var input = $('<input id="' + data.uuid + '" name="files" type="hidden">').val(data.uuid);
                    $('.files').append(input);
                });
                this.on("removedfile", function (file) {
                    if (file.name in filenameDict) {
                        $('#' + filenameDict[file.name]).remove();
                    }
                    if ($dropzone.data('remove-url') && $dropzone.data('csrf')) {
                        $.post($dropzone.data('remove-url'), {
                            file: filenameDict[file.name],
                            _csrf: $dropzone.data('csrf')
                        });
                    }
                })
            }
        });
    }

    // Emojify
    emojify.setConfig({
        img_dir: suburl + '/img/emoji',
        ignore_emoticons: true
    });
    var hasEmoji = document.getElementsByClassName('has-emoji');
    for (var i = 0; i < hasEmoji.length; i++) {
        emojify.run(hasEmoji[i]);
    }

    // Clipboard JS
    var clipboard = new Clipboard('.clipboard');
    clipboard.on('success', function (e) {
        e.clearSelection();

        $('#' + e.trigger.getAttribute('id')).popup('destroy');
        e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-success'))
        $('#' + e.trigger.getAttribute('id')).popup('show');
        e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-original'))
    });

    clipboard.on('error', function (e) {
        $('#' + e.trigger.getAttribute('id')).popup('destroy');
        e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-error'))
        $('#' + e.trigger.getAttribute('id')).popup('show');
        e.trigger.setAttribute('data-content', e.trigger.getAttribute('data-original'))
    });

    // Helpers.
    $('.delete-button').click(function () {
        var $this = $(this);
        var filter = "";
        if ($this.attr("id")) {
          filter += "#"+$this.attr("id")
        }
        $('.delete.modal'+filter).modal({
            closable: false,
            onApprove: function () {
                if ($this.data('type') == "form") {
                    $($this.data('form')).submit();
                    return;
                }

                $.post($this.data('url'), {
                    "_csrf": csrf,
                    "id": $this.data("id")
                }).done(function (data) {
                    window.location.href = data.redirect;
                });
            }
        }).modal('show');
        return false;
    });
    $('.show-panel.button').click(function () {
        $($(this).data('panel')).show();
    });
    $('.show-modal.button').click(function () {
        $($(this).data('modal')).modal('show');
    });
    $('.delete-post.button').click(function () {
        var $this = $(this);
        $.post($this.data('request-url'), {
            "_csrf": csrf
        }).done(function () {
            window.location.href = $this.data('done-url');
        });
    });

    // Set anchor.
    $('.markdown').each(function () {
        var headers = {};
        $(this).find('h1, h2, h3, h4, h5, h6').each(function () {
            var node = $(this);
            var val = encodeURIComponent(node.text().toLowerCase().replace(/[^\u00C0-\u1FFF\u2C00-\uD7FF\w\- ]/g, '').replace(/[ ]/g, '-'));
            var name = val;
            if (headers[val] > 0) {
                name = val + '-' + headers[val];
            }
            if (headers[val] == undefined) {
                headers[val] = 1;
            } else {
                headers[val] += 1;
            }
            node = node.wrap('<div id="' + name + '" class="anchor-wrap" ></div>');
            node.append('<a class="anchor" href="#' + name + '"><span class="octicon octicon-link"></span></a>');
        });
    });
    $('.markdown').autolink();

    $('.issue-checkbox').click(function() {
        var numChecked = $('.issue-checkbox').children('input:checked').length;
        if (numChecked > 0) {
            $('.issue-filters').hide();
            $('.issue-actions').show();
        } else {
            $('.issue-filters').show();
            $('.issue-actions').hide();
        }
    });

    $('.issue-action').click(function () {
        var action = this.dataset.action
        var elementId = this.dataset.elementId
        var issueIDs = $('.issue-checkbox').children('input:checked').map(function() {
            return this.dataset.issueId;
        }).get().join();
        var url = this.dataset.url
        updateIssuesMeta(url, action, issueIDs, elementId, function() {
            location.reload();
        });
    });

    buttonsClickOnEnter();
    searchUsers();
    searchRepositories();

    initCommentForm();
    initInstall();
    initRepository();
    initWikiForm();
    initEditForm();
    initEditor();
    initOrganization();
    initProtectedBranch();
    initWebhook();
    initAdmin();
    initCodeView();
    initDashboardSearch();
    initTeamSettings();

    // Repo clone url.
    if ($('#repo-clone-url').length > 0) {
        switch (localStorage.getItem('repo-clone-protocol')) {
            case 'ssh':
                if ($('#repo-clone-ssh').click().length === 0) {
                    $('#repo-clone-https').click();
                }
                break;
            default:
                $('#repo-clone-https').click();
                break;
        }
    }

    var routes = {
        'div.user.settings': initUserSettings,
        'div.repository.settings.collaboration': initRepositoryCollaboration
    };

    var selector;
    for (selector in routes) {
        if ($(selector).length > 0) {
            routes[selector]();
            break;
        }
    }
});

function changeHash(hash) {
    if (history.pushState) {
        history.pushState(null, null, hash);
    }
    else {
        location.hash = hash;
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
        var a = parseInt($select.attr('rel').substr(1));
        var b = parseInt($from.attr('rel').substr(1));
        var c;
        if (a != b) {
            if (a > b) {
                c = a;
                a = b;
                b = c;
            }
            var classes = [];
            for (var i = a; i <= b; i++) {
                classes.push('.L' + i);
            }
            $list.filter(classes.join(',')).addClass('active');
            changeHash('#L' + a + '-' + 'L' + b);
            return
        }
    }
    $select.addClass('active');
    changeHash('#' + $select.attr('rel'));
}

$(function () {
    if ($('.user.signin').length > 0) return;
    $('form').areYouSure();

    // Parse SSH Key
    $("#ssh-key-content").on('change paste keyup',function(){
        var arrays = $(this).val().split(" ");
        var $title = $("#ssh-key-title")
        if ($title.val() === "" && arrays.length === 3 && arrays[2] !== "") {
            $title.val(arrays[2]);
        }
    });
});

function initDashboardSearch() {
    var el = document.getElementById('dashboard-repo-search');
    if (!el) {
        return;
    }

    new Vue({
        delimiters: ['${', '}'],
        el: el,

        data: {
            tab: 'repos',
            repos: [],
            searchQuery: '',
            suburl: document.querySelector('meta[name=_suburl]').content,
            uid: document.querySelector('meta[name=_context_uid]').content
        },

        mounted: function() {
            this.searchRepos();

            Vue.nextTick(function() {
                document.querySelector('#search_repo').focus();
            });
        },

        methods: {
            changeTab: function(t) {
                this.tab = t;
            },

            searchKeyUp: function() {
                this.searchRepos();
            },

            searchRepos: function() {
                var self = this;
                $.getJSON(this.searchURL(), function(result) {
                    self.repos = result.data;
                });
            },

            searchURL: function() {
                return this.suburl + '/api/v1/repos/search?uid=' + this.uid + '&q=' + this.searchQuery;
            },

            repoClass: function(repo) {
                if (repo.fork) {
                    return 'octicon octicon-repo-forked';
                } else if (repo.mirror) {
                    return 'octicon octicon-repo-clone';
                } else if (repo.private) {
                    return 'octicon octicon-lock';
                } else {
                    return 'octicon octicon-repo';
                }
            }
        }
    });
}
