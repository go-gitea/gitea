/*!
 * # Fomantic-UI - Form Validation
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

'use strict';

$.isFunction = $.isFunction || function(obj) {
  return typeof obj === "function" && typeof obj.nodeType !== "number";
};

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.form = function(parameters) {
  var
    $allModules      = $(this),
    moduleSelector   = $allModules.selector || '',

    time             = new Date().getTime(),
    performance      = [],

    query            = arguments[0],
    legacyParameters = arguments[1],
    methodInvoked    = (typeof query == 'string'),
    queryArguments   = [].slice.call(arguments, 1),
    returnedValue
  ;
  $allModules
    .each(function() {
      var
        $module     = $(this),
        element     = this,

        formErrors  = [],
        keyHeldDown = false,

        // set at run-time
        $field,
        $group,
        $message,
        $prompt,
        $submit,
        $clear,
        $reset,

        settings,
        validation,

        metadata,
        selector,
        className,
        regExp,
        error,

        namespace,
        moduleNamespace,
        eventNamespace,

        submitting = false,
        dirty = false,
        history = ['clean', 'clean'],

        instance,
        module
      ;

      module      = {

        initialize: function() {

          // settings grabbed at run time
          module.get.settings();
          if(methodInvoked) {
            if(instance === undefined) {
              module.instantiate();
            }
            module.invoke(query);
          }
          else {
            if(instance !== undefined) {
              instance.invoke('destroy');
            }
            module.verbose('Initializing form validation', $module, settings);
            module.bindEvents();
            module.set.defaults();
            if (settings.autoCheckRequired) {
              module.set.autoCheck();
            }
            module.instantiate();
          }
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous module', instance);
          module.removeEvents();
          $module
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          module.verbose('Refreshing selector cache');
          $field      = $module.find(selector.field);
          $group      = $module.find(selector.group);
          $message    = $module.find(selector.message);
          $prompt     = $module.find(selector.prompt);

          $submit     = $module.find(selector.submit);
          $clear      = $module.find(selector.clear);
          $reset      = $module.find(selector.reset);
        },

        submit: function() {
          module.verbose('Submitting form', $module);
          submitting = true;
          $module.submit();
        },

        attachEvents: function(selector, action) {
          action = action || 'submit';
          $(selector).on('click' + eventNamespace, function(event) {
            module[action]();
            event.preventDefault();
          });
        },

        bindEvents: function() {
          module.verbose('Attaching form events');
          $module
            .on('submit' + eventNamespace, module.validate.form)
            .on('blur'   + eventNamespace, selector.field, module.event.field.blur)
            .on('click'  + eventNamespace, selector.submit, module.submit)
            .on('click'  + eventNamespace, selector.reset, module.reset)
            .on('click'  + eventNamespace, selector.clear, module.clear)
          ;
          if(settings.keyboardShortcuts) {
            $module.on('keydown' + eventNamespace, selector.field, module.event.field.keydown);
          }
          $field.each(function(index, el) {
            var
              $input     = $(el),
              type       = $input.prop('type'),
              inputEvent = module.get.changeEvent(type, $input)
            ;
            $input.on(inputEvent + eventNamespace, module.event.field.change);
          });

          // Dirty events
          if (settings.preventLeaving) {
            $(window).on('beforeunload' + eventNamespace, module.event.beforeUnload);
          }

          $field.on('change click keyup keydown blur', function(e) {
            $(this).triggerHandler(e.type + ".dirty");
          });

          $field.on('change.dirty click.dirty keyup.dirty keydown.dirty blur.dirty', module.determine.isDirty);

          $module.on('dirty' + eventNamespace, function(e) {
            settings.onDirty.call();
          });

          $module.on('clean' + eventNamespace, function(e) {
            settings.onClean.call();
          })
        },

        clear: function() {
          $field.each(function (index, el) {
            var
              $field       = $(el),
              $element     = $field.parent(),
              $fieldGroup  = $field.closest($group),
              $prompt      = $fieldGroup.find(selector.prompt),
              $calendar    = $field.closest(selector.uiCalendar),
              defaultValue = $field.data(metadata.defaultValue) || '',
              isCheckbox   = $element.is(selector.uiCheckbox),
              isDropdown   = $element.is(selector.uiDropdown)  && module.can.useElement('dropdown'),
              isCalendar   = ($calendar.length > 0  && module.can.useElement('calendar')),
              isErrored    = $fieldGroup.hasClass(className.error)
            ;
            if(isErrored) {
              module.verbose('Resetting error on field', $fieldGroup);
              $fieldGroup.removeClass(className.error);
              $prompt.remove();
            }
            if(isDropdown) {
              module.verbose('Resetting dropdown value', $element, defaultValue);
              $element.dropdown('clear', true);
            }
            else if(isCheckbox) {
              $field.prop('checked', false);
            }
            else if (isCalendar) {
              $calendar.calendar('clear');
            }
            else {
              module.verbose('Resetting field value', $field, defaultValue);
              $field.val('');
            }
          });
          module.remove.states();
        },

        reset: function() {
          $field.each(function (index, el) {
            var
              $field       = $(el),
              $element     = $field.parent(),
              $fieldGroup  = $field.closest($group),
              $calendar    = $field.closest(selector.uiCalendar),
              $prompt      = $fieldGroup.find(selector.prompt),
              defaultValue = $field.data(metadata.defaultValue),
              isCheckbox   = $element.is(selector.uiCheckbox),
              isDropdown   = $element.is(selector.uiDropdown)  && module.can.useElement('dropdown'),
              isCalendar   = ($calendar.length > 0  && module.can.useElement('calendar')),
              isErrored    = $fieldGroup.hasClass(className.error)
            ;
            if(defaultValue === undefined) {
              return;
            }
            if(isErrored) {
              module.verbose('Resetting error on field', $fieldGroup);
              $fieldGroup.removeClass(className.error);
              $prompt.remove();
            }
            if(isDropdown) {
              module.verbose('Resetting dropdown value', $element, defaultValue);
              $element.dropdown('restore defaults', true);
            }
            else if(isCheckbox) {
              module.verbose('Resetting checkbox value', $element, defaultValue);
              $field.prop('checked', defaultValue);
            }
            else if (isCalendar) {
              $calendar.calendar('set date', defaultValue);
            }
            else {
              module.verbose('Resetting field value', $field, defaultValue);
              $field.val(defaultValue);
            }
          });
          module.remove.states();
        },

        determine: {
          isValid: function() {
            var
              allValid = true
            ;
            $.each(validation, function(fieldName, field) {
              if( !( module.validate.field(field, fieldName, true) ) ) {
                allValid = false;
              }
            });
            return allValid;
          },
          isDirty: function(e) {
            var formIsDirty = false;

            $field.each(function(index, el) {
              var
                $el = $(el),
                isCheckbox = ($el.filter(selector.checkbox).length > 0),
                isDirty
              ;

              if (isCheckbox) {
                isDirty = module.is.checkboxDirty($el);
              } else {
                isDirty = module.is.fieldDirty($el);
              }

              $el.data(settings.metadata.isDirty, isDirty);

              formIsDirty |= isDirty;
            });

            if (formIsDirty) {
              module.set.dirty();
            } else {
              module.set.clean();
            }

            if (e && e.namespace === 'dirty') {
              e.stopImmediatePropagation();
              e.preventDefault();
            }
          }
        },

        is: {
          bracketedRule: function(rule) {
            return (rule.type && rule.type.match(settings.regExp.bracket));
          },
          shorthandFields: function(fields) {
            var
              fieldKeys = Object.keys(fields),
              firstRule = fields[fieldKeys[0]]
            ;
            return module.is.shorthandRules(firstRule);
          },
          // duck type rule test
          shorthandRules: function(rules) {
            return (typeof rules == 'string' || Array.isArray(rules));
          },
          empty: function($field) {
            if(!$field || $field.length === 0) {
              return true;
            }
            else if($field.is(selector.checkbox)) {
              return !$field.is(':checked');
            }
            else {
              return module.is.blank($field);
            }
          },
          blank: function($field) {
            return String($field.val()).trim() === '';
          },
          valid: function(field, showErrors) {
            var
              allValid = true
            ;
            if(field) {
              module.verbose('Checking if field is valid', field);
              return module.validate.field(validation[field], field, !!showErrors);
            }
            else {
              module.verbose('Checking if form is valid');
              $.each(validation, function(fieldName, field) {
                if( !module.is.valid(fieldName, showErrors) ) {
                  allValid = false;
                }
              });
              return allValid;
            }
          },
          dirty: function() {
            return dirty;
          },
          clean: function() {
            return !dirty;
          },
          fieldDirty: function($el) {
            var initialValue = $el.data(metadata.defaultValue);
            // Explicitly check for null/undefined here as value may be `false`, so ($el.data(dataInitialValue) || '') would not work
            if (initialValue == null) { initialValue = ''; }
            else if(Array.isArray(initialValue)) {
              initialValue = initialValue.toString();
            }
            var currentValue = $el.val();
            if (currentValue == null) { currentValue = ''; }
            // multiple select values are returned as arrays which are never equal, so do string conversion first
            else if(Array.isArray(currentValue)) {
              currentValue = currentValue.toString();
            }
            // Boolean values can be encoded as "true/false" or "True/False" depending on underlying frameworks so we need a case insensitive comparison
            var boolRegex = /^(true|false)$/i;
            var isBoolValue = boolRegex.test(initialValue) && boolRegex.test(currentValue);
            if (isBoolValue) {
              var regex = new RegExp("^" + initialValue + "$", "i");
              return !regex.test(currentValue);
            }

            return currentValue !== initialValue;
          },
          checkboxDirty: function($el) {
            var initialValue = $el.data(metadata.defaultValue);
            var currentValue = $el.is(":checked");

            return initialValue !== currentValue;
          },
          justDirty: function() {
            return (history[0] === 'dirty');
          },
          justClean: function() {
            return (history[0] === 'clean');
          }
        },

        removeEvents: function() {
          $module.off(eventNamespace);
          $field.off(eventNamespace);
          $submit.off(eventNamespace);
          $field.off(eventNamespace);
        },

        event: {
          field: {
            keydown: function(event) {
              var
                $field       = $(this),
                key          = event.which,
                isInput      = $field.is(selector.input),
                isCheckbox   = $field.is(selector.checkbox),
                isInDropdown = ($field.closest(selector.uiDropdown).length > 0),
                keyCode      = {
                  enter  : 13,
                  escape : 27
                }
              ;
              if( key == keyCode.escape) {
                module.verbose('Escape key pressed blurring field');
                $field
                  .blur()
                ;
              }
              if(!event.ctrlKey && key == keyCode.enter && isInput && !isInDropdown && !isCheckbox) {
                if(!keyHeldDown) {
                  $field.one('keyup' + eventNamespace, module.event.field.keyup);
                  module.submit();
                  module.debug('Enter pressed on input submitting form');
                }
                keyHeldDown = true;
              }
            },
            keyup: function() {
              keyHeldDown = false;
            },
            blur: function(event) {
              var
                $field          = $(this),
                $fieldGroup     = $field.closest($group),
                validationRules = module.get.validation($field)
              ;
              if( $fieldGroup.hasClass(className.error) ) {
                module.debug('Revalidating field', $field, validationRules);
                if(validationRules) {
                  module.validate.field( validationRules );
                }
              }
              else if(settings.on == 'blur') {
                if(validationRules) {
                  module.validate.field( validationRules );
                }
              }
            },
            change: function(event) {
              var
                $field      = $(this),
                $fieldGroup = $field.closest($group),
                validationRules = module.get.validation($field)
              ;
              if(validationRules && (settings.on == 'change' || ( $fieldGroup.hasClass(className.error) && settings.revalidate) )) {
                clearTimeout(module.timer);
                module.timer = setTimeout(function() {
                  module.debug('Revalidating field', $field,  module.get.validation($field));
                  module.validate.field( validationRules );
                  if(!settings.inline) {
                    module.validate.form(false,true);
                  }
                }, settings.delay);
              }
            }
          },
          beforeUnload: function(event) {
            if (module.is.dirty() && !submitting) {
              var event = event || window.event;

              // For modern browsers
              if (event) {
                event.returnValue = settings.text.leavingMessage;
              }

              // For olders...
              return settings.text.leavingMessage;
            }
          }

        },

        get: {
          ancillaryValue: function(rule) {
            if(!rule.type || (!rule.value && !module.is.bracketedRule(rule))) {
              return false;
            }
            return (rule.value !== undefined)
              ? rule.value
              : rule.type.match(settings.regExp.bracket)[1] + ''
            ;
          },
          ruleName: function(rule) {
            if( module.is.bracketedRule(rule) ) {
              return rule.type.replace(rule.type.match(settings.regExp.bracket)[0], '');
            }
            return rule.type;
          },
          changeEvent: function(type, $input) {
            if(type == 'checkbox' || type == 'radio' || type == 'hidden' || $input.is('select')) {
              return 'change';
            }
            else {
              return module.get.inputEvent();
            }
          },
          inputEvent: function() {
            return (document.createElement('input').oninput !== undefined)
              ? 'input'
              : (document.createElement('input').onpropertychange !== undefined)
                ? 'propertychange'
                : 'keyup'
            ;
          },
          fieldsFromShorthand: function(fields) {
            var
              fullFields = {}
            ;
            $.each(fields, function(name, rules) {
              if(typeof rules == 'string') {
                rules = [rules];
              }
              fullFields[name] = {
                rules: []
              };
              $.each(rules, function(index, rule) {
                fullFields[name].rules.push({ type: rule });
              });
            });
            return fullFields;
          },
          prompt: function(rule, field) {
            var
              ruleName      = module.get.ruleName(rule),
              ancillary     = module.get.ancillaryValue(rule),
              $field        = module.get.field(field.identifier),
              value         = $field.val(),
              prompt        = $.isFunction(rule.prompt)
                ? rule.prompt(value)
                : rule.prompt || settings.prompt[ruleName] || settings.text.unspecifiedRule,
              requiresValue = (prompt.search('{value}') !== -1),
              requiresName  = (prompt.search('{name}') !== -1),
              $label,
              name
            ;
            if(requiresValue) {
              prompt = prompt.replace(/\{value\}/g, $field.val());
            }
            if(requiresName) {
              $label = $field.closest(selector.group).find('label').eq(0);
              name = ($label.length == 1)
                ? $label.text()
                : $field.prop('placeholder') || settings.text.unspecifiedField
              ;
              prompt = prompt.replace(/\{name\}/g, name);
            }
            prompt = prompt.replace(/\{identifier\}/g, field.identifier);
            prompt = prompt.replace(/\{ruleValue\}/g, ancillary);
            if(!rule.prompt) {
              module.verbose('Using default validation prompt for type', prompt, ruleName);
            }
            return prompt;
          },
          settings: function() {
            if($.isPlainObject(parameters)) {
              var
                keys     = Object.keys(parameters),
                isLegacySettings = (keys.length > 0)
                  ? (parameters[keys[0]].identifier !== undefined && parameters[keys[0]].rules !== undefined)
                  : false
              ;
              if(isLegacySettings) {
                // 1.x (ducktyped)
                settings   = $.extend(true, {}, $.fn.form.settings, legacyParameters);
                validation = $.extend({}, $.fn.form.settings.defaults, parameters);
                module.error(settings.error.oldSyntax, element);
                module.verbose('Extending settings from legacy parameters', validation, settings);
              }
              else {
                // 2.x
                if(parameters.fields && module.is.shorthandFields(parameters.fields)) {
                  parameters.fields = module.get.fieldsFromShorthand(parameters.fields);
                }
                settings   = $.extend(true, {}, $.fn.form.settings, parameters);
                validation = $.extend({}, $.fn.form.settings.defaults, settings.fields);
                module.verbose('Extending settings', validation, settings);
              }
            }
            else {
              settings   = $.fn.form.settings;
              validation = $.fn.form.settings.defaults;
              module.verbose('Using default form validation', validation, settings);
            }

            // shorthand
            namespace       = settings.namespace;
            metadata        = settings.metadata;
            selector        = settings.selector;
            className       = settings.className;
            regExp          = settings.regExp;
            error           = settings.error;
            moduleNamespace = 'module-' + namespace;
            eventNamespace  = '.' + namespace;

            // grab instance
            instance = $module.data(moduleNamespace);

            // refresh selector cache
            module.refresh();
          },
          field: function(identifier) {
            module.verbose('Finding field with identifier', identifier);
            identifier = module.escape.string(identifier);
            var t;
            if((t=$field.filter('#' + identifier)).length > 0 ) {
              return t;
            }
            if((t=$field.filter('[name="' + identifier +'"]')).length > 0 ) {
              return t;
            }
            if((t=$field.filter('[name="' + identifier +'[]"]')).length > 0 ) {
              return t;
            }
            if((t=$field.filter('[data-' + metadata.validate + '="'+ identifier +'"]')).length > 0 ) {
              return t;
            }
            return $('<input/>');
          },
          fields: function(fields) {
            var
              $fields = $()
            ;
            $.each(fields, function(index, name) {
              $fields = $fields.add( module.get.field(name) );
            });
            return $fields;
          },
          validation: function($field) {
            var
              fieldValidation,
              identifier
            ;
            if(!validation) {
              return false;
            }
            $.each(validation, function(fieldName, field) {
              identifier = field.identifier || fieldName;
              $.each(module.get.field(identifier), function(index, groupField) {
                if(groupField == $field[0]) {
                  field.identifier = identifier;
                  fieldValidation = field;
                  return false;
                }
              });
            });
            return fieldValidation || false;
          },
          value: function (field) {
            var
              fields = [],
              results
            ;
            fields.push(field);
            results = module.get.values.call(element, fields);
            return results[field];
          },
          values: function (fields) {
            var
              $fields = Array.isArray(fields)
                ? module.get.fields(fields)
                : $field,
              values = {}
            ;
            $fields.each(function(index, field) {
              var
                $field       = $(field),
                $calendar    = $field.closest(selector.uiCalendar),
                name         = $field.prop('name'),
                value        = $field.val(),
                isCheckbox   = $field.is(selector.checkbox),
                isRadio      = $field.is(selector.radio),
                isMultiple   = (name.indexOf('[]') !== -1),
                isCalendar   = ($calendar.length > 0  && module.can.useElement('calendar')),
                isChecked    = (isCheckbox)
                  ? $field.is(':checked')
                  : false
              ;
              if(name) {
                if(isMultiple) {
                  name = name.replace('[]', '');
                  if(!values[name]) {
                    values[name] = [];
                  }
                  if(isCheckbox) {
                    if(isChecked) {
                      values[name].push(value || true);
                    }
                    else {
                      values[name].push(false);
                    }
                  }
                  else {
                    values[name].push(value);
                  }
                }
                else {
                  if(isRadio) {
                    if(values[name] === undefined || values[name] === false) {
                      values[name] = (isChecked)
                        ? value || true
                        : false
                      ;
                    }
                  }
                  else if(isCheckbox) {
                    if(isChecked) {
                      values[name] = value || true;
                    }
                    else {
                      values[name] = false;
                    }
                  }
                  else if(isCalendar) {
                    var date = $calendar.calendar('get date');

                    if (date !== null) {
                      if (settings.dateHandling == 'date') {
                        values[name] = date;
                      } else if(settings.dateHandling == 'input') {
                        values[name] = $calendar.calendar('get input date')
                      } else if (settings.dateHandling == 'formatter') {
                        var type = $calendar.calendar('setting', 'type');

                        switch(type) {
                          case 'date':
                          values[name] = settings.formatter.date(date);
                          break;

                          case 'datetime':
                          values[name] = settings.formatter.datetime(date);
                          break;

                          case 'time':
                          values[name] = settings.formatter.time(date);
                          break;

                          case 'month':
                          values[name] = settings.formatter.month(date);
                          break;

                          case 'year':
                          values[name] = settings.formatter.year(date);
                          break;

                          default:
                          module.debug('Wrong calendar mode', $calendar, type);
                          values[name] = '';
                        }
                      }
                    } else {
                      values[name] = '';
                    }
                  } else {
                    values[name] = value;
                  }
                }
              }
            });
            return values;
          },
          dirtyFields: function() {
            return $field.filter(function(index, e) {
              return $(e).data(metadata.isDirty);
            });
          }
        },

        has: {

          field: function(identifier) {
            module.verbose('Checking for existence of a field with identifier', identifier);
            identifier = module.escape.string(identifier);
            if(typeof identifier !== 'string') {
              module.error(error.identifier, identifier);
            }
            if($field.filter('#' + identifier).length > 0 ) {
              return true;
            }
            else if( $field.filter('[name="' + identifier +'"]').length > 0 ) {
              return true;
            }
            else if( $field.filter('[data-' + metadata.validate + '="'+ identifier +'"]').length > 0 ) {
              return true;
            }
            return false;
          }

        },

        can: {
            useElement: function(element){
               if ($.fn[element] !== undefined) {
                   return true;
               }
               module.error(error.noElement.replace('{element}',element));
               return false;
            }
        },

        escape: {
          string: function(text) {
            text =  String(text);
            return text.replace(regExp.escape, '\\$&');
          }
        },

        add: {
          // alias
          rule: function(name, rules) {
            module.add.field(name, rules);
          },
          field: function(name, rules) {
            // Validation should have at least a standard format
            if(validation[name] === undefined || validation[name].rules === undefined) {
              validation[name] = {
                rules: []
              };
            }
            var
              newValidation = {
                rules: []
              }
            ;
            if(module.is.shorthandRules(rules)) {
              rules = Array.isArray(rules)
                ? rules
                : [rules]
              ;
              $.each(rules, function(_index, rule) {
                newValidation.rules.push({ type: rule });
              });
            }
            else {
              newValidation.rules = rules.rules;
            }
            // For each new rule, check if there's not already one with the same type
            $.each(newValidation.rules, function (_index, rule) {
              if ($.grep(validation[name].rules, function(item){ return item.type == rule.type; }).length == 0) {
                validation[name].rules.push(rule);
              }
            });
            module.debug('Adding rules', newValidation.rules, validation);
          },
          fields: function(fields) {
            var
              newValidation
            ;
            if(fields && module.is.shorthandFields(fields)) {
              newValidation = module.get.fieldsFromShorthand(fields);
            }
            else {
              newValidation = fields;
            }
            validation = $.extend({}, validation, newValidation);
          },
          prompt: function(identifier, errors, internal) {
            var
              $field       = module.get.field(identifier),
              $fieldGroup  = $field.closest($group),
              $prompt      = $fieldGroup.children(selector.prompt),
              promptExists = ($prompt.length !== 0)
            ;
            errors = (typeof errors == 'string')
              ? [errors]
              : errors
            ;
            module.verbose('Adding field error state', identifier);
            if(!internal) {
              $fieldGroup
                  .addClass(className.error)
              ;
            }
            if(settings.inline) {
              if(!promptExists) {
                $prompt = settings.templates.prompt(errors, className.label);
                $prompt
                  .appendTo($fieldGroup)
                ;
              }
              $prompt
                .html(errors[0])
              ;
              if(!promptExists) {
                if(settings.transition && module.can.useElement('transition') && $module.transition('is supported')) {
                  module.verbose('Displaying error with css transition', settings.transition);
                  $prompt.transition(settings.transition + ' in', settings.duration);
                }
                else {
                  module.verbose('Displaying error with fallback javascript animation');
                  $prompt
                    .fadeIn(settings.duration)
                  ;
                }
              }
              else {
                module.verbose('Inline errors are disabled, no inline error added', identifier);
              }
            }
          },
          errors: function(errors) {
            module.debug('Adding form error messages', errors);
            module.set.error();
            $message
              .html( settings.templates.error(errors) )
            ;
          }
        },

        remove: {
          errors: function() {
            module.debug('Removing form error messages');
            $message.empty();
          },
          states: function() {
            $module.removeClass(className.error).removeClass(className.success);
            if(!settings.inline) {
              module.remove.errors();
            }
            module.determine.isDirty();
          },
          rule: function(field, rule) {
            var
              rules = Array.isArray(rule)
                ? rule
                : [rule]
            ;
            if(validation[field] === undefined || !Array.isArray(validation[field].rules)) {
              return;
            }
            if(rule === undefined) {
              module.debug('Removed all rules');
              validation[field].rules = [];
              return;
            }
            $.each(validation[field].rules, function(index, rule) {
              if(rule && rules.indexOf(rule.type) !== -1) {
                module.debug('Removed rule', rule.type);
                validation[field].rules.splice(index, 1);
              }
            });
          },
          field: function(field) {
            var
              fields = Array.isArray(field)
                ? field
                : [field]
            ;
            $.each(fields, function(index, field) {
              module.remove.rule(field);
            });
          },
          // alias
          rules: function(field, rules) {
            if(Array.isArray(field)) {
              $.each(field, function(index, field) {
                module.remove.rule(field, rules);
              });
            }
            else {
              module.remove.rule(field, rules);
            }
          },
          fields: function(fields) {
            module.remove.field(fields);
          },
          prompt: function(identifier) {
            var
              $field      = module.get.field(identifier),
              $fieldGroup = $field.closest($group),
              $prompt     = $fieldGroup.children(selector.prompt)
            ;
            $fieldGroup
              .removeClass(className.error)
            ;
            if(settings.inline && $prompt.is(':visible')) {
              module.verbose('Removing prompt for field', identifier);
              if(settings.transition  && module.can.useElement('transition') && $module.transition('is supported')) {
                $prompt.transition(settings.transition + ' out', settings.duration, function() {
                  $prompt.remove();
                });
              }
              else {
                $prompt
                  .fadeOut(settings.duration, function(){
                    $prompt.remove();
                  })
                ;
              }
            }
          }
        },

        set: {
          success: function() {
            $module
              .removeClass(className.error)
              .addClass(className.success)
            ;
          },
          defaults: function () {
            $field.each(function (index, el) {
              var
                $el        = $(el),
                $parent    = $el.parent(),
                isCheckbox = ($el.filter(selector.checkbox).length > 0),
                isDropdown = $parent.is(selector.uiDropdown) && module.can.useElement('dropdown'),
                $calendar   = $el.closest(selector.uiCalendar),
                isCalendar  = ($calendar.length > 0  && module.can.useElement('calendar')),
                value      = (isCheckbox)
                  ? $el.is(':checked')
                  : $el.val()
              ;
              if (isDropdown) {
                $parent.dropdown('save defaults');
              }
              else if (isCalendar) {
                $calendar.calendar('refresh');
              }
              $el.data(metadata.defaultValue, value);
              $el.data(metadata.isDirty, false);
            });
          },
          error: function() {
            $module
              .removeClass(className.success)
              .addClass(className.error)
            ;
          },
          value: function (field, value) {
            var
              fields = {}
            ;
            fields[field] = value;
            return module.set.values.call(element, fields);
          },
          values: function (fields) {
            if($.isEmptyObject(fields)) {
              return;
            }
            $.each(fields, function(key, value) {
              var
                $field      = module.get.field(key),
                $element    = $field.parent(),
                $calendar   = $field.closest(selector.uiCalendar),
                isMultiple  = Array.isArray(value),
                isCheckbox  = $element.is(selector.uiCheckbox)  && module.can.useElement('checkbox'),
                isDropdown  = $element.is(selector.uiDropdown) && module.can.useElement('dropdown'),
                isRadio     = ($field.is(selector.radio) && isCheckbox),
                isCalendar  = ($calendar.length > 0  && module.can.useElement('calendar')),
                fieldExists = ($field.length > 0),
                $multipleField
              ;
              if(fieldExists) {
                if(isMultiple && isCheckbox) {
                  module.verbose('Selecting multiple', value, $field);
                  $element.checkbox('uncheck');
                  $.each(value, function(index, value) {
                    $multipleField = $field.filter('[value="' + value + '"]');
                    $element       = $multipleField.parent();
                    if($multipleField.length > 0) {
                      $element.checkbox('check');
                    }
                  });
                }
                else if(isRadio) {
                  module.verbose('Selecting radio value', value, $field);
                  $field.filter('[value="' + value + '"]')
                    .parent(selector.uiCheckbox)
                      .checkbox('check')
                  ;
                }
                else if(isCheckbox) {
                  module.verbose('Setting checkbox value', value, $element);
                  if(value === true || value === 1) {
                    $element.checkbox('check');
                  }
                  else {
                    $element.checkbox('uncheck');
                  }
                }
                else if(isDropdown) {
                  module.verbose('Setting dropdown value', value, $element);
                  $element.dropdown('set selected', value);
                }
                else if (isCalendar) {
                  $calendar.calendar('set date',value);
                }
                else {
                  module.verbose('Setting field value', value, $field);
                  $field.val(value);
                }
              }
            });
          },
          dirty: function() {
            module.verbose('Setting state dirty');
            dirty = true;
            history[0] = history[1];
            history[1] = 'dirty';

            if (module.is.justClean()) {
              $module.trigger('dirty');
            }
          },
          clean: function() {
            module.verbose('Setting state clean');
            dirty = false;
            history[0] = history[1];
            history[1] = 'clean';

            if (module.is.justDirty()) {
              $module.trigger('clean');
            }
          },
          asClean: function() {
            module.set.defaults();
            module.set.clean();
          },
          asDirty: function() {
            module.set.defaults();
            module.set.dirty();
          },
          autoCheck: function() {
            module.debug('Enabling auto check on required fields');
            $field.each(function (_index, el) {
              var
                $el        = $(el),
                $elGroup   = $(el).closest($group),
                isCheckbox = ($el.filter(selector.checkbox).length > 0),
                isRequired = $el.prop('required') || $elGroup.hasClass(className.required) || $elGroup.parent().hasClass(className.required),
                isDisabled = $el.is(':disabled') || $elGroup.hasClass(className.disabled) || $elGroup.parent().hasClass(className.disabled),
                validation = module.get.validation($el),
                hasEmptyRule = validation
                  ? $.grep(validation.rules, function(rule) { return rule.type == "empty" }) !== 0
                  : false,
                identifier = validation.identifier || $el.attr('id') || $el.attr('name') || $el.data(metadata.validate)
              ;
              if (isRequired && !isDisabled && !hasEmptyRule && identifier !== undefined) {
                if (isCheckbox) {
                  module.verbose("Adding 'checked' rule on field", identifier);
                  module.add.rule(identifier, "checked");
                } else {
                  module.verbose("Adding 'empty' rule on field", identifier);
                  module.add.rule(identifier, "empty");
                }
              }
            });
          }
        },

        validate: {

          form: function(event, ignoreCallbacks) {
            var values = module.get.values();

            // input keydown event will fire submit repeatedly by browser default
            if(keyHeldDown) {
              return false;
            }

            // reset errors
            formErrors = [];
            if( module.determine.isValid() ) {
              module.debug('Form has no validation errors, submitting');
              module.set.success();
              if(!settings.inline) {
                module.remove.errors();
              }
              if(ignoreCallbacks !== true) {
                return settings.onSuccess.call(element, event, values);
              }
            }
            else {
              module.debug('Form has errors');
              submitting = false;
              module.set.error();
              if(!settings.inline) {
                module.add.errors(formErrors);
              }
              // prevent ajax submit
              if(event && $module.data('moduleApi') !== undefined) {
                event.stopImmediatePropagation();
              }
              if(ignoreCallbacks !== true) {
                return settings.onFailure.call(element, formErrors, values);
              }
            }
          },

          // takes a validation object and returns whether field passes validation
          field: function(field, fieldName, showErrors) {
            showErrors = (showErrors !== undefined)
              ? showErrors
              : true
            ;
            if(typeof field == 'string') {
              module.verbose('Validating field', field);
              fieldName = field;
              field     = validation[field];
            }
            var
              identifier    = field.identifier || fieldName,
              $field        = module.get.field(identifier),
              $dependsField = (field.depends)
                ? module.get.field(field.depends)
                : false,
              fieldValid  = true,
              fieldErrors = []
            ;
            if(!field.identifier) {
              module.debug('Using field name as identifier', identifier);
              field.identifier = identifier;
            }
            var isDisabled = !$field.filter(':not(:disabled)').length;
            if(isDisabled) {
              module.debug('Field is disabled. Skipping', identifier);
            }
            else if(field.optional && module.is.blank($field)){
              module.debug('Field is optional and blank. Skipping', identifier);
            }
            else if(field.depends && module.is.empty($dependsField)) {
              module.debug('Field depends on another value that is not present or empty. Skipping', $dependsField);
            }
            else if(field.rules !== undefined) {
              if(showErrors) {
                $field.closest($group).removeClass(className.error);
              }
              $.each(field.rules, function(index, rule) {
                if( module.has.field(identifier)) {
                  var invalidFields = module.validate.rule(field, rule,true) || [];
                  if (invalidFields.length>0){
                    module.debug('Field is invalid', identifier, rule.type);
                    fieldErrors.push(module.get.prompt(rule, field));
                    fieldValid = false;
                    if(showErrors){
                      $(invalidFields).closest($group).addClass(className.error);
                    }
                  }
                }
              });
            }
            if(fieldValid) {
              if(showErrors) {
                module.remove.prompt(identifier, fieldErrors);
                settings.onValid.call($field);
              }
            }
            else {
              if(showErrors) {
                formErrors = formErrors.concat(fieldErrors);
                module.add.prompt(identifier, fieldErrors, true);
                settings.onInvalid.call($field, fieldErrors);
              }
              return false;
            }
            return true;
          },

          // takes validation rule and returns whether field passes rule
          rule: function(field, rule, internal) {
            var
              $field       = module.get.field(field.identifier),
              ancillary    = module.get.ancillaryValue(rule),
              ruleName     = module.get.ruleName(rule),
              ruleFunction = settings.rules[ruleName],
              invalidFields = [],
              isCheckbox = $field.is(selector.checkbox),
              isValid = function(field){
                var value = (isCheckbox ? $(field).filter(':checked').val() : $(field).val());
                // cast to string avoiding encoding special values
                value = (value === undefined || value === '' || value === null)
                    ? ''
                    : (settings.shouldTrim) ? String(value + '').trim() : String(value + '')
                ;
                return ruleFunction.call(field, value, ancillary, $module);
              }
            ;
            if( !$.isFunction(ruleFunction) ) {
              module.error(error.noRule, ruleName);
              return;
            }
            if(isCheckbox) {
              if (!isValid($field)) {
                invalidFields = $field;
              }
            } else {
              $.each($field, function (index, field) {
                if (!isValid(field)) {
                  invalidFields.push(field);
                }
              });
            }
            return internal ? invalidFields : !(invalidFields.length>0);
          }
        },

        setting: function(name, value) {
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            settings[name] = value;
          }
          else {
            return settings[name];
          }
        },
        internal: function(name, value) {
          if( $.isPlainObject(name) ) {
            $.extend(true, module, name);
          }
          else if(value !== undefined) {
            module[name] = value;
          }
          else {
            return module[name];
          }
        },
        debug: function() {
          if(!settings.silent && settings.debug) {
            if(settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.debug = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.debug.apply(console, arguments);
            }
          }
        },
        verbose: function() {
          if(!settings.silent && settings.verbose && settings.debug) {
            if(settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.verbose = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.verbose.apply(console, arguments);
            }
          }
        },
        error: function() {
          if(!settings.silent) {
            module.error = Function.prototype.bind.call(console.error, console, settings.name + ':');
            module.error.apply(console, arguments);
          }
        },
        performance: {
          log: function(message) {
            var
              currentTime,
              executionTime,
              previousTime
            ;
            if(settings.performance) {
              currentTime   = new Date().getTime();
              previousTime  = time || currentTime;
              executionTime = currentTime - previousTime;
              time          = currentTime;
              performance.push({
                'Name'           : message[0],
                'Arguments'      : [].slice.call(message, 1) || '',
                'Element'        : element,
                'Execution Time' : executionTime
              });
            }
            clearTimeout(module.performance.timer);
            module.performance.timer = setTimeout(module.performance.display, 500);
          },
          display: function() {
            var
              title = settings.name + ':',
              totalTime = 0
            ;
            time = false;
            clearTimeout(module.performance.timer);
            $.each(performance, function(index, data) {
              totalTime += data['Execution Time'];
            });
            title += ' ' + totalTime + 'ms';
            if(moduleSelector) {
              title += ' \'' + moduleSelector + '\'';
            }
            if($allModules.length > 1) {
              title += ' ' + '(' + $allModules.length + ')';
            }
            if( (console.group !== undefined || console.table !== undefined) && performance.length > 0) {
              console.groupCollapsed(title);
              if(console.table) {
                console.table(performance);
              }
              else {
                $.each(performance, function(index, data) {
                  console.log(data['Name'] + ': ' + data['Execution Time']+'ms');
                });
              }
              console.groupEnd();
            }
            performance = [];
          }
        },
        invoke: function(query, passedArguments, context) {
          var
            object = instance,
            maxDepth,
            found,
            response
          ;
          passedArguments = passedArguments || queryArguments;
          context         = element         || context;
          if(typeof query == 'string' && object !== undefined) {
            query    = query.split(/[\. ]/);
            maxDepth = query.length - 1;
            $.each(query, function(depth, value) {
              var camelCaseValue = (depth != maxDepth)
                ? value + query[depth + 1].charAt(0).toUpperCase() + query[depth + 1].slice(1)
                : query
              ;
              if( $.isPlainObject( object[camelCaseValue] ) && (depth != maxDepth) ) {
                object = object[camelCaseValue];
              }
              else if( object[camelCaseValue] !== undefined ) {
                found = object[camelCaseValue];
                return false;
              }
              else if( $.isPlainObject( object[value] ) && (depth != maxDepth) ) {
                object = object[value];
              }
              else if( object[value] !== undefined ) {
                found = object[value];
                return false;
              }
              else {
                return false;
              }
            });
          }
          if( $.isFunction( found ) ) {
            response = found.apply(context, passedArguments);
          }
          else if(found !== undefined) {
            response = found;
          }
          if(Array.isArray(returnedValue)) {
            returnedValue.push(response);
          }
          else if(returnedValue !== undefined) {
            returnedValue = [returnedValue, response];
          }
          else if(response !== undefined) {
            returnedValue = response;
          }
          return found;
        }
      };
      module.initialize();
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.form.settings = {

  name              : 'Form',
  namespace         : 'form',

  debug             : false,
  verbose           : false,
  performance       : true,

  fields            : false,

  keyboardShortcuts : true,
  on                : 'submit',
  inline            : false,

  delay             : 200,
  revalidate        : true,
  shouldTrim        : true,

  transition        : 'scale',
  duration          : 200,

  autoCheckRequired : false,
  preventLeaving    : false,
  dateHandling      : 'date', // 'date', 'input', 'formatter'

  onValid           : function() {},
  onInvalid         : function() {},
  onSuccess         : function() { return true; },
  onFailure         : function() { return false; },
  onDirty           : function() {},
  onClean           : function() {},

  metadata : {
    defaultValue : 'default',
    validate     : 'validate',
    isDirty      : 'isDirty'
  },

  regExp: {
    htmlID  : /^[a-zA-Z][\w:.-]*$/g,
    bracket : /\[(.*)\]/i,
    decimal : /^\d+\.?\d*$/,
    email   : /^[a-z0-9!#$%&'*+\/=?^_`{|}~.-]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/i,
    escape  : /[\-\[\]\/\{\}\(\)\*\+\?\.\\\^\$\|:,=@]/g,
    flags   : /^\/(.*)\/(.*)?/,
    integer : /^\-?\d+$/,
    number  : /^\-?\d*(\.\d+)?$/,
    url     : /(https?:\/\/(?:www\.|(?!www))[^\s\.]+\.[^\s]{2,}|www\.[^\s]+\.[^\s]{2,})/i
  },

  text: {
    unspecifiedRule  : 'Please enter a valid value',
    unspecifiedField : 'This field',
    leavingMessage   : 'There are unsaved changes on this page which will be discarded if you continue.'
  },

  prompt: {
    empty                : '{name} must have a value',
    checked              : '{name} must be checked',
    email                : '{name} must be a valid e-mail',
    url                  : '{name} must be a valid url',
    regExp               : '{name} is not formatted correctly',
    integer              : '{name} must be an integer',
    decimal              : '{name} must be a decimal number',
    number               : '{name} must be set to a number',
    is                   : '{name} must be "{ruleValue}"',
    isExactly            : '{name} must be exactly "{ruleValue}"',
    not                  : '{name} cannot be set to "{ruleValue}"',
    notExactly           : '{name} cannot be set to exactly "{ruleValue}"',
    contain              : '{name} must contain "{ruleValue}"',
    containExactly       : '{name} must contain exactly "{ruleValue}"',
    doesntContain        : '{name} cannot contain  "{ruleValue}"',
    doesntContainExactly : '{name} cannot contain exactly "{ruleValue}"',
    minLength            : '{name} must be at least {ruleValue} characters',
    length               : '{name} must be at least {ruleValue} characters',
    exactLength          : '{name} must be exactly {ruleValue} characters',
    maxLength            : '{name} cannot be longer than {ruleValue} characters',
    match                : '{name} must match {ruleValue} field',
    different            : '{name} must have a different value than {ruleValue} field',
    creditCard           : '{name} must be a valid credit card number',
    minCount             : '{name} must have at least {ruleValue} choices',
    exactCount           : '{name} must have exactly {ruleValue} choices',
    maxCount             : '{name} must have {ruleValue} or less choices'
  },

  selector : {
    checkbox   : 'input[type="checkbox"], input[type="radio"]',
    clear      : '.clear',
    field      : 'input:not(.search), textarea, select',
    group      : '.field',
    input      : 'input',
    message    : '.error.message',
    prompt     : '.prompt.label',
    radio      : 'input[type="radio"]',
    reset      : '.reset:not([type="reset"])',
    submit     : '.submit:not([type="submit"])',
    uiCheckbox : '.ui.checkbox',
    uiDropdown : '.ui.dropdown',
    uiCalendar : '.ui.calendar'
  },

  className : {
    error    : 'error',
    label    : 'ui basic red pointing prompt label',
    pressed  : 'down',
    success  : 'success',
    required : 'required',
    disabled : 'disabled'
  },

  error: {
    identifier : 'You must specify a string identifier for each field',
    method     : 'The method you called is not defined.',
    noRule     : 'There is no rule matching the one you specified',
    oldSyntax  : 'Starting in 2.0 forms now only take a single settings object. Validation settings converted to new syntax automatically.',
    noElement  : 'This module requires ui {element}'
  },

  templates: {

    // template that produces error message
    error: function(errors) {
      var
        html = '<ul class="list">'
      ;
      $.each(errors, function(index, value) {
        html += '<li>' + value + '</li>';
      });
      html += '</ul>';
      return $(html);
    },

    // template that produces label
    prompt: function(errors, labelClasses) {
      return $('<div/>')
        .addClass(labelClasses)
        .html(errors[0])
      ;
    }
  },

  formatter: {
    date: function(date) {
      return Intl.DateTimeFormat('en-GB').format(date);
    },
    datetime: function(date) {
      return Intl.DateTimeFormat('en-GB', {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      }).format(date);
    },
    time: function(date) {
      return Intl.DateTimeFormat('en-GB', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      }).format(date);
    },
    month: function(date) {
      return Intl.DateTimeFormat('en-GB', {
        month: '2-digit',
        year: 'numeric'
      }).format(date);
    },
    year: function(date) {
      return Intl.DateTimeFormat('en-GB', {
        year: 'numeric'
      }).format(date);
    }
  },

  rules: {

    // is not empty or blank string
    empty: function(value) {
      return !(value === undefined || '' === value || Array.isArray(value) && value.length === 0);
    },

    // checkbox checked
    checked: function() {
      return ($(this).filter(':checked').length > 0);
    },

    // is most likely an email
    email: function(value){
      return $.fn.form.settings.regExp.email.test(value);
    },

    // value is most likely url
    url: function(value) {
      return $.fn.form.settings.regExp.url.test(value);
    },

    // matches specified regExp
    regExp: function(value, regExp) {
      if(regExp instanceof RegExp) {
        return value.match(regExp);
      }
      var
        regExpParts = regExp.match($.fn.form.settings.regExp.flags),
        flags
      ;
      // regular expression specified as /baz/gi (flags)
      if(regExpParts) {
        regExp = (regExpParts.length >= 2)
          ? regExpParts[1]
          : regExp
        ;
        flags = (regExpParts.length >= 3)
          ? regExpParts[2]
          : ''
        ;
      }
      return value.match( new RegExp(regExp, flags) );
    },

    // is valid integer or matches range
    integer: function(value, range) {
      var
        intRegExp = $.fn.form.settings.regExp.integer,
        min,
        max,
        parts
      ;
      if( !range || ['', '..'].indexOf(range) !== -1) {
        // do nothing
      }
      else if(range.indexOf('..') == -1) {
        if(intRegExp.test(range)) {
          min = max = range - 0;
        }
      }
      else {
        parts = range.split('..', 2);
        if(intRegExp.test(parts[0])) {
          min = parts[0] - 0;
        }
        if(intRegExp.test(parts[1])) {
          max = parts[1] - 0;
        }
      }
      return (
        intRegExp.test(value) &&
        (min === undefined || value >= min) &&
        (max === undefined || value <= max)
      );
    },

    // is valid number (with decimal)
    decimal: function(value) {
      return $.fn.form.settings.regExp.decimal.test(value);
    },

    // is valid number
    number: function(value) {
      return $.fn.form.settings.regExp.number.test(value);
    },

    // is value (case insensitive)
    is: function(value, text) {
      text = (typeof text == 'string')
        ? text.toLowerCase()
        : text
      ;
      value = (typeof value == 'string')
        ? value.toLowerCase()
        : value
      ;
      return (value == text);
    },

    // is value
    isExactly: function(value, text) {
      return (value == text);
    },

    // value is not another value (case insensitive)
    not: function(value, notValue) {
      value = (typeof value == 'string')
        ? value.toLowerCase()
        : value
      ;
      notValue = (typeof notValue == 'string')
        ? notValue.toLowerCase()
        : notValue
      ;
      return (value != notValue);
    },

    // value is not another value (case sensitive)
    notExactly: function(value, notValue) {
      return (value != notValue);
    },

    // value contains text (insensitive)
    contains: function(value, text) {
      // escape regex characters
      text = text.replace($.fn.form.settings.regExp.escape, "\\$&");
      return (value.search( new RegExp(text, 'i') ) !== -1);
    },

    // value contains text (case sensitive)
    containsExactly: function(value, text) {
      // escape regex characters
      text = text.replace($.fn.form.settings.regExp.escape, "\\$&");
      return (value.search( new RegExp(text) ) !== -1);
    },

    // value contains text (insensitive)
    doesntContain: function(value, text) {
      // escape regex characters
      text = text.replace($.fn.form.settings.regExp.escape, "\\$&");
      return (value.search( new RegExp(text, 'i') ) === -1);
    },

    // value contains text (case sensitive)
    doesntContainExactly: function(value, text) {
      // escape regex characters
      text = text.replace($.fn.form.settings.regExp.escape, "\\$&");
      return (value.search( new RegExp(text) ) === -1);
    },

    // is at least string length
    minLength: function(value, requiredLength) {
      return (value !== undefined)
        ? (value.length >= requiredLength)
        : false
      ;
    },

    // see rls notes for 2.0.6 (this is a duplicate of minLength)
    length: function(value, requiredLength) {
      return (value !== undefined)
        ? (value.length >= requiredLength)
        : false
      ;
    },

    // is exactly length
    exactLength: function(value, requiredLength) {
      return (value !== undefined)
        ? (value.length == requiredLength)
        : false
      ;
    },

    // is less than length
    maxLength: function(value, maxLength) {
      return (value !== undefined)
        ? (value.length <= maxLength)
        : false
      ;
    },

    // matches another field
    match: function(value, identifier, $module) {
      var
        matchingValue,
        matchingElement
      ;
      if((matchingElement = $module.find('[data-validate="'+ identifier +'"]')).length > 0 ) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('#' + identifier)).length > 0) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('[name="' + identifier +'"]')).length > 0) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('[name="' + identifier +'[]"]')).length > 0 ) {
        matchingValue = matchingElement;
      }
      return (matchingValue !== undefined)
        ? ( value.toString() == matchingValue.toString() )
        : false
      ;
    },

    // different than another field
    different: function(value, identifier, $module) {
      // use either id or name of field
      var
        matchingValue,
        matchingElement
      ;
      if((matchingElement = $module.find('[data-validate="'+ identifier +'"]')).length > 0 ) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('#' + identifier)).length > 0) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('[name="' + identifier +'"]')).length > 0) {
        matchingValue = matchingElement.val();
      }
      else if((matchingElement = $module.find('[name="' + identifier +'[]"]')).length > 0 ) {
        matchingValue = matchingElement;
      }
      return (matchingValue !== undefined)
        ? ( value.toString() !== matchingValue.toString() )
        : false
      ;
    },

    creditCard: function(cardNumber, cardTypes) {
      var
        cards = {
          visa: {
            pattern : /^4/,
            length  : [16]
          },
          amex: {
            pattern : /^3[47]/,
            length  : [15]
          },
          mastercard: {
            pattern : /^5[1-5]/,
            length  : [16]
          },
          discover: {
            pattern : /^(6011|622(12[6-9]|1[3-9][0-9]|[2-8][0-9]{2}|9[0-1][0-9]|92[0-5]|64[4-9])|65)/,
            length  : [16]
          },
          unionPay: {
            pattern : /^(62|88)/,
            length  : [16, 17, 18, 19]
          },
          jcb: {
            pattern : /^35(2[89]|[3-8][0-9])/,
            length  : [16]
          },
          maestro: {
            pattern : /^(5018|5020|5038|6304|6759|676[1-3])/,
            length  : [12, 13, 14, 15, 16, 17, 18, 19]
          },
          dinersClub: {
            pattern : /^(30[0-5]|^36)/,
            length  : [14]
          },
          laser: {
            pattern : /^(6304|670[69]|6771)/,
            length  : [16, 17, 18, 19]
          },
          visaElectron: {
            pattern : /^(4026|417500|4508|4844|491(3|7))/,
            length  : [16]
          }
        },
        valid         = {},
        validCard     = false,
        requiredTypes = (typeof cardTypes == 'string')
          ? cardTypes.split(',')
          : false,
        unionPay,
        validation
      ;

      if(typeof cardNumber !== 'string' || cardNumber.length === 0) {
        return;
      }

      // allow dashes in card
      cardNumber = cardNumber.replace(/[\-]/g, '');

      // verify card types
      if(requiredTypes) {
        $.each(requiredTypes, function(index, type){
          // verify each card type
          validation = cards[type];
          if(validation) {
            valid = {
              length  : ($.inArray(cardNumber.length, validation.length) !== -1),
              pattern : (cardNumber.search(validation.pattern) !== -1)
            };
            if(valid.length && valid.pattern) {
              validCard = true;
            }
          }
        });

        if(!validCard) {
          return false;
        }
      }

      // skip luhn for UnionPay
      unionPay = {
        number  : ($.inArray(cardNumber.length, cards.unionPay.length) !== -1),
        pattern : (cardNumber.search(cards.unionPay.pattern) !== -1)
      };
      if(unionPay.number && unionPay.pattern) {
        return true;
      }

      // verify luhn, adapted from  <https://gist.github.com/2134376>
      var
        length        = cardNumber.length,
        multiple      = 0,
        producedValue = [
          [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],
          [0, 2, 4, 6, 8, 1, 3, 5, 7, 9]
        ],
        sum           = 0
      ;
      while (length--) {
        sum += producedValue[multiple][parseInt(cardNumber.charAt(length), 10)];
        multiple ^= 1;
      }
      return (sum % 10 === 0 && sum > 0);
    },

    minCount: function(value, minCount) {
      if(minCount == 0) {
        return true;
      }
      if(minCount == 1) {
        return (value !== '');
      }
      return (value.split(',').length >= minCount);
    },

    exactCount: function(value, exactCount) {
      if(exactCount == 0) {
        return (value === '');
      }
      if(exactCount == 1) {
        return (value !== '' && value.search(',') === -1);
      }
      return (value.split(',').length == exactCount);
    },

    maxCount: function(value, maxCount) {
      if(maxCount == 0) {
        return false;
      }
      if(maxCount == 1) {
        return (value.search(',') === -1);
      }
      return (value.split(',').length <= maxCount);
    }
  }

};

})( jQuery, window, document );
