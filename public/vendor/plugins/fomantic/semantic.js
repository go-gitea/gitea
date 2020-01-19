 /*
 * # Fomantic UI - 2.8.2
 * https://github.com/fomantic/Fomantic-UI
 * http://fomantic-ui.com/
 *
 * Copyright 2014 Contributors
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */
/*!
 * # Fomantic-UI - Site
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

$.isFunction = $.isFunction || function(obj) {
    return typeof obj === "function" && typeof obj.nodeType !== "number";
};

$.site = $.fn.site = function(parameters) {
  var
    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    settings        = ( $.isPlainObject(parameters) )
      ? $.extend(true, {}, $.site.settings, parameters)
      : $.extend({}, $.site.settings),

    namespace       = settings.namespace,
    error           = settings.error,

    moduleNamespace = 'module-' + namespace,

    $document       = $(document),
    $module         = $document,
    element         = this,
    instance        = $module.data(moduleNamespace),

    module,
    returnedValue
  ;
  module = {

    initialize: function() {
      module.instantiate();
    },

    instantiate: function() {
      module.verbose('Storing instance of site', module);
      instance = module;
      $module
        .data(moduleNamespace, module)
      ;
    },

    normalize: function() {
      module.fix.console();
      module.fix.requestAnimationFrame();
    },

    fix: {
      console: function() {
        module.debug('Normalizing window.console');
        if (console === undefined || console.log === undefined) {
          module.verbose('Console not available, normalizing events');
          module.disable.console();
        }
        if (typeof console.group == 'undefined' || typeof console.groupEnd == 'undefined' || typeof console.groupCollapsed == 'undefined') {
          module.verbose('Console group not available, normalizing events');
          window.console.group = function() {};
          window.console.groupEnd = function() {};
          window.console.groupCollapsed = function() {};
        }
        if (typeof console.markTimeline == 'undefined') {
          module.verbose('Mark timeline not available, normalizing events');
          window.console.markTimeline = function() {};
        }
      },
      consoleClear: function() {
        module.debug('Disabling programmatic console clearing');
        window.console.clear = function() {};
      },
      requestAnimationFrame: function() {
        module.debug('Normalizing requestAnimationFrame');
        if(window.requestAnimationFrame === undefined) {
          module.debug('RequestAnimationFrame not available, normalizing event');
          window.requestAnimationFrame = window.requestAnimationFrame
            || window.mozRequestAnimationFrame
            || window.webkitRequestAnimationFrame
            || window.msRequestAnimationFrame
            || function(callback) { setTimeout(callback, 0); }
          ;
        }
      }
    },

    moduleExists: function(name) {
      return ($.fn[name] !== undefined && $.fn[name].settings !== undefined);
    },

    enabled: {
      modules: function(modules) {
        var
          enabledModules = []
        ;
        modules = modules || settings.modules;
        $.each(modules, function(index, name) {
          if(module.moduleExists(name)) {
            enabledModules.push(name);
          }
        });
        return enabledModules;
      }
    },

    disabled: {
      modules: function(modules) {
        var
          disabledModules = []
        ;
        modules = modules || settings.modules;
        $.each(modules, function(index, name) {
          if(!module.moduleExists(name)) {
            disabledModules.push(name);
          }
        });
        return disabledModules;
      }
    },

    change: {
      setting: function(setting, value, modules, modifyExisting) {
        modules = (typeof modules === 'string')
          ? (modules === 'all')
            ? settings.modules
            : [modules]
          : modules || settings.modules
        ;
        modifyExisting = (modifyExisting !== undefined)
          ? modifyExisting
          : true
        ;
        $.each(modules, function(index, name) {
          var
            namespace = (module.moduleExists(name))
              ? $.fn[name].settings.namespace || false
              : true,
            $existingModules
          ;
          if(module.moduleExists(name)) {
            module.verbose('Changing default setting', setting, value, name);
            $.fn[name].settings[setting] = value;
            if(modifyExisting && namespace) {
              $existingModules = $(':data(module-' + namespace + ')');
              if($existingModules.length > 0) {
                module.verbose('Modifying existing settings', $existingModules);
                $existingModules[name]('setting', setting, value);
              }
            }
          }
        });
      },
      settings: function(newSettings, modules, modifyExisting) {
        modules = (typeof modules === 'string')
          ? [modules]
          : modules || settings.modules
        ;
        modifyExisting = (modifyExisting !== undefined)
          ? modifyExisting
          : true
        ;
        $.each(modules, function(index, name) {
          var
            $existingModules
          ;
          if(module.moduleExists(name)) {
            module.verbose('Changing default setting', newSettings, name);
            $.extend(true, $.fn[name].settings, newSettings);
            if(modifyExisting && namespace) {
              $existingModules = $(':data(module-' + namespace + ')');
              if($existingModules.length > 0) {
                module.verbose('Modifying existing settings', $existingModules);
                $existingModules[name]('setting', newSettings);
              }
            }
          }
        });
      }
    },

    enable: {
      console: function() {
        module.console(true);
      },
      debug: function(modules, modifyExisting) {
        modules = modules || settings.modules;
        module.debug('Enabling debug for modules', modules);
        module.change.setting('debug', true, modules, modifyExisting);
      },
      verbose: function(modules, modifyExisting) {
        modules = modules || settings.modules;
        module.debug('Enabling verbose debug for modules', modules);
        module.change.setting('verbose', true, modules, modifyExisting);
      }
    },
    disable: {
      console: function() {
        module.console(false);
      },
      debug: function(modules, modifyExisting) {
        modules = modules || settings.modules;
        module.debug('Disabling debug for modules', modules);
        module.change.setting('debug', false, modules, modifyExisting);
      },
      verbose: function(modules, modifyExisting) {
        modules = modules || settings.modules;
        module.debug('Disabling verbose debug for modules', modules);
        module.change.setting('verbose', false, modules, modifyExisting);
      }
    },

    console: function(enable) {
      if(enable) {
        if(instance.cache.console === undefined) {
          module.error(error.console);
          return;
        }
        module.debug('Restoring console function');
        window.console = instance.cache.console;
      }
      else {
        module.debug('Disabling console function');
        instance.cache.console = window.console;
        window.console = {
          clear          : function(){},
          error          : function(){},
          group          : function(){},
          groupCollapsed : function(){},
          groupEnd       : function(){},
          info           : function(){},
          log            : function(){},
          markTimeline   : function(){},
          warn           : function(){}
        };
      }
    },

    destroy: function() {
      module.verbose('Destroying previous site for', $module);
      $module
        .removeData(moduleNamespace)
      ;
    },

    cache: {},

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
      if(settings.debug) {
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
      if(settings.verbose && settings.debug) {
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
      module.error = Function.prototype.bind.call(console.error, console, settings.name + ':');
      module.error.apply(console, arguments);
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
            'Element'        : element,
            'Name'           : message[0],
            'Arguments'      : [].slice.call(message, 1) || '',
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
            module.error(error.method, query);
            return false;
          }
        });
      }
      if ( $.isFunction( found ) ) {
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

  if(methodInvoked) {
    if(instance === undefined) {
      module.initialize();
    }
    module.invoke(query);
  }
  else {
    if(instance !== undefined) {
      module.destroy();
    }
    module.initialize();
  }
  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.site.settings = {

  name        : 'Site',
  namespace   : 'site',

  error : {
    console : 'Console cannot be restored, most likely it was overwritten outside of module',
    method : 'The method you called is not defined.'
  },

  debug       : false,
  verbose     : false,
  performance : true,

  modules: [
    'accordion',
    'api',
    'calendar',
    'checkbox',
    'dimmer',
    'dropdown',
    'embed',
    'form',
    'modal',
    'nag',
    'popup',
    'slider',
    'rating',
    'shape',
    'sidebar',
    'state',
    'sticky',
    'tab',
    'toast',
    'transition',
    'visibility',
    'visit'
  ],

  siteNamespace   : 'site',
  namespaceStub   : {
    cache     : {},
    config    : {},
    sections  : {},
    section   : {},
    utilities : {}
  }

};

// allows for selection of elements with data attributes
$.extend($.expr[ ":" ], {
  data: ($.expr.createPseudo)
    ? $.expr.createPseudo(function(dataName) {
        return function(elem) {
          return !!$.data(elem, dataName);
        };
      })
    : function(elem, i, match) {
      // support: jQuery < 1.8
      return !!$.data(elem, match[ 3 ]);
    }
});


})( jQuery, window, document );

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

          module.determine.isDirty();
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
            return $.trim($field.val()) === '';
          },
          valid: function(field) {
            var
              allValid = true
            ;
            if(field) {
              module.verbose('Checking if field is valid', field);
              return module.validate.field(validation[field], field, false);
            }
            else {
              module.verbose('Checking if form is valid');
              $.each(validation, function(fieldName, field) {
                if( !module.is.valid(fieldName) ) {
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
            var currentValue = $el.val();
            if (currentValue == null) { currentValue = ''; }

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
              prompt = prompt.replace('{value}', $field.val());
            }
            if(requiresName) {
              $label = $field.closest(selector.group).find('label').eq(0);
              name = ($label.length == 1)
                ? $label.text()
                : $field.prop('placeholder') || settings.text.unspecifiedField
              ;
              prompt = prompt.replace('{name}', name);
            }
            prompt = prompt.replace('{identifier}', field.identifier);
            prompt = prompt.replace('{ruleValue}', ancillary);
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
                    if(values[name] === undefined || values[name] == false) {
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
            var
              newValidation = {}
            ;
            if(module.is.shorthandRules(rules)) {
              rules = Array.isArray(rules)
                ? rules
                : [rules]
              ;
              newValidation[name] = {
                rules: []
              };
              $.each(rules, function(index, rule) {
                newValidation[name].rules.push({ type: rule });
              });
            }
            else {
              newValidation[name] = rules;
            }
            validation = $.extend({}, validation, newValidation);
            module.debug('Adding rules', newValidation, validation);
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
              if(ignoreCallbacks !== true) {
                return settings.onSuccess.call(element, event, values);
              }
            }
            else {
              module.debug('Form has errors');
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
            var isDisabled = true;
            $.each($field, function(){
                if(!$(this).prop('disabled')) {
                  isDisabled = false;
                  return false;
                }
            });
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
              $field.closest($group).removeClass(className.error);
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
                    : (settings.shouldTrim) ? $.trim(value + '') : String(value + '')
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
    field      : 'input, textarea, select',
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
    error   : 'error',
    label   : 'ui basic red pointing prompt label',
    pressed : 'down',
    success : 'success'
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

/*!
 * # Fomantic-UI - Accordion
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

$.fn.accordion = function(parameters) {
  var
    $allModules     = $(this),

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.accordion.settings, parameters)
          : $.extend({}, $.fn.accordion.settings),

        className       = settings.className,
        namespace       = settings.namespace,
        selector        = settings.selector,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,
        moduleSelector  = $allModules.selector || '',

        $module  = $(this),
        $title   = $module.find(selector.title),
        $content = $module.find(selector.content),

        element  = this,
        instance = $module.data(moduleNamespace),
        observer,
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing', $module);
          module.bind.events();
          if(settings.observeChanges) {
            module.observeChanges();
          }
          module.instantiate();
        },

        instantiate: function() {
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.debug('Destroying previous instance', $module);
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          $title   = $module.find(selector.title);
          $content = $module.find(selector.content);
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            observer = new MutationObserver(function(mutations) {
              module.debug('DOM tree modified, updating selector cache');
              module.refresh();
            });
            observer.observe(element, {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', observer);
          }
        },

        bind: {
          events: function() {
            module.debug('Binding delegated events');
            $module
              .on(settings.on + eventNamespace, selector.trigger, module.event.click)
            ;
          }
        },

        event: {
          click: function() {
            module.toggle.call(this);
          }
        },

        toggle: function(query) {
          var
            $activeTitle = (query !== undefined)
              ? (typeof query === 'number')
                ? $title.eq(query)
                : $(query).closest(selector.title)
              : $(this).closest(selector.title),
            $activeContent = $activeTitle.next($content),
            isAnimating = $activeContent.hasClass(className.animating),
            isActive    = $activeContent.hasClass(className.active),
            isOpen      = (isActive && !isAnimating),
            isOpening   = (!isActive && isAnimating)
          ;
          module.debug('Toggling visibility of content', $activeTitle);
          if(isOpen || isOpening) {
            if(settings.collapsible) {
              module.close.call($activeTitle);
            }
            else {
              module.debug('Cannot close accordion content collapsing is disabled');
            }
          }
          else {
            module.open.call($activeTitle);
          }
        },

        open: function(query) {
          var
            $activeTitle = (query !== undefined)
              ? (typeof query === 'number')
                ? $title.eq(query)
                : $(query).closest(selector.title)
              : $(this).closest(selector.title),
            $activeContent = $activeTitle.next($content),
            isAnimating = $activeContent.hasClass(className.animating),
            isActive    = $activeContent.hasClass(className.active),
            isOpen      = (isActive || isAnimating)
          ;
          if(isOpen) {
            module.debug('Accordion already open, skipping', $activeContent);
            return;
          }
          module.debug('Opening accordion content', $activeTitle);
          settings.onOpening.call($activeContent);
          settings.onChanging.call($activeContent);
          if(settings.exclusive) {
            module.closeOthers.call($activeTitle);
          }
          $activeTitle
            .addClass(className.active)
          ;
          $activeContent
            .stop(true, true)
            .addClass(className.animating)
          ;
          if(settings.animateChildren) {
            if($.fn.transition !== undefined && $module.transition('is supported')) {
              $activeContent
                .children()
                  .transition({
                    animation        : 'fade in',
                    queue            : false,
                    useFailSafe      : true,
                    debug            : settings.debug,
                    verbose          : settings.verbose,
                    duration         : settings.duration,
                    skipInlineHidden : true,
                    onComplete: function() {
                      $activeContent.children().removeClass(className.transition);
                    }
                  })
              ;
            }
            else {
              $activeContent
                .children()
                  .stop(true, true)
                  .animate({
                    opacity: 1
                  }, settings.duration, module.resetOpacity)
              ;
            }
          }
          $activeContent
            .slideDown(settings.duration, settings.easing, function() {
              $activeContent
                .removeClass(className.animating)
                .addClass(className.active)
              ;
              module.reset.display.call(this);
              settings.onOpen.call(this);
              settings.onChange.call(this);
            })
          ;
        },

        close: function(query) {
          var
            $activeTitle = (query !== undefined)
              ? (typeof query === 'number')
                ? $title.eq(query)
                : $(query).closest(selector.title)
              : $(this).closest(selector.title),
            $activeContent = $activeTitle.next($content),
            isAnimating    = $activeContent.hasClass(className.animating),
            isActive       = $activeContent.hasClass(className.active),
            isOpening      = (!isActive && isAnimating),
            isClosing      = (isActive && isAnimating)
          ;
          if((isActive || isOpening) && !isClosing) {
            module.debug('Closing accordion content', $activeContent);
            settings.onClosing.call($activeContent);
            settings.onChanging.call($activeContent);
            $activeTitle
              .removeClass(className.active)
            ;
            $activeContent
              .stop(true, true)
              .addClass(className.animating)
            ;
            if(settings.animateChildren) {
              if($.fn.transition !== undefined && $module.transition('is supported')) {
                $activeContent
                  .children()
                    .transition({
                      animation        : 'fade out',
                      queue            : false,
                      useFailSafe      : true,
                      debug            : settings.debug,
                      verbose          : settings.verbose,
                      duration         : settings.duration,
                      skipInlineHidden : true
                    })
                ;
              }
              else {
                $activeContent
                  .children()
                    .stop(true, true)
                    .animate({
                      opacity: 0
                    }, settings.duration, module.resetOpacity)
                ;
              }
            }
            $activeContent
              .slideUp(settings.duration, settings.easing, function() {
                $activeContent
                  .removeClass(className.animating)
                  .removeClass(className.active)
                ;
                module.reset.display.call(this);
                settings.onClose.call(this);
                settings.onChange.call(this);
              })
            ;
          }
        },

        closeOthers: function(index) {
          var
            $activeTitle = (index !== undefined)
              ? $title.eq(index)
              : $(this).closest(selector.title),
            $parentTitles    = $activeTitle.parents(selector.content).prev(selector.title),
            $activeAccordion = $activeTitle.closest(selector.accordion),
            activeSelector   = selector.title + '.' + className.active + ':visible',
            activeContent    = selector.content + '.' + className.active + ':visible',
            $openTitles,
            $nestedTitles,
            $openContents
          ;
          if(settings.closeNested) {
            $openTitles   = $activeAccordion.find(activeSelector).not($parentTitles);
            $openContents = $openTitles.next($content);
          }
          else {
            $openTitles   = $activeAccordion.find(activeSelector).not($parentTitles);
            $nestedTitles = $activeAccordion.find(activeContent).find(activeSelector).not($parentTitles);
            $openTitles   = $openTitles.not($nestedTitles);
            $openContents = $openTitles.next($content);
          }
          if( ($openTitles.length > 0) ) {
            module.debug('Exclusive enabled, closing other content', $openTitles);
            $openTitles
              .removeClass(className.active)
            ;
            $openContents
              .removeClass(className.animating)
              .stop(true, true)
            ;
            if(settings.animateChildren) {
              if($.fn.transition !== undefined && $module.transition('is supported')) {
                $openContents
                  .children()
                    .transition({
                      animation        : 'fade out',
                      useFailSafe      : true,
                      debug            : settings.debug,
                      verbose          : settings.verbose,
                      duration         : settings.duration,
                      skipInlineHidden : true
                    })
                ;
              }
              else {
                $openContents
                  .children()
                    .stop(true, true)
                    .animate({
                      opacity: 0
                    }, settings.duration, module.resetOpacity)
                ;
              }
            }
            $openContents
              .slideUp(settings.duration , settings.easing, function() {
                $(this).removeClass(className.active);
                module.reset.display.call(this);
              })
            ;
          }
        },

        reset: {

          display: function() {
            module.verbose('Removing inline display from element', this);
            $(this).css('display', '');
            if( $(this).attr('style') === '') {
              $(this)
                .attr('style', '')
                .removeAttr('style')
              ;
            }
          },

          opacity: function() {
            module.verbose('Removing inline opacity from element', this);
            $(this).css('opacity', '');
            if( $(this).attr('style') === '') {
              $(this)
                .attr('style', '')
                .removeAttr('style')
              ;
            }
          },

        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
          }
          else {
            return settings[name];
          }
        },
        internal: function(name, value) {
          module.debug('Changing internal', name, value);
          if(value !== undefined) {
            if( $.isPlainObject(name) ) {
              $.extend(true, module, name);
            }
            else {
              module[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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
      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;
  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.accordion.settings = {

  name            : 'Accordion',
  namespace       : 'accordion',

  silent          : false,
  debug           : false,
  verbose         : false,
  performance     : true,

  on              : 'click', // event on title that opens accordion

  observeChanges  : true,  // whether accordion should automatically refresh on DOM insertion

  exclusive       : true,  // whether a single accordion content panel should be open at once
  collapsible     : true,  // whether accordion content can be closed
  closeNested     : false, // whether nested content should be closed when a panel is closed
  animateChildren : true,  // whether children opacity should be animated

  duration        : 350, // duration of animation
  easing          : 'easeOutQuad', // easing equation for animation

  onOpening       : function(){}, // callback before open animation
  onClosing       : function(){}, // callback before closing animation
  onChanging      : function(){}, // callback before closing or opening animation

  onOpen          : function(){}, // callback after open animation
  onClose         : function(){}, // callback after closing animation
  onChange        : function(){}, // callback after closing or opening animation

  error: {
    method : 'The method you called is not defined'
  },

  className   : {
    active    : 'active',
    animating : 'animating',
    transition: 'transition'
  },

  selector    : {
    accordion : '.accordion',
    title     : '.title',
    trigger   : '.title',
    content   : '.content'
  }

};

// Adds easing
$.extend( $.easing, {
  easeOutQuad: function (x, t, b, c, d) {
    return -c *(t/=d)*(t-2) + b;
  }
});

})( jQuery, window, document );


/*!
 * # Fomantic-UI - Calendar
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

$.fn.calendar = function(parameters) {
  var
    $allModules    = $(this),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue,
    timeGapTable = {
      '5': {'row': 4, 'column': 3 },
      '10': {'row': 3, 'column': 2 },
      '15': {'row': 2, 'column': 2 },
      '20': {'row': 3, 'column': 1 },
      '30': {'row': 2, 'column': 1 }
    }
  ;

  $allModules
    .each(function () {
      var
        settings = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.calendar.settings, parameters)
          : $.extend({}, $.fn.calendar.settings),

        className = settings.className,
        namespace = settings.namespace,
        selector = settings.selector,
        formatter = settings.formatter,
        parser = settings.parser,
        metadata = settings.metadata,
        timeGap = timeGapTable[settings.minTimeGap],
        error = settings.error,

        eventNamespace = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module = $(this),
        $input = $module.find(selector.input),
        $container = $module.find(selector.popup),
        $activator = $module.find(selector.activator),

        element = this,
        instance = $module.data(moduleNamespace),

        isTouch,
        isTouchDown = false,
        focusDateUsedForRange = false,
        module
      ;

      module = {

        initialize: function () {
          module.debug('Initializing calendar for', element, $module);

          isTouch = module.get.isTouch();
          module.setup.config();
          module.setup.popup();
          module.setup.inline();
          module.setup.input();
          module.setup.date();
          module.create.calendar();

          module.bind.events();
          module.instantiate();
        },

        instantiate: function () {
          module.verbose('Storing instance of calendar');
          instance = module;
          $module.data(moduleNamespace, instance);
        },

        destroy: function () {
          module.verbose('Destroying previous calendar for', element);
          $module.removeData(moduleNamespace);
          module.unbind.events();
        },

        setup: {
          config: function () {
            if (module.get.minDate() !== null) {
              module.set.minDate($module.data(metadata.minDate));
            }
            if (module.get.maxDate() !== null) {
              module.set.maxDate($module.data(metadata.maxDate));
            }
            module.setting('type', module.get.type());
          },
          popup: function () {
            if (settings.inline) {
              return;
            }
            if (!$activator.length) {
              $activator = $module.children().first();
              if (!$activator.length) {
                return;
              }
            }
            if ($.fn.popup === undefined) {
              module.error(error.popup);
              return;
            }
            if (!$container.length) {
              //prepend the popup element to the activator's parent so that it has less chance of messing with
              //the styling (eg input action button needs to be the last child to have correct border radius)
              var $activatorParent = $activator.parent(),
                  domPositionFunction = $activatorParent.closest(selector.append).length !== 0 ? 'appendTo' : 'prependTo';
              $container = $('<div/>').addClass(className.popup)[domPositionFunction]($activatorParent);
            }
            $container.addClass(className.calendar);
            var onVisible = settings.onVisible;
            var onHidden = settings.onHidden;
            if (!$input.length) {
              //no input, $container has to handle focus/blur
              $container.attr('tabindex', '0');
              onVisible = function () {
                module.focus();
                return settings.onVisible.apply($container, arguments);
              };
              onHidden = function () {
                module.blur();
                return settings.onHidden.apply($container, arguments);
              };
            }
            var onShow = function () {
              //reset the focus date onShow
              module.set.focusDate(module.get.date());
              module.set.mode(settings.startMode);
              return settings.onShow.apply($container, arguments);
            };
            var on = settings.on || ($input.length ? 'focus' : 'click');
            var options = $.extend({}, settings.popupOptions, {
              popup: $container,
              on: on,
              hoverable: on === 'hover',
              onShow: onShow,
              onVisible: onVisible,
              onHide: settings.onHide,
              onHidden: onHidden
            });
            module.popup(options);
          },
          inline: function () {
            if ($activator.length && !settings.inline) {
              return;
            }
            $container = $('<div/>').addClass(className.calendar).appendTo($module);
            if (!$input.length) {
              $container.attr('tabindex', '0');
            }
          },
          input: function () {
            if (settings.touchReadonly && $input.length && isTouch) {
              $input.prop('readonly', true);
            }
          },
          date: function () {
            var date;
            if (settings.initialDate) {
              date = parser.date(settings.initialDate, settings);
            } else if ($module.data(metadata.date) !== undefined) {
              date = parser.date($module.data(metadata.date), settings);
            } else if ($input.length) {
              date = parser.date($input.val(), settings);
            }
            module.set.date(date, settings.formatInput, false);
          }
        },

        create: {
          calendar: function () {
            var i, r, c, p, row, cell, pageGrid;

            var mode = module.get.mode();
            var today = new Date();
            var date = module.get.date();
            var focusDate = module.get.focusDate();
            var display = focusDate || date || settings.initialDate || today;
            display = module.helper.dateInRange(display);

            if (!focusDate) {
              focusDate = display;
              module.set.focusDate(focusDate, false, false);
            }

            var isYear = mode === 'year';
            var isMonth = mode === 'month';
            var isDay = mode === 'day';
            var isHour = mode === 'hour';
            var isMinute = mode === 'minute';
            var isTimeOnly = settings.type === 'time';

            var multiMonth = Math.max(settings.multiMonth, 1);
            var monthOffset = !isDay ? 0 : module.get.monthOffset();

            var minute = display.getMinutes();
            var hour = display.getHours();
            var day = display.getDate();
            var startMonth = display.getMonth() + monthOffset;
            var year = display.getFullYear();

            var columns = isDay ? settings.showWeekNumbers ? 8 : 7 : isHour ? 4 : timeGap['column'];
            var rows = isDay || isHour ? 6 : timeGap['row'];
            var pages = isDay ? multiMonth : 1;

            var container = $container;
            var tooltipPosition = container.hasClass("left") ? "right center" : "left center";
            container.empty();
            if (pages > 1) {
              pageGrid = $('<div/>').addClass(className.grid).appendTo(container);
            }

            for (p = 0; p < pages; p++) {
              if (pages > 1) {
                var pageColumn = $('<div/>').addClass(className.column).appendTo(pageGrid);
                container = pageColumn;
              }

              var month = startMonth + p;
              var firstMonthDayColumn = (new Date(year, month, 1).getDay() - settings.firstDayOfWeek % 7 + 7) % 7;
              if (!settings.constantHeight && isDay) {
                var requiredCells = new Date(year, month + 1, 0).getDate() + firstMonthDayColumn;
                rows = Math.ceil(requiredCells / 7);
              }

              var yearChange = isYear ? 10 : isMonth ? 1 : 0;
              var monthChange = isDay ? 1 : 0;
              var dayChange = isHour || isMinute ? 1 : 0;
              var prevNextDay = isHour || isMinute ? day : 1;
              var prevDate = new Date(year - yearChange, month - monthChange, prevNextDay - dayChange, hour);
              var nextDate = new Date(year + yearChange, month + monthChange, prevNextDay + dayChange, hour);

              var prevLast = isYear ? new Date(Math.ceil(year / 10) * 10 - 9, 0, 0) :
                isMonth ? new Date(year, 0, 0) : isDay ? new Date(year, month, 0) : new Date(year, month, day, -1);
              var nextFirst = isYear ? new Date(Math.ceil(year / 10) * 10 + 1, 0, 1) :
                isMonth ? new Date(year + 1, 0, 1) : isDay ? new Date(year, month + 1, 1) : new Date(year, month, day + 1);

              var tempMode = mode;
              if (isDay && settings.showWeekNumbers){
                tempMode += ' andweek';
              }
              var table = $('<table/>').addClass(className.table).addClass(tempMode).appendTo(container);
              var textColumns = columns;
              //no header for time-only mode
              if (!isTimeOnly) {
                var thead = $('<thead/>').appendTo(table);

                row = $('<tr/>').appendTo(thead);
                cell = $('<th/>').attr('colspan', '' + columns).appendTo(row);

                var headerDate = isYear || isMonth ? new Date(year, 0, 1) :
                  isDay ? new Date(year, month, 1) : new Date(year, month, day, hour, minute);
                var headerText = $('<span/>').addClass(className.link).appendTo(cell);
                headerText.text(formatter.header(headerDate, mode, settings));
                var newMode = isMonth ? (settings.disableYear ? 'day' : 'year') :
                  isDay ? (settings.disableMonth ? 'year' : 'month') : 'day';
                headerText.data(metadata.mode, newMode);

                if (p === 0) {
                  var prev = $('<span/>').addClass(className.prev).appendTo(cell);
                  prev.data(metadata.focusDate, prevDate);
                  prev.toggleClass(className.disabledCell, !module.helper.isDateInRange(prevLast, mode));
                  $('<i/>').addClass(className.prevIcon).appendTo(prev);
                }

                if (p === pages - 1) {
                  var next = $('<span/>').addClass(className.next).appendTo(cell);
                  next.data(metadata.focusDate, nextDate);
                  next.toggleClass(className.disabledCell, !module.helper.isDateInRange(nextFirst, mode));
                  $('<i/>').addClass(className.nextIcon).appendTo(next);
                }
                if (isDay) {
                  row = $('<tr/>').appendTo(thead);
                  if(settings.showWeekNumbers) {
                      cell = $('<th/>').appendTo(row);
                      cell.text(settings.text.weekNo);
                      cell.addClass(className.weekCell);
                      textColumns--;
                  }
                  for (i = 0; i < textColumns; i++) {
                    cell = $('<th/>').appendTo(row);
                    cell.text(formatter.dayColumnHeader((i + settings.firstDayOfWeek) % 7, settings));
                  }
                }
              }

              var tbody = $('<tbody/>').appendTo(table);
              i = isYear ? Math.ceil(year / 10) * 10 - 9 : isDay ? 1 - firstMonthDayColumn : 0;
              for (r = 0; r < rows; r++) {
                row = $('<tr/>').appendTo(tbody);
                if(isDay && settings.showWeekNumbers){
                    cell = $('<th/>').appendTo(row);
                    cell.text(module.get.weekOfYear(year,month,i+1-settings.firstDayOfWeek));
                    cell.addClass(className.weekCell);
                }
                for (c = 0; c < textColumns; c++, i++) {
                  var cellDate = isYear ? new Date(i, month, 1, hour, minute) :
                    isMonth ? new Date(year, i, 1, hour, minute) : isDay ? new Date(year, month, i, hour, minute) :
                      isHour ? new Date(year, month, day, i) : new Date(year, month, day, hour, i * settings.minTimeGap);
                  var cellText = isYear ? i :
                    isMonth ? settings.text.monthsShort[i] : isDay ? cellDate.getDate() :
                      formatter.time(cellDate, settings, true);
                  cell = $('<td/>').addClass(className.cell).appendTo(row);
                  cell.text(cellText);
                  cell.data(metadata.date, cellDate);
                  var adjacent = isDay && cellDate.getMonth() !== ((month + 12) % 12);
                  var disabled = (!settings.selectAdjacentDays && adjacent) || !module.helper.isDateInRange(cellDate, mode) || settings.isDisabled(cellDate, mode) || module.helper.isDisabled(cellDate, mode) || !module.helper.isEnabled(cellDate, mode);
                  if (disabled) {
                    var disabledDate = module.helper.findDayAsObject(cellDate, mode, settings.disabledDates);
                    if (disabledDate !== null && disabledDate[metadata.message]) {
                      cell.attr("data-tooltip", disabledDate[metadata.message]);
                      cell.attr("data-position", tooltipPosition);
                    }
                  } else {
                    var eventDate = module.helper.findDayAsObject(cellDate, mode, settings.eventDates);
                    if (eventDate !== null) {
                      cell.addClass(eventDate[metadata.class] || settings.eventClass);
                      if (eventDate[metadata.message]) {
                        cell.attr("data-tooltip", eventDate[metadata.message]);
                        cell.attr("data-position", tooltipPosition);
                      }
                    }
                  }
                  var active = module.helper.dateEqual(cellDate, date, mode);
                  var isToday = module.helper.dateEqual(cellDate, today, mode);
                  cell.toggleClass(className.adjacentCell, adjacent);
                  cell.toggleClass(className.disabledCell, disabled);
                  cell.toggleClass(className.activeCell, active && !adjacent);
                  if (!isHour && !isMinute) {
                    cell.toggleClass(className.todayCell, !adjacent && isToday);
                  }

                  // Allow for external modifications of each cell
                  var cellOptions = {
                    mode: mode,
                    adjacent: adjacent,
                    disabled: disabled,
                    active: active,
                    today: isToday
                  };
                  formatter.cell(cell, cellDate, cellOptions);

                  if (module.helper.dateEqual(cellDate, focusDate, mode)) {
                    //ensure that the focus date is exactly equal to the cell date
                    //so that, if selected, the correct value is set
                    module.set.focusDate(cellDate, false, false);
                  }
                }
              }

              if (settings.today) {
                var todayRow = $('<tr/>').appendTo(tbody);
                var todayButton = $('<td/>').attr('colspan', '' + columns).addClass(className.today).appendTo(todayRow);
                todayButton.text(formatter.today(settings));
                todayButton.data(metadata.date, today);
              }

              module.update.focus(false, table);
            }
          }
        },

        update: {
          focus: function (updateRange, container) {
            container = container || $container;
            var mode = module.get.mode();
            var date = module.get.date();
            var focusDate = module.get.focusDate();
            var startDate = module.get.startDate();
            var endDate = module.get.endDate();
            var rangeDate = (updateRange ? focusDate : null) || date || (!isTouch ? focusDate : null);

            container.find('td').each(function () {
              var cell = $(this);
              var cellDate = cell.data(metadata.date);
              if (!cellDate) {
                return;
              }
              var disabled = cell.hasClass(className.disabledCell);
              var active = cell.hasClass(className.activeCell);
              var adjacent = cell.hasClass(className.adjacentCell);
              var focused = module.helper.dateEqual(cellDate, focusDate, mode);
              var inRange = !rangeDate ? false :
                ((!!startDate && module.helper.isDateInRange(cellDate, mode, startDate, rangeDate)) ||
                (!!endDate && module.helper.isDateInRange(cellDate, mode, rangeDate, endDate)));
              cell.toggleClass(className.focusCell, focused && (!isTouch || isTouchDown) && (!adjacent || (settings.selectAdjacentDays && adjacent)) && !disabled);

              if (module.helper.isTodayButton(cell)) {
                return;
              }
              cell.toggleClass(className.rangeCell, inRange && !active && !disabled);
            });
          }
        },

        refresh: function () {
          module.create.calendar();
        },

        bind: {
          events: function () {
            module.debug('Binding events');
            $container.on('mousedown' + eventNamespace, module.event.mousedown);
            $container.on('touchstart' + eventNamespace, module.event.mousedown);
            $container.on('mouseup' + eventNamespace, module.event.mouseup);
            $container.on('touchend' + eventNamespace, module.event.mouseup);
            $container.on('mouseover' + eventNamespace, module.event.mouseover);
            if ($input.length) {
              $input.on('input' + eventNamespace, module.event.inputChange);
              $input.on('focus' + eventNamespace, module.event.inputFocus);
              $input.on('blur' + eventNamespace, module.event.inputBlur);
              $input.on('click' + eventNamespace, module.event.inputClick);
              $input.on('keydown' + eventNamespace, module.event.keydown);
            } else {
              $container.on('keydown' + eventNamespace, module.event.keydown);
            }
          }
        },

        unbind: {
          events: function () {
            module.debug('Unbinding events');
            $container.off(eventNamespace);
            if ($input.length) {
              $input.off(eventNamespace);
            }
          }
        },

        event: {
          mouseover: function (event) {
            var target = $(event.target);
            var date = target.data(metadata.date);
            var mousedown = event.buttons === 1;
            if (date) {
              module.set.focusDate(date, false, true, mousedown);
            }
          },
          mousedown: function (event) {
            if ($input.length) {
              //prevent the mousedown on the calendar causing the input to lose focus
              event.preventDefault();
            }
            isTouchDown = event.type.indexOf('touch') >= 0;
            var target = $(event.target);
            var date = target.data(metadata.date);
            if (date) {
              module.set.focusDate(date, false, true, true);
            }
          },
          mouseup: function (event) {
            //ensure input has focus so that it receives keydown events for calendar navigation
            module.focus();
            event.preventDefault();
            event.stopPropagation();
            isTouchDown = false;
            var target = $(event.target);
            if (target.hasClass("disabled")) {
              return;
            }
            var parent = target.parent();
            if (parent.data(metadata.date) || parent.data(metadata.focusDate) || parent.data(metadata.mode)) {
              //clicked on a child element, switch to parent (used when clicking directly on prev/next <i> icon element)
              target = parent;
            }
            var date = target.data(metadata.date);
            var focusDate = target.data(metadata.focusDate);
            var mode = target.data(metadata.mode);
            if (date && settings.onSelect.call(element, date, module.get.mode()) !== false) {
              var forceSet = target.hasClass(className.today);
              module.selectDate(date, forceSet);
            }
            else if (focusDate) {
              module.set.focusDate(focusDate);
            }
            else if (mode) {
              module.set.mode(mode);
            }
          },
          keydown: function (event) {
            var keyCode = event.which;
            if (keyCode === 27 || keyCode === 9) {
              //esc || tab
              module.popup('hide');
            }

            if (module.popup('is visible')) {
              if (keyCode === 37 || keyCode === 38 || keyCode === 39 || keyCode === 40) {
                //arrow keys
                var mode = module.get.mode();
                var bigIncrement = mode === 'day' ? 7 : mode === 'hour' ? 4 : mode === 'minute' ? timeGap['column'] : 3;
                var increment = keyCode === 37 ? -1 : keyCode === 38 ? -bigIncrement : keyCode == 39 ? 1 : bigIncrement;
                increment *= mode === 'minute' ? settings.minTimeGap : 1;
                var focusDate = module.get.focusDate() || module.get.date() || new Date();
                var year = focusDate.getFullYear() + (mode === 'year' ? increment : 0);
                var month = focusDate.getMonth() + (mode === 'month' ? increment : 0);
                var day = focusDate.getDate() + (mode === 'day' ? increment : 0);
                var hour = focusDate.getHours() + (mode === 'hour' ? increment : 0);
                var minute = focusDate.getMinutes() + (mode === 'minute' ? increment : 0);
                var newFocusDate = new Date(year, month, day, hour, minute);
                if (settings.type === 'time') {
                  newFocusDate = module.helper.mergeDateTime(focusDate, newFocusDate);
                }
                if (module.helper.isDateInRange(newFocusDate, mode)) {
                  module.set.focusDate(newFocusDate);
                }
              } else if (keyCode === 13) {
                //enter
                var mode = module.get.mode();
                var date = module.get.focusDate();
                if (date && !settings.isDisabled(date, mode) && !module.helper.isDisabled(date, mode) && module.helper.isEnabled(date, mode)) {
                  module.selectDate(date);
                }
                //disable form submission:
                event.preventDefault();
                event.stopPropagation();
              }
            }

            if (keyCode === 38 || keyCode === 40) {
              //arrow-up || arrow-down
              event.preventDefault(); //don't scroll
              module.popup('show');
            }
          },
          inputChange: function () {
            var val = $input.val();
            var date = parser.date(val, settings);
            module.set.date(date, false);
          },
          inputFocus: function () {
            $container.addClass(className.active);
          },
          inputBlur: function () {
            $container.removeClass(className.active);
            if (settings.formatInput) {
              var date = module.get.date();
              var text = formatter.datetime(date, settings);
              $input.val(text);
            }
          },
          inputClick: function () {
            module.popup('show');
          }
        },

        get: {
          weekOfYear: function(weekYear,weekMonth,weekDay) {
              // adapted from http://www.merlyn.demon.co.uk/weekcalc.htm
              var ms1d = 864e5, // milliseconds in a day
                  ms7d = 7 * ms1d; // milliseconds in a week

              return function() { // return a closure so constants get calculated only once
                  var DC3 = Date.UTC(weekYear, weekMonth, weekDay + 3) / ms1d, // an Absolute Day Number
                      AWN = Math.floor(DC3 / 7), // an Absolute Week Number
                      Wyr = new Date(AWN * ms7d).getUTCFullYear();

                  return AWN - Math.floor(Date.UTC(Wyr, 0, 7) / ms7d) + 1;
              }();
          },
          date: function () {
            return module.helper.sanitiseDate($module.data(metadata.date)) || null;
          },
          inputDate: function() {
            return $input.val();
          },
          focusDate: function () {
            return $module.data(metadata.focusDate) || null;
          },
          startDate: function () {
            var startModule = module.get.calendarModule(settings.startCalendar);
            return (startModule ? startModule.get.date() : $module.data(metadata.startDate)) || null;
          },
          endDate: function () {
            var endModule = module.get.calendarModule(settings.endCalendar);
            return (endModule ? endModule.get.date() : $module.data(metadata.endDate)) || null;
          },
          minDate: function() {
            return $module.data(metadata.minDate) || null;
          },
          maxDate: function() {
            return $module.data(metadata.maxDate) || null;
          },
          monthOffset: function () {
            return $module.data(metadata.monthOffset) || 0;
          },
          mode: function () {
            //only returns valid modes for the current settings
            var mode = $module.data(metadata.mode) || settings.startMode;
            var validModes = module.get.validModes();
            if ($.inArray(mode, validModes) >= 0) {
              return mode;
            }
            return settings.type === 'time' ? 'hour' :
              settings.type === 'month' ? 'month' :
                settings.type === 'year' ? 'year' : 'day';
          },
          type: function() {
            return $module.data(metadata.type) || settings.type;
          },
          validModes: function () {
            var validModes = [];
            if (settings.type !== 'time') {
              if (!settings.disableYear || settings.type === 'year') {
                validModes.push('year');
              }
              if (!(settings.disableMonth || settings.type === 'year') || settings.type === 'month') {
                validModes.push('month');
              }
              if (settings.type.indexOf('date') >= 0) {
                validModes.push('day');
              }
            }
            if (settings.type.indexOf('time') >= 0) {
              validModes.push('hour');
              if (!settings.disableMinute) {
                validModes.push('minute');
              }
            }
            return validModes;
          },
          isTouch: function () {
            try {
              document.createEvent('TouchEvent');
              return true;
            }
            catch (e) {
              return false;
            }
          },
          calendarModule: function (selector) {
            if (!selector) {
              return null;
            }
            if (!(selector instanceof $)) {
              selector = $(selector).first();
            }
            //assume range related calendars are using the same namespace
            return selector.data(moduleNamespace);
          }
        },

        set: {
          date: function (date, updateInput, fireChange) {
            updateInput = updateInput !== false;
            fireChange = fireChange !== false;
            date = module.helper.sanitiseDate(date);
            date = module.helper.dateInRange(date);

            var mode = module.get.mode();
            var text = formatter.datetime(date, settings);

            if (fireChange && settings.onBeforeChange.call(element, date, text, mode) === false) {
              return false;
            }

            module.set.focusDate(date);

            if (settings.isDisabled(date, mode)) {
              return false;
            }

            var endDate = module.get.endDate();
            if (!!endDate && !!date && date > endDate) {
              //selected date is greater than end date in range, so clear end date
              module.set.endDate(undefined);
            }
            module.set.dataKeyValue(metadata.date, date);

            if (updateInput && $input.length) {
              $input.val(text);
            }

            if (fireChange) {
              settings.onChange.call(element, date, text, mode);
            }
          },
          startDate: function (date, refreshCalendar) {
            date = module.helper.sanitiseDate(date);
            var startModule = module.get.calendarModule(settings.startCalendar);
            if (startModule) {
              startModule.set.date(date);
            }
            module.set.dataKeyValue(metadata.startDate, date, refreshCalendar);
          },
          endDate: function (date, refreshCalendar) {
            date = module.helper.sanitiseDate(date);
            var endModule = module.get.calendarModule(settings.endCalendar);
            if (endModule) {
              endModule.set.date(date);
            }
            module.set.dataKeyValue(metadata.endDate, date, refreshCalendar);
          },
          focusDate: function (date, refreshCalendar, updateFocus, updateRange) {
            date = module.helper.sanitiseDate(date);
            date = module.helper.dateInRange(date);
            var isDay = module.get.mode() === 'day';
            var oldFocusDate = module.get.focusDate();
            if (isDay && date && oldFocusDate) {
              var yearDelta = date.getFullYear() - oldFocusDate.getFullYear();
              var monthDelta = yearDelta * 12 + date.getMonth() - oldFocusDate.getMonth();
              if (monthDelta) {
                var monthOffset = module.get.monthOffset() - monthDelta;
                module.set.monthOffset(monthOffset, false);
              }
            }
            var changed = module.set.dataKeyValue(metadata.focusDate, date, refreshCalendar);
            updateFocus = (updateFocus !== false && changed && refreshCalendar === false) || focusDateUsedForRange != updateRange;
            focusDateUsedForRange = updateRange;
            if (updateFocus) {
              module.update.focus(updateRange);
            }
          },
          minDate: function (date) {
            date = module.helper.sanitiseDate(date);
            if (settings.maxDate !== null && settings.maxDate <= date) {
              module.verbose('Unable to set minDate variable bigger that maxDate variable', date, settings.maxDate);
            } else {
              module.setting('minDate', date);
              module.set.dataKeyValue(metadata.minDate, date);
            }
          },
          maxDate: function (date) {
            date = module.helper.sanitiseDate(date);
            if (settings.minDate !== null && settings.minDate >= date) {
              module.verbose('Unable to set maxDate variable lower that minDate variable', date, settings.minDate);
            } else {
              module.setting('maxDate', date);
              module.set.dataKeyValue(metadata.maxDate, date);
            }
          },
          monthOffset: function (monthOffset, refreshCalendar) {
            var multiMonth = Math.max(settings.multiMonth, 1);
            monthOffset = Math.max(1 - multiMonth, Math.min(0, monthOffset));
            module.set.dataKeyValue(metadata.monthOffset, monthOffset, refreshCalendar);
          },
          mode: function (mode, refreshCalendar) {
            module.set.dataKeyValue(metadata.mode, mode, refreshCalendar);
          },
          dataKeyValue: function (key, value, refreshCalendar) {
            var oldValue = $module.data(key);
            var equal = oldValue === value || (oldValue <= value && oldValue >= value); //equality test for dates and string objects
            if (value) {
              $module.data(key, value);
            } else {
              $module.removeData(key);
            }
            refreshCalendar = refreshCalendar !== false && !equal;
            if (refreshCalendar) {
              module.refresh();
            }
            return !equal;
          }
        },

        selectDate: function (date, forceSet) {
          module.verbose('New date selection', date);
          var mode = module.get.mode();
          var complete = forceSet || mode === 'minute' ||
            (settings.disableMinute && mode === 'hour') ||
            (settings.type === 'date' && mode === 'day') ||
            (settings.type === 'month' && mode === 'month') ||
            (settings.type === 'year' && mode === 'year');
          if (complete) {
            var canceled = module.set.date(date) === false;
            if (!canceled && settings.closable) {
              module.popup('hide');
              //if this is a range calendar, show the end date calendar popup and focus the input
              var endModule = module.get.calendarModule(settings.endCalendar);
              if (endModule) {
                endModule.popup('show');
                endModule.focus();
              }
            }
          } else {
            var newMode = mode === 'year' ? (!settings.disableMonth ? 'month' : 'day') :
              mode === 'month' ? 'day' : mode === 'day' ? 'hour' : 'minute';
            module.set.mode(newMode);
            if (mode === 'hour' || (mode === 'day' && module.get.date())) {
              //the user has chosen enough to consider a valid date/time has been chosen
              module.set.date(date);
            } else {
              module.set.focusDate(date);
            }
          }
        },

        changeDate: function (date) {
          module.set.date(date);
        },

        clear: function () {
          module.set.date(undefined);
        },

        popup: function () {
          return $activator.popup.apply($activator, arguments);
        },

        focus: function () {
          if ($input.length) {
            $input.focus();
          } else {
            $container.focus();
          }
        },
        blur: function () {
          if ($input.length) {
            $input.blur();
          } else {
            $container.blur();
          }
        },

        helper: {
          isDisabled: function(date, mode) {
            return mode === 'day' && ((settings.disabledDaysOfWeek.indexOf(date.getDay()) !== -1) || settings.disabledDates.some(function(d){
              if(typeof d === 'string') {
                d = module.helper.sanitiseDate(d);
              }
              if (d instanceof Date) {
                return module.helper.dateEqual(date, d, mode);
              }
              if (d !== null && typeof d === 'object' && d[metadata.date]) {
                return module.helper.dateEqual(date, module.helper.sanitiseDate(d[metadata.date]), mode);
              }
            }));
          },
          isEnabled: function(date, mode) {
            if (mode === 'day') {
              return settings.enabledDates.length === 0 || settings.enabledDates.some(function(d){
                if(typeof d === 'string') {
                  d = module.helper.sanitiseDate(d);
                }
                if (d instanceof Date) {
                  return module.helper.dateEqual(date, d, mode);
                }
                if (d !== null && typeof d === 'object' && d[metadata.date]) {
                  return module.helper.dateEqual(date, module.helper.sanitiseDate(d[metadata.date]), mode);
                }
              });
            } else {
              return true;
            }
          },
          findDayAsObject: function(date, mode, dates) {
            if (mode === 'day') {
              var i = 0, il = dates.length;
              var d;
              for (; i < il; i++) {
                d = dates[i];
                if(typeof d === 'string') {
                  d = module.helper.sanitiseDate(d);
                }
                if (d instanceof Date && module.helper.dateEqual(date, d, mode)) {
                  var dateObject = {};
                  dateObject[metadata.date] = d;
                  return dateObject;
                }
                else if (d !== null && typeof d === 'object' && d[metadata.date] && module.helper.dateEqual(date,module.helper.sanitiseDate(d[metadata.date]), mode)  ) {
                  return d;
                }
              }
            }
            return null;
          },
          sanitiseDate: function (date) {
            if (!date) {
              return undefined;
            }
            if (!(date instanceof Date)) {
              date = parser.date('' + date, settings);
            }
            if (!date || date === null || isNaN(date.getTime())) {
              return undefined;
            }
            return date;
          },
          dateDiff: function (date1, date2, mode) {
            mode = mode || 'day';
            var isTimeOnly = settings.type === 'time';
            var isYear = mode === 'year';
            var isYearOrMonth = isYear || mode === 'month';
            var isMinute = mode === 'minute';
            var isHourOrMinute = isMinute || mode === 'hour';
            //only care about a minute accuracy of settings.minTimeGap
            date1 = new Date(
              isTimeOnly ? 2000 : date1.getFullYear(),
              isTimeOnly ? 0 : isYear ? 0 : date1.getMonth(),
              isTimeOnly ? 1 : isYearOrMonth ? 1 : date1.getDate(),
              !isHourOrMinute ? 0 : date1.getHours(),
              !isMinute ? 0 : settings.minTimeGap * Math.floor(date1.getMinutes() / settings.minTimeGap));
            date2 = new Date(
              isTimeOnly ? 2000 : date2.getFullYear(),
              isTimeOnly ? 0 : isYear ? 0 : date2.getMonth(),
              isTimeOnly ? 1 : isYearOrMonth ? 1 : date2.getDate(),
              !isHourOrMinute ? 0 : date2.getHours(),
              !isMinute ? 0 : settings.minTimeGap * Math.floor(date2.getMinutes() / settings.minTimeGap));
            return date2.getTime() - date1.getTime();
          },
          dateEqual: function (date1, date2, mode) {
            return !!date1 && !!date2 && module.helper.dateDiff(date1, date2, mode) === 0;
          },
          isDateInRange: function (date, mode, minDate, maxDate) {
            if (!minDate && !maxDate) {
              var startDate = module.get.startDate();
              minDate = startDate && settings.minDate ? new Date(Math.max(startDate, settings.minDate)) : startDate || settings.minDate;
              maxDate = settings.maxDate;
            }
            minDate = minDate && new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate(), minDate.getHours(), settings.minTimeGap * Math.ceil(minDate.getMinutes() / settings.minTimeGap));
            return !(!date ||
            (minDate && module.helper.dateDiff(date, minDate, mode) > 0) ||
            (maxDate && module.helper.dateDiff(maxDate, date, mode) > 0));
          },
          dateInRange: function (date, minDate, maxDate) {
            if (!minDate && !maxDate) {
              var startDate = module.get.startDate();
              minDate = startDate && settings.minDate ? new Date(Math.max(startDate, settings.minDate)) : startDate || settings.minDate;
              maxDate = settings.maxDate;
            }
            minDate = minDate && new Date(minDate.getFullYear(), minDate.getMonth(), minDate.getDate(), minDate.getHours(), settings.minTimeGap * Math.ceil(minDate.getMinutes() / settings.minTimeGap));
            var isTimeOnly = settings.type === 'time';
            return !date ? date :
              (minDate && module.helper.dateDiff(date, minDate, 'minute') > 0) ?
                (isTimeOnly ? module.helper.mergeDateTime(date, minDate) : minDate) :
                (maxDate && module.helper.dateDiff(maxDate, date, 'minute') > 0) ?
                  (isTimeOnly ? module.helper.mergeDateTime(date, maxDate) : maxDate) :
                  date;
          },
          mergeDateTime: function (date, time) {
            return (!date || !time) ? time :
              new Date(date.getFullYear(), date.getMonth(), date.getDate(), time.getHours(), time.getMinutes());
          },
          isTodayButton: function(element) {
            return element.text() === settings.text.today;
          }
        },

        setting: function (name, value) {
          module.debug('Changing setting', name, value);
          if ($.isPlainObject(name)) {
            $.extend(true, settings, name);
          }
          else if (value !== undefined) {
            if ($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
          }
          else {
            return settings[name];
          }
        },
        internal: function (name, value) {
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
        debug: function () {
          if (!settings.silent && settings.debug) {
            if (settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.debug = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.debug.apply(console, arguments);
            }
          }
        },
        verbose: function () {
          if (!settings.silent && settings.verbose && settings.debug) {
            if (settings.performance) {
              module.performance.log(arguments);
            }
            else {
              module.verbose = Function.prototype.bind.call(console.info, console, settings.name + ':');
              module.verbose.apply(console, arguments);
            }
          }
        },
        error: function () {
          if (!settings.silent) {
            module.error = Function.prototype.bind.call(console.error, console, settings.name + ':');
            module.error.apply(console, arguments);
          }
        },
        performance: {
          log: function (message) {
            var
              currentTime,
              executionTime,
              previousTime
              ;
            if (settings.performance) {
              currentTime = new Date().getTime();
              previousTime = time || currentTime;
              executionTime = currentTime - previousTime;
              time = currentTime;
              performance.push({
                'Name': message[0],
                'Arguments': [].slice.call(message, 1) || '',
                'Element': element,
                'Execution Time': executionTime
              });
            }
            clearTimeout(module.performance.timer);
            module.performance.timer = setTimeout(module.performance.display, 500);
          },
          display: function () {
            var
              title = settings.name + ':',
              totalTime = 0
              ;
            time = false;
            clearTimeout(module.performance.timer);
            $.each(performance, function (index, data) {
              totalTime += data['Execution Time'];
            });
            title += ' ' + totalTime + 'ms';
            if (moduleSelector) {
              title += ' \'' + moduleSelector + '\'';
            }
            if ((console.group !== undefined || console.table !== undefined) && performance.length > 0) {
              console.groupCollapsed(title);
              if (console.table) {
                console.table(performance);
              }
              else {
                $.each(performance, function (index, data) {
                  console.log(data['Name'] + ': ' + data['Execution Time'] + 'ms');
                });
              }
              console.groupEnd();
            }
            performance = [];
          }
        },
        invoke: function (query, passedArguments, context) {
          var
            object = instance,
            maxDepth,
            found,
            response
            ;
          passedArguments = passedArguments || queryArguments;
          context = element || context;
          if (typeof query == 'string' && object !== undefined) {
            query = query.split(/[\. ]/);
            maxDepth = query.length - 1;
            $.each(query, function (depth, value) {
              var camelCaseValue = (depth != maxDepth)
                  ? value + query[depth + 1].charAt(0).toUpperCase() + query[depth + 1].slice(1)
                  : query
                ;
              if ($.isPlainObject(object[camelCaseValue]) && (depth != maxDepth)) {
                object = object[camelCaseValue];
              }
              else if (object[camelCaseValue] !== undefined) {
                found = object[camelCaseValue];
                return false;
              }
              else if ($.isPlainObject(object[value]) && (depth != maxDepth)) {
                object = object[value];
              }
              else if (object[value] !== undefined) {
                found = object[value];
                return false;
              }
              else {
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ($.isFunction(found)) {
            response = found.apply(context, passedArguments);
          }
          else if (found !== undefined) {
            response = found;
          }
          if (Array.isArray(returnedValue)) {
            returnedValue.push(response);
          }
          else if (returnedValue !== undefined) {
            returnedValue = [returnedValue, response];
          }
          else if (response !== undefined) {
            returnedValue = response;
          }
          return found;
        }
      };

      if (methodInvoked) {
        if (instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if (instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;
  return (returnedValue !== undefined)
    ? returnedValue
    : this
    ;
};

$.fn.calendar.settings = {

  name            : 'Calendar',
  namespace       : 'calendar',

  silent: false,
  debug: false,
  verbose: false,
  performance: false,

  type               : 'datetime', // picker type, can be 'datetime', 'date', 'time', 'month', or 'year'
  firstDayOfWeek     : 0,          // day for first day column (0 = Sunday)
  constantHeight     : true,       // add rows to shorter months to keep day calendar height consistent (6 rows)
  today              : false,      // show a 'today/now' button at the bottom of the calendar
  closable           : true,       // close the popup after selecting a date/time
  monthFirst         : true,       // month before day when parsing/converting date from/to text
  touchReadonly      : true,       // set input to readonly on touch devices
  inline             : false,      // create the calendar inline instead of inside a popup
  on                 : null,       // when to show the popup (defaults to 'focus' for input, 'click' for others)
  initialDate        : null,       // date to display initially when no date is selected (null = now)
  startMode          : false,      // display mode to start in, can be 'year', 'month', 'day', 'hour', 'minute' (false = 'day')
  minDate            : null,       // minimum date/time that can be selected, dates/times before are disabled
  maxDate            : null,       // maximum date/time that can be selected, dates/times after are disabled
  ampm               : true,       // show am/pm in time mode
  disableYear        : false,      // disable year selection mode
  disableMonth       : false,      // disable month selection mode
  disableMinute      : false,      // disable minute selection mode
  formatInput        : true,       // format the input text upon input blur and module creation
  startCalendar      : null,       // jquery object or selector for another calendar that represents the start date of a date range
  endCalendar        : null,       // jquery object or selector for another calendar that represents the end date of a date range
  multiMonth         : 1,          // show multiple months when in 'day' mode
  minTimeGap         : 5,
  showWeekNumbers    : null,       // show Number of Week at the very first column of a dayView
  disabledDates      : [],         // specific day(s) which won't be selectable and contain additional information.
  disabledDaysOfWeek : [],         // day(s) which won't be selectable(s) (0 = Sunday)
  enabledDates       : [],         // specific day(s) which will be selectable, all other days will be disabled
  eventDates         : [],         // specific day(s) which will be shown in a different color and using tooltips
  centuryBreak       : 60,         // starting short year until 99 where it will be assumed to belong to the last century
  currentCentury     : 2000,       // century to be added to 2-digit years (00 to {centuryBreak}-1)
  selectAdjacentDays : false,     // The calendar can show dates from adjacent month. These adjacent month dates can also be made selectable.
  // popup options ('popup', 'on', 'hoverable', and show/hide callbacks are overridden)
  popupOptions: {
    position: 'bottom left',
    lastResort: 'bottom left',
    prefer: 'opposite',
    hideOnScroll: false
  },

  text: {
    days: ['S', 'M', 'T', 'W', 'T', 'F', 'S'],
    months: ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'],
    monthsShort: ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'],
    today: 'Today',
    now: 'Now',
    am: 'AM',
    pm: 'PM',
    weekNo: 'Week'
  },

  formatter: {
    header: function (date, mode, settings) {
      return mode === 'year' ? settings.formatter.yearHeader(date, settings) :
        mode === 'month' ? settings.formatter.monthHeader(date, settings) :
          mode === 'day' ? settings.formatter.dayHeader(date, settings) :
            mode === 'hour' ? settings.formatter.hourHeader(date, settings) :
              settings.formatter.minuteHeader(date, settings);
    },
    yearHeader: function (date, settings) {
      var decadeYear = Math.ceil(date.getFullYear() / 10) * 10;
      return (decadeYear - 9) + ' - ' + (decadeYear + 2);
    },
    monthHeader: function (date, settings) {
      return date.getFullYear();
    },
    dayHeader: function (date, settings) {
      var month = settings.text.months[date.getMonth()];
      var year = date.getFullYear();
      return month + ' ' + year;
    },
    hourHeader: function (date, settings) {
      return settings.formatter.date(date, settings);
    },
    minuteHeader: function (date, settings) {
      return settings.formatter.date(date, settings);
    },
    dayColumnHeader: function (day, settings) {
      return settings.text.days[day];
    },
    datetime: function (date, settings) {
      if (!date) {
        return '';
      }
      var day = settings.type === 'time' ? '' : settings.formatter.date(date, settings);
      var time = settings.type.indexOf('time') < 0 ? '' : settings.formatter.time(date, settings, false);
      var separator = settings.type === 'datetime' ? ' ' : '';
      return day + separator + time;
    },
    date: function (date, settings) {
      if (!date) {
        return '';
      }
      var day = date.getDate();
      var month = settings.text.months[date.getMonth()];
      var year = date.getFullYear();
      return settings.type === 'year' ? year :
        settings.type === 'month' ? month + ' ' + year :
        (settings.monthFirst ? month + ' ' + day : day + ' ' + month) + ', ' + year;
    },
    time: function (date, settings, forCalendar) {
      if (!date) {
        return '';
      }
      var hour = date.getHours();
      var minute = date.getMinutes();
      var ampm = '';
      if (settings.ampm) {
        ampm = ' ' + (hour < 12 ? settings.text.am : settings.text.pm);
        hour = hour === 0 ? 12 : hour > 12 ? hour - 12 : hour;
      }
      return hour + ':' + (minute < 10 ? '0' : '') + minute + ampm;
    },
    today: function (settings) {
      return settings.type === 'date' ? settings.text.today : settings.text.now;
    },
    cell: function (cell, date, cellOptions) {
    }
  },

  parser: {
    date: function (text, settings) {
      if (text instanceof Date) {
        return text;
      }
      if (!text) {
        return null;
      }
      text = ('' + text).trim().toLowerCase();
      if (text.length === 0) {
        return null;
      }
      // Reverse date and month in some cases
      text = settings.monthFirst ? text : text.replace(/[\/\-\.]/g,'/').replace(/([0-9]+)\/([0-9]+)/,'$2/$1');
      var textDate = new Date(text);
      if(!isNaN(textDate.getDate())) {
        return textDate;
      }

      var i, j, k;
      var minute = -1, hour = -1, day = -1, month = -1, year = -1;
      var isAm = undefined;

      var isTimeOnly = settings.type === 'time';
      var isDateOnly = settings.type.indexOf('time') < 0;

      var words = text.split(settings.regExp.dateWords), word;
      var numbers = text.split(settings.regExp.dateNumbers), number;

      var parts;
      var monthString;

      if (!isDateOnly) {
        //am/pm
        isAm = $.inArray(settings.text.am.toLowerCase(), words) >= 0 ? true :
          $.inArray(settings.text.pm.toLowerCase(), words) >= 0 ? false : undefined;

        //time with ':'
        for (i = 0; i < numbers.length; i++) {
          number = numbers[i];
          if (number.indexOf(':') >= 0) {
            if (hour < 0 || minute < 0) {
              parts = number.split(':');
              for (k = 0; k < Math.min(2, parts.length); k++) {
                j = parseInt(parts[k]);
                if (isNaN(j)) {
                  j = 0;
                }
                if (k === 0) {
                  hour = j % 24;
                } else {
                  minute = j % 60;
                }
              }
            }
            numbers.splice(i, 1);
          }
        }
      }

      if (!isTimeOnly) {
        //textual month
        for (i = 0; i < words.length; i++) {
          word = words[i];
          if (word.length <= 0) {
            continue;
          }
          for (j = 0; j < settings.text.months.length; j++) {
            monthString = settings.text.months[j];
            monthString = monthString.substring(0, word.length).toLowerCase();
            if (monthString === word) {
              month = j + 1;
              break;
            }
          }
          if (month >= 0) {
            break;
          }
        }

        //year > settings.centuryBreak
        for (i = 0; i < numbers.length; i++) {
          j = parseInt(numbers[i]);
          if (isNaN(j)) {
            continue;
          }
          if (j >= settings.centuryBreak && i === numbers.length-1) {
            if (j <= 99) {
              j += settings.currentCentury - 100;
            }
            year = j;
            numbers.splice(i, 1);
            break;
          }
        }

        //numeric month
        if (month < 0) {
          for (i = 0; i < numbers.length; i++) {
            k = i > 1 || settings.monthFirst ? i : i === 1 ? 0 : 1;
            j = parseInt(numbers[k]);
            if (isNaN(j)) {
              continue;
            }
            if (1 <= j && j <= 12) {
              month = j;
              numbers.splice(k, 1);
              break;
            }
          }
        }

        //day
        for (i = 0; i < numbers.length; i++) {
          j = parseInt(numbers[i]);
          if (isNaN(j)) {
            continue;
          }
          if (1 <= j && j <= 31) {
            day = j;
            numbers.splice(i, 1);
            break;
          }
        }

        //year <= settings.centuryBreak
        if (year < 0) {
          for (i = numbers.length - 1; i >= 0; i--) {
            j = parseInt(numbers[i]);
            if (isNaN(j)) {
              continue;
            }
            if (j <= 99) {
              j += settings.currentCentury;
            }
            year = j;
            numbers.splice(i, 1);
            break;
          }
        }
      }

      if (!isDateOnly) {
        //hour
        if (hour < 0) {
          for (i = 0; i < numbers.length; i++) {
            j = parseInt(numbers[i]);
            if (isNaN(j)) {
              continue;
            }
            if (0 <= j && j <= 23) {
              hour = j;
              numbers.splice(i, 1);
              break;
            }
          }
        }

        //minute
        if (minute < 0) {
          for (i = 0; i < numbers.length; i++) {
            j = parseInt(numbers[i]);
            if (isNaN(j)) {
              continue;
            }
            if (0 <= j && j <= 59) {
              minute = j;
              numbers.splice(i, 1);
              break;
            }
          }
        }
      }

      if (minute < 0 && hour < 0 && day < 0 && month < 0 && year < 0) {
        return null;
      }

      if (minute < 0) {
        minute = 0;
      }
      if (hour < 0) {
        hour = 0;
      }
      if (day < 0) {
        day = 1;
      }
      if (month < 0) {
        month = 1;
      }
      if (year < 0) {
        year = new Date().getFullYear();
      }

      if (isAm !== undefined) {
        if (isAm) {
          if (hour === 12) {
            hour = 0;
          }
        } else if (hour < 12) {
          hour += 12;
        }
      }

      var date = new Date(year, month - 1, day, hour, minute);
      if (date.getMonth() !== month - 1 || date.getFullYear() !== year) {
        //month or year don't match up, switch to last day of the month
        date = new Date(year, month, 0, hour, minute);
      }
      return isNaN(date.getTime()) ? null : date;
    }
  },

  // callback before date is changed, return false to cancel the change
  onBeforeChange: function (date, text, mode) {
    return true;
  },

  // callback when date changes
  onChange: function (date, text, mode) {
  },

  // callback before show animation, return false to prevent show
  onShow: function () {
  },

  // callback after show animation
  onVisible: function () {
  },

  // callback before hide animation, return false to prevent hide
  onHide: function () {
  },

  // callback after hide animation
  onHidden: function () {
  },

  // callback before item is selected, return false to prevent selection
  onSelect: function (date, mode) {
  },

  // is the given date disabled?
  isDisabled: function (date, mode) {
    return false;
  },

  selector: {
    popup: '.ui.popup',
    input: 'input',
    activator: 'input',
    append: '.inline.field,.inline.fields'
  },

  regExp: {
    dateWords: /[^A-Za-z\u00C0-\u024F]+/g,
    dateNumbers: /[^\d:]+/g
  },

  error: {
    popup: 'UI Popup, a required component is not included in this page',
    method: 'The method you called is not defined.'
  },

  className: {
    calendar: 'calendar',
    active: 'active',
    popup: 'ui popup',
    grid: 'ui equal width grid',
    column: 'column',
    table: 'ui celled center aligned unstackable table',
    prev: 'prev link',
    next: 'next link',
    prevIcon: 'chevron left icon',
    nextIcon: 'chevron right icon',
    link: 'link',
    cell: 'link',
    disabledCell: 'disabled',
    weekCell: 'disabled',
    adjacentCell: 'adjacent',
    activeCell: 'active',
    rangeCell: 'range',
    focusCell: 'focus',
    todayCell: 'today',
    today: 'today link'
  },

  metadata: {
    date: 'date',
    focusDate: 'focusDate',
    startDate: 'startDate',
    endDate: 'endDate',
    minDate: 'minDate',
    maxDate: 'maxDate',
    mode: 'mode',
    type: 'type',
    monthOffset: 'monthOffset',
    message: 'message',
    class: 'class'
  },

  eventClass: 'blue'
};

})(jQuery, window, document);

/*!
 * # Fomantic-UI - Checkbox
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

$.fn.checkbox = function(parameters) {
  var
    $allModules    = $(this),
    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings        = $.extend(true, {}, $.fn.checkbox.settings, parameters),

        className       = settings.className,
        namespace       = settings.namespace,
        selector        = settings.selector,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $label          = $(this).children(selector.label),
        $input          = $(this).children(selector.input),
        input           = $input[0],

        initialLoad     = false,
        shortcutPressed = false,
        instance        = $module.data(moduleNamespace),

        observer,
        element         = this,
        module
      ;

      module      = {

        initialize: function() {
          module.verbose('Initializing checkbox', settings);

          module.create.label();
          module.bind.events();

          module.set.tabbable();
          module.hide.input();

          module.observeChanges();
          module.instantiate();
          module.setup();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying module');
          module.unbind.events();
          module.show.input();
          $module.removeData(moduleNamespace);
        },

        fix: {
          reference: function() {
            if( $module.is(selector.input) ) {
              module.debug('Behavior called on <input> adjusting invoked element');
              $module = $module.closest(selector.checkbox);
              module.refresh();
            }
          }
        },

        setup: function() {
          module.set.initialLoad();
          if( module.is.indeterminate() ) {
            module.debug('Initial value is indeterminate');
            module.indeterminate();
          }
          else if( module.is.checked() ) {
            module.debug('Initial value is checked');
            module.check();
          }
          else {
            module.debug('Initial value is unchecked');
            module.uncheck();
          }
          module.remove.initialLoad();
        },

        refresh: function() {
          $label = $module.children(selector.label);
          $input = $module.children(selector.input);
          input  = $input[0];
        },

        hide: {
          input: function() {
            module.verbose('Modifying <input> z-index to be unselectable');
            $input.addClass(className.hidden);
          }
        },
        show: {
          input: function() {
            module.verbose('Modifying <input> z-index to be selectable');
            $input.removeClass(className.hidden);
          }
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            observer = new MutationObserver(function(mutations) {
              module.debug('DOM tree modified, updating selector cache');
              module.refresh();
            });
            observer.observe(element, {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', observer);
          }
        },

        attachEvents: function(selector, event) {
          var
            $element = $(selector)
          ;
          event = $.isFunction(module[event])
            ? module[event]
            : module.toggle
          ;
          if($element.length > 0) {
            module.debug('Attaching checkbox events to element', selector, event);
            $element
              .on('click' + eventNamespace, event)
            ;
          }
          else {
            module.error(error.notFound);
          }
        },

        preventDefaultOnInputTarget: function() {
          if(typeof event !== 'undefined' && $(event.target).is(selector.input)) {
            module.verbose('Preventing default check action after manual check action');
            event.preventDefault();
          }
        },

        event: {
          change: function(event) {
            if( !module.should.ignoreCallbacks() ) {
              settings.onChange.call(input);
            }
          },
          click: function(event) {
            var
              $target = $(event.target)
            ;
            if( $target.is(selector.input) ) {
              module.verbose('Using default check action on initialized checkbox');
              return;
            }
            if( $target.is(selector.link) ) {
              module.debug('Clicking link inside checkbox, skipping toggle');
              return;
            }
            module.toggle();
            $input.focus();
            event.preventDefault();
          },
          keydown: function(event) {
            var
              key     = event.which,
              keyCode = {
                enter  : 13,
                space  : 32,
                escape : 27,
                left   : 37,
                up     : 38,
                right  : 39,
                down   : 40
              }
            ;

            var r = module.get.radios(),
                rIndex = r.index($module),
                rLen = r.length,
                checkIndex = false;

            if(key == keyCode.left || key == keyCode.up) {
              checkIndex = (rIndex === 0 ? rLen : rIndex) - 1;
            } else if(key == keyCode.right || key == keyCode.down) {
              checkIndex = rIndex === rLen-1 ? 0 : rIndex+1;
            }

            if (!module.should.ignoreCallbacks() && checkIndex !== false) {
              if(settings.beforeUnchecked.apply(input)===false) {
                module.verbose('Option not allowed to be unchecked, cancelling key navigation');
                return false;
              }
              if (settings.beforeChecked.apply($(r[checkIndex]).children(selector.input)[0])===false) {
                module.verbose('Next option should not allow check, cancelling key navigation');
                return false;
              }
            }

            if(key == keyCode.escape) {
              module.verbose('Escape key pressed blurring field');
              $input.blur();
              shortcutPressed = true;
            }
            else if(!event.ctrlKey && ( key == keyCode.space || (key == keyCode.enter && settings.enableEnterKey)) ) {
              module.verbose('Enter/space key pressed, toggling checkbox');
              module.toggle();
              shortcutPressed = true;
            }
            else {
              shortcutPressed = false;
            }
          },
          keyup: function(event) {
            if(shortcutPressed) {
              event.preventDefault();
            }
          }
        },

        check: function() {
          if( !module.should.allowCheck() ) {
            return;
          }
          module.debug('Checking checkbox', $input);
          module.set.checked();
          if( !module.should.ignoreCallbacks() ) {
            settings.onChecked.call(input);
            module.trigger.change();
          }
          module.preventDefaultOnInputTarget();
        },

        uncheck: function() {
          if( !module.should.allowUncheck() ) {
            return;
          }
          module.debug('Unchecking checkbox');
          module.set.unchecked();
          if( !module.should.ignoreCallbacks() ) {
            settings.onUnchecked.call(input);
            module.trigger.change();
          }
          module.preventDefaultOnInputTarget();
        },

        indeterminate: function() {
          if( module.should.allowIndeterminate() ) {
            module.debug('Checkbox is already indeterminate');
            return;
          }
          module.debug('Making checkbox indeterminate');
          module.set.indeterminate();
          if( !module.should.ignoreCallbacks() ) {
            settings.onIndeterminate.call(input);
            module.trigger.change();
          }
        },

        determinate: function() {
          if( module.should.allowDeterminate() ) {
            module.debug('Checkbox is already determinate');
            return;
          }
          module.debug('Making checkbox determinate');
          module.set.determinate();
          if( !module.should.ignoreCallbacks() ) {
            settings.onDeterminate.call(input);
            module.trigger.change();
          }
        },

        enable: function() {
          if( module.is.enabled() ) {
            module.debug('Checkbox is already enabled');
            return;
          }
          module.debug('Enabling checkbox');
          module.set.enabled();
          if( !module.should.ignoreCallbacks() ) {
            settings.onEnable.call(input);
            // preserve legacy callbacks
            settings.onEnabled.call(input);
            module.trigger.change();
          }
        },

        disable: function() {
          if( module.is.disabled() ) {
            module.debug('Checkbox is already disabled');
            return;
          }
          module.debug('Disabling checkbox');
          module.set.disabled();
          if( !module.should.ignoreCallbacks() ) {
            settings.onDisable.call(input);
            // preserve legacy callbacks
            settings.onDisabled.call(input);
            module.trigger.change();
          }
        },

        get: {
          radios: function() {
            var
              name = module.get.name()
            ;
            return $('input[name="' + name + '"]').closest(selector.checkbox);
          },
          otherRadios: function() {
            return module.get.radios().not($module);
          },
          name: function() {
            return $input.attr('name');
          }
        },

        is: {
          initialLoad: function() {
            return initialLoad;
          },
          radio: function() {
            return ($input.hasClass(className.radio) || $input.attr('type') == 'radio');
          },
          indeterminate: function() {
            return $input.prop('indeterminate') !== undefined && $input.prop('indeterminate');
          },
          checked: function() {
            return $input.prop('checked') !== undefined && $input.prop('checked');
          },
          disabled: function() {
            return $input.prop('disabled') !== undefined && $input.prop('disabled');
          },
          enabled: function() {
            return !module.is.disabled();
          },
          determinate: function() {
            return !module.is.indeterminate();
          },
          unchecked: function() {
            return !module.is.checked();
          }
        },

        should: {
          allowCheck: function() {
            if(module.is.determinate() && module.is.checked() && !module.is.initialLoad() ) {
              module.debug('Should not allow check, checkbox is already checked');
              return false;
            }
            if(!module.should.ignoreCallbacks() && settings.beforeChecked.apply(input) === false) {
              module.debug('Should not allow check, beforeChecked cancelled');
              return false;
            }
            return true;
          },
          allowUncheck: function() {
            if(module.is.determinate() && module.is.unchecked() && !module.is.initialLoad() ) {
              module.debug('Should not allow uncheck, checkbox is already unchecked');
              return false;
            }
            if(!module.should.ignoreCallbacks() && settings.beforeUnchecked.apply(input) === false) {
              module.debug('Should not allow uncheck, beforeUnchecked cancelled');
              return false;
            }
            return true;
          },
          allowIndeterminate: function() {
            if(module.is.indeterminate() && !module.is.initialLoad() ) {
              module.debug('Should not allow indeterminate, checkbox is already indeterminate');
              return false;
            }
            if(!module.should.ignoreCallbacks() && settings.beforeIndeterminate.apply(input) === false) {
              module.debug('Should not allow indeterminate, beforeIndeterminate cancelled');
              return false;
            }
            return true;
          },
          allowDeterminate: function() {
            if(module.is.determinate() && !module.is.initialLoad() ) {
              module.debug('Should not allow determinate, checkbox is already determinate');
              return false;
            }
            if(!module.should.ignoreCallbacks() && settings.beforeDeterminate.apply(input) === false) {
              module.debug('Should not allow determinate, beforeDeterminate cancelled');
              return false;
            }
            return true;
          },
          ignoreCallbacks: function() {
            return (initialLoad && !settings.fireOnInit);
          }
        },

        can: {
          change: function() {
            return !( $module.hasClass(className.disabled) || $module.hasClass(className.readOnly) || $input.prop('disabled') || $input.prop('readonly') );
          },
          uncheck: function() {
            return (typeof settings.uncheckable === 'boolean')
              ? settings.uncheckable
              : !module.is.radio()
            ;
          }
        },

        set: {
          initialLoad: function() {
            initialLoad = true;
          },
          checked: function() {
            module.verbose('Setting class to checked');
            $module
              .removeClass(className.indeterminate)
              .addClass(className.checked)
            ;
            if( module.is.radio() ) {
              module.uncheckOthers();
            }
            if(!module.is.indeterminate() && module.is.checked()) {
              module.debug('Input is already checked, skipping input property change');
              return;
            }
            module.verbose('Setting state to checked', input);
            $input
              .prop('indeterminate', false)
              .prop('checked', true)
            ;
          },
          unchecked: function() {
            module.verbose('Removing checked class');
            $module
              .removeClass(className.indeterminate)
              .removeClass(className.checked)
            ;
            if(!module.is.indeterminate() &&  module.is.unchecked() ) {
              module.debug('Input is already unchecked');
              return;
            }
            module.debug('Setting state to unchecked');
            $input
              .prop('indeterminate', false)
              .prop('checked', false)
            ;
          },
          indeterminate: function() {
            module.verbose('Setting class to indeterminate');
            $module
              .addClass(className.indeterminate)
            ;
            if( module.is.indeterminate() ) {
              module.debug('Input is already indeterminate, skipping input property change');
              return;
            }
            module.debug('Setting state to indeterminate');
            $input
              .prop('indeterminate', true)
            ;
          },
          determinate: function() {
            module.verbose('Removing indeterminate class');
            $module
              .removeClass(className.indeterminate)
            ;
            if( module.is.determinate() ) {
              module.debug('Input is already determinate, skipping input property change');
              return;
            }
            module.debug('Setting state to determinate');
            $input
              .prop('indeterminate', false)
            ;
          },
          disabled: function() {
            module.verbose('Setting class to disabled');
            $module
              .addClass(className.disabled)
            ;
            if( module.is.disabled() ) {
              module.debug('Input is already disabled, skipping input property change');
              return;
            }
            module.debug('Setting state to disabled');
            $input
              .prop('disabled', 'disabled')
            ;
          },
          enabled: function() {
            module.verbose('Removing disabled class');
            $module.removeClass(className.disabled);
            if( module.is.enabled() ) {
              module.debug('Input is already enabled, skipping input property change');
              return;
            }
            module.debug('Setting state to enabled');
            $input
              .prop('disabled', false)
            ;
          },
          tabbable: function() {
            module.verbose('Adding tabindex to checkbox');
            if( $input.attr('tabindex') === undefined) {
              $input.attr('tabindex', 0);
            }
          }
        },

        remove: {
          initialLoad: function() {
            initialLoad = false;
          }
        },

        trigger: {
          change: function() {
            var
              events       = document.createEvent('HTMLEvents'),
              inputElement = $input[0]
            ;
            if(inputElement) {
              module.verbose('Triggering native change event');
              events.initEvent('change', true, false);
              inputElement.dispatchEvent(events);
            }
          }
        },


        create: {
          label: function() {
            if($input.prevAll(selector.label).length > 0) {
              $input.prev(selector.label).detach().insertAfter($input);
              module.debug('Moving existing label', $label);
            }
            else if( !module.has.label() ) {
              $label = $('<label>').insertAfter($input);
              module.debug('Creating label', $label);
            }
          }
        },

        has: {
          label: function() {
            return ($label.length > 0);
          }
        },

        bind: {
          events: function() {
            module.verbose('Attaching checkbox events');
            $module
              .on('click'   + eventNamespace, module.event.click)
              .on('change'  + eventNamespace, module.event.change)
              .on('keydown' + eventNamespace, selector.input, module.event.keydown)
              .on('keyup'   + eventNamespace, selector.input, module.event.keyup)
            ;
          }
        },

        unbind: {
          events: function() {
            module.debug('Removing events');
            $module
              .off(eventNamespace)
            ;
          }
        },

        uncheckOthers: function() {
          var
            $radios = module.get.otherRadios()
          ;
          module.debug('Unchecking other radios', $radios);
          $radios.removeClass(className.checked);
        },

        toggle: function() {
          if( !module.can.change() ) {
            if(!module.is.radio()) {
              module.debug('Checkbox is read-only or disabled, ignoring toggle');
            }
            return;
          }
          if( module.is.indeterminate() || module.is.unchecked() ) {
            module.debug('Currently unchecked');
            module.check();
          }
          else if( module.is.checked() && module.can.uncheck() ) {
            module.debug('Currently checked');
            module.uncheck();
          }
        },
        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.checkbox.settings = {

  name                : 'Checkbox',
  namespace           : 'checkbox',

  silent              : false,
  debug               : false,
  verbose             : true,
  performance         : true,

  // delegated event context
  uncheckable         : 'auto',
  fireOnInit          : false,
  enableEnterKey      : true,

  onChange            : function(){},

  beforeChecked       : function(){},
  beforeUnchecked     : function(){},
  beforeDeterminate   : function(){},
  beforeIndeterminate : function(){},

  onChecked           : function(){},
  onUnchecked         : function(){},

  onDeterminate       : function() {},
  onIndeterminate     : function() {},

  onEnable            : function(){},
  onDisable           : function(){},

  // preserve misspelled callbacks (will be removed in 3.0)
  onEnabled           : function(){},
  onDisabled          : function(){},

  className       : {
    checked       : 'checked',
    indeterminate : 'indeterminate',
    disabled      : 'disabled',
    hidden        : 'hidden',
    radio         : 'radio',
    readOnly      : 'read-only'
  },

  error     : {
    method       : 'The method you called is not defined'
  },

  selector : {
    checkbox : '.ui.checkbox',
    label    : 'label, .box',
    input    : 'input[type="checkbox"], input[type="radio"]',
    link     : 'a[href]'
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Dimmer
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

$.fn.dimmer = function(parameters) {
  var
    $allModules     = $(this),

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.dimmer.settings, parameters)
          : $.extend({}, $.fn.dimmer.settings),

        selector        = settings.selector,
        namespace       = settings.namespace,
        className       = settings.className,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,
        moduleSelector  = $allModules.selector || '',

        clickEvent      = ('ontouchstart' in document.documentElement)
          ? 'touchstart'
          : 'click',

        $module = $(this),
        $dimmer,
        $dimmable,

        element   = this,
        instance  = $module.data(moduleNamespace),
        module
      ;

      module = {

        preinitialize: function() {
          if( module.is.dimmer() ) {

            $dimmable = $module.parent();
            $dimmer   = $module;
          }
          else {
            $dimmable = $module;
            if( module.has.dimmer() ) {
              if(settings.dimmerName) {
                $dimmer = $dimmable.find(selector.dimmer).filter('.' + settings.dimmerName);
              }
              else {
                $dimmer = $dimmable.find(selector.dimmer);
              }
            }
            else {
              $dimmer = module.create();
            }
          }
        },

        initialize: function() {
          module.debug('Initializing dimmer', settings);

          module.bind.events();
          module.set.dimmable();
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous module', $dimmer);
          module.unbind.events();
          module.remove.variation();
          $dimmable
            .off(eventNamespace)
          ;
        },

        bind: {
          events: function() {
            if(settings.on == 'hover') {
              $dimmable
                .on('mouseenter' + eventNamespace, module.show)
                .on('mouseleave' + eventNamespace, module.hide)
              ;
            }
            else if(settings.on == 'click') {
              $dimmable
                .on(clickEvent + eventNamespace, module.toggle)
              ;
            }
            if( module.is.page() ) {
              module.debug('Setting as a page dimmer', $dimmable);
              module.set.pageDimmer();
            }

            if( module.is.closable() ) {
              module.verbose('Adding dimmer close event', $dimmer);
              $dimmable
                .on(clickEvent + eventNamespace, selector.dimmer, module.event.click)
              ;
            }
          }
        },

        unbind: {
          events: function() {
            $module
              .removeData(moduleNamespace)
            ;
            $dimmable
              .off(eventNamespace)
            ;
          }
        },

        event: {
          click: function(event) {
            module.verbose('Determining if event occured on dimmer', event);
            if( $dimmer.find(event.target).length === 0 || $(event.target).is(selector.content) ) {
              module.hide();
              event.stopImmediatePropagation();
            }
          }
        },

        addContent: function(element) {
          var
            $content = $(element)
          ;
          module.debug('Add content to dimmer', $content);
          if($content.parent()[0] !== $dimmer[0]) {
            $content.detach().appendTo($dimmer);
          }
        },

        create: function() {
          var
            $element = $( settings.template.dimmer(settings) )
          ;
          if(settings.dimmerName) {
            module.debug('Creating named dimmer', settings.dimmerName);
            $element.addClass(settings.dimmerName);
          }
          $element
            .appendTo($dimmable)
          ;
          return $element;
        },

        show: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          module.debug('Showing dimmer', $dimmer, settings);
          module.set.variation();
          if( (!module.is.dimmed() || module.is.animating()) && module.is.enabled() ) {
            module.animate.show(callback);
            settings.onShow.call(element);
            settings.onChange.call(element);
          }
          else {
            module.debug('Dimmer is already shown or disabled');
          }
        },

        hide: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( module.is.dimmed() || module.is.animating() ) {
            module.debug('Hiding dimmer', $dimmer);
            module.animate.hide(callback);
            settings.onHide.call(element);
            settings.onChange.call(element);
          }
          else {
            module.debug('Dimmer is not visible');
          }
        },

        toggle: function() {
          module.verbose('Toggling dimmer visibility', $dimmer);
          if( !module.is.dimmed() ) {
            module.show();
          }
          else {
            if ( module.is.closable() ) {
              module.hide();
            }
          }
        },

        animate: {
          show: function(callback) {
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if(settings.useCSS && $.fn.transition !== undefined && $dimmer.transition('is supported')) {
              if(settings.useFlex) {
                module.debug('Using flex dimmer');
                module.remove.legacy();
              }
              else {
                module.debug('Using legacy non-flex dimmer');
                module.set.legacy();
              }
              if(settings.opacity !== 'auto') {
                module.set.opacity();
              }
              $dimmer
                .transition({
                  displayType : settings.useFlex
                    ? 'flex'
                    : 'block',
                  animation   : settings.transition + ' in',
                  queue       : false,
                  duration    : module.get.duration(),
                  useFailSafe : true,
                  onStart     : function() {
                    module.set.dimmed();
                  },
                  onComplete  : function() {
                    module.set.active();
                    callback();
                  }
                })
              ;
            }
            else {
              module.verbose('Showing dimmer animation with javascript');
              module.set.dimmed();
              if(settings.opacity == 'auto') {
                settings.opacity = 0.8;
              }
              $dimmer
                .stop()
                .css({
                  opacity : 0,
                  width   : '100%',
                  height  : '100%'
                })
                .fadeTo(module.get.duration(), settings.opacity, function() {
                  $dimmer.removeAttr('style');
                  module.set.active();
                  callback();
                })
              ;
            }
          },
          hide: function(callback) {
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if(settings.useCSS && $.fn.transition !== undefined && $dimmer.transition('is supported')) {
              module.verbose('Hiding dimmer with css');
              $dimmer
                .transition({
                  displayType : settings.useFlex
                    ? 'flex'
                    : 'block',
                  animation   : settings.transition + ' out',
                  queue       : false,
                  duration    : module.get.duration(),
                  useFailSafe : true,
                  onComplete  : function() {
                    module.remove.dimmed();
                    module.remove.variation();
                    module.remove.active();
                    callback();
                  }
                })
              ;
            }
            else {
              module.verbose('Hiding dimmer with javascript');
              $dimmer
                .stop()
                .fadeOut(module.get.duration(), function() {
                  module.remove.dimmed();
                  module.remove.active();
                  $dimmer.removeAttr('style');
                  callback();
                })
              ;
            }
          }
        },

        get: {
          dimmer: function() {
            return $dimmer;
          },
          duration: function() {
            if(typeof settings.duration == 'object') {
              if( module.is.active() ) {
                return settings.duration.hide;
              }
              else {
                return settings.duration.show;
              }
            }
            return settings.duration;
          }
        },

        has: {
          dimmer: function() {
            if(settings.dimmerName) {
              return ($module.find(selector.dimmer).filter('.' + settings.dimmerName).length > 0);
            }
            else {
              return ( $module.find(selector.dimmer).length > 0 );
            }
          }
        },

        is: {
          active: function() {
            return $dimmer.hasClass(className.active);
          },
          animating: function() {
            return ( $dimmer.is(':animated') || $dimmer.hasClass(className.animating) );
          },
          closable: function() {
            if(settings.closable == 'auto') {
              if(settings.on == 'hover') {
                return false;
              }
              return true;
            }
            return settings.closable;
          },
          dimmer: function() {
            return $module.hasClass(className.dimmer);
          },
          dimmable: function() {
            return $module.hasClass(className.dimmable);
          },
          dimmed: function() {
            return $dimmable.hasClass(className.dimmed);
          },
          disabled: function() {
            return $dimmable.hasClass(className.disabled);
          },
          enabled: function() {
            return !module.is.disabled();
          },
          page: function () {
            return $dimmable.is('body');
          },
          pageDimmer: function() {
            return $dimmer.hasClass(className.pageDimmer);
          }
        },

        can: {
          show: function() {
            return !$dimmer.hasClass(className.disabled);
          }
        },

        set: {
          opacity: function(opacity) {
            var
              color      = $dimmer.css('background-color'),
              colorArray = color.split(','),
              isRGB      = (colorArray && colorArray.length == 3),
              isRGBA     = (colorArray && colorArray.length == 4)
            ;
            opacity    = settings.opacity === 0 ? 0 : settings.opacity || opacity;
            if(isRGB || isRGBA) {
              colorArray[3] = opacity + ')';
              color         = colorArray.join(',');
            }
            else {
              color = 'rgba(0, 0, 0, ' + opacity + ')';
            }
            module.debug('Setting opacity to', opacity);
            $dimmer.css('background-color', color);
          },
          legacy: function() {
            $dimmer.addClass(className.legacy);
          },
          active: function() {
            $dimmer.addClass(className.active);
          },
          dimmable: function() {
            $dimmable.addClass(className.dimmable);
          },
          dimmed: function() {
            $dimmable.addClass(className.dimmed);
          },
          pageDimmer: function() {
            $dimmer.addClass(className.pageDimmer);
          },
          disabled: function() {
            $dimmer.addClass(className.disabled);
          },
          variation: function(variation) {
            variation = variation || settings.variation;
            if(variation) {
              $dimmer.addClass(variation);
            }
          }
        },

        remove: {
          active: function() {
            $dimmer
              .removeClass(className.active)
            ;
          },
          legacy: function() {
            $dimmer.removeClass(className.legacy);
          },
          dimmed: function() {
            $dimmable.removeClass(className.dimmed);
          },
          disabled: function() {
            $dimmer.removeClass(className.disabled);
          },
          variation: function(variation) {
            variation = variation || settings.variation;
            if(variation) {
              $dimmer.removeClass(variation);
            }
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      module.preinitialize();

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.dimmer.settings = {

  name        : 'Dimmer',
  namespace   : 'dimmer',

  silent      : false,
  debug       : false,
  verbose     : false,
  performance : true,

  // whether should use flex layout
  useFlex     : true,

  // name to distinguish between multiple dimmers in context
  dimmerName  : false,

  // whether to add a variation type
  variation   : false,

  // whether to bind close events
  closable    : 'auto',

  // whether to use css animations
  useCSS      : true,

  // css animation to use
  transition  : 'fade',

  // event to bind to
  on          : false,

  // overriding opacity value
  opacity     : 'auto',

  // transition durations
  duration    : {
    show : 500,
    hide : 500
  },
// whether the dynamically created dimmer should have a loader
  displayLoader: false,
  loaderText  : false,
  loaderVariation : '',

  onChange    : function(){},
  onShow      : function(){},
  onHide      : function(){},

  error   : {
    method   : 'The method you called is not defined.'
  },

  className : {
    active     : 'active',
    animating  : 'animating',
    dimmable   : 'dimmable',
    dimmed     : 'dimmed',
    dimmer     : 'dimmer',
    disabled   : 'disabled',
    hide       : 'hide',
    legacy     : 'legacy',
    pageDimmer : 'page',
    show       : 'show',
    loader     : 'ui loader'
  },

  selector: {
    dimmer   : '> .ui.dimmer',
    content  : '.ui.dimmer > .content, .ui.dimmer > .content > .center'
  },

  template: {
    dimmer: function(settings) {
        var d = $('<div/>').addClass('ui dimmer'),l;
        if(settings.displayLoader) {
          l = $('<div/>')
              .addClass(settings.className.loader)
              .addClass(settings.loaderVariation);
          if(!!settings.loaderText){
            l.text(settings.loaderText);
            l.addClass('text');
          }
          d.append(l);
        }
        return d;
    }
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Dropdown
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

$.fn.dropdown = function(parameters) {
  var
    $allModules    = $(this),
    $document      = $(document),

    moduleSelector = $allModules.selector || '',

    hasTouch       = ('ontouchstart' in document.documentElement),
    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue
  ;

  $allModules
    .each(function(elementIndex) {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.dropdown.settings, parameters)
          : $.extend({}, $.fn.dropdown.settings),

        className       = settings.className,
        message         = settings.message,
        fields          = settings.fields,
        keys            = settings.keys,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        regExp          = settings.regExp,
        selector        = settings.selector,
        error           = settings.error,
        templates       = settings.templates,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $context        = $(settings.context),
        $text           = $module.find(selector.text),
        $search         = $module.find(selector.search),
        $sizer          = $module.find(selector.sizer),
        $input          = $module.find(selector.input),
        $icon           = $module.find(selector.icon),
        $clear          = $module.find(selector.clearIcon),

        $combo = ($module.prev().find(selector.text).length > 0)
          ? $module.prev().find(selector.text)
          : $module.prev(),

        $menu           = $module.children(selector.menu),
        $item           = $menu.find(selector.item),
        $divider        = settings.hideDividers ? $item.parent().children(selector.divider) : $(),

        activated       = false,
        itemActivated   = false,
        internalChange  = false,
        iconClicked     = false,
        element         = this,
        instance        = $module.data(moduleNamespace),

        selectActionActive,
        initialLoad,
        pageLostFocus,
        willRefocus,
        elementNamespace,
        id,
        selectObserver,
        menuObserver,
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing dropdown', settings);

          if( module.is.alreadySetup() ) {
            module.setup.reference();
          }
          else {
            if (settings.ignoreDiacritics && !String.prototype.normalize) {
              settings.ignoreDiacritics = false;
              module.error(error.noNormalize, element);
            }

            module.setup.layout();

            if(settings.values) {
              module.change.values(settings.values);
            }

            module.refreshData();

            module.save.defaults();
            module.restore.selected();

            module.create.id();
            module.bind.events();

            module.observeChanges();
            module.instantiate();
          }

        },

        instantiate: function() {
          module.verbose('Storing instance of dropdown', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous dropdown', $module);
          module.remove.tabbable();
          module.remove.active();
          $menu.transition('stop all');
          $menu.removeClass(className.visible).addClass(className.hidden);
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
          $menu
            .off(eventNamespace)
          ;
          $document
            .off(elementNamespace)
          ;
          module.disconnect.menuObserver();
          module.disconnect.selectObserver();
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            selectObserver = new MutationObserver(module.event.select.mutation);
            menuObserver   = new MutationObserver(module.event.menu.mutation);
            module.debug('Setting up mutation observer', selectObserver, menuObserver);
            module.observe.select();
            module.observe.menu();
          }
        },

        disconnect: {
          menuObserver: function() {
            if(menuObserver) {
              menuObserver.disconnect();
            }
          },
          selectObserver: function() {
            if(selectObserver) {
              selectObserver.disconnect();
            }
          }
        },
        observe: {
          select: function() {
            if(module.has.input() && selectObserver) {
              selectObserver.observe($module[0], {
                childList : true,
                subtree   : true
              });
            }
          },
          menu: function() {
            if(module.has.menu() && menuObserver) {
              menuObserver.observe($menu[0], {
                childList : true,
                subtree   : true
              });
            }
          }
        },

        create: {
          id: function() {
            id = (Math.random().toString(16) + '000000000').substr(2, 8);
            elementNamespace = '.' + id;
            module.verbose('Creating unique id for element', id);
          },
          userChoice: function(values) {
            var
              $userChoices,
              $userChoice,
              isUserValue,
              html
            ;
            values = values || module.get.userValues();
            if(!values) {
              return false;
            }
            values = Array.isArray(values)
              ? values
              : [values]
            ;
            $.each(values, function(index, value) {
              if(module.get.item(value) === false) {
                html         = settings.templates.addition( module.add.variables(message.addResult, value) );
                $userChoice  = $('<div />')
                  .html(html)
                  .attr('data-' + metadata.value, value)
                  .attr('data-' + metadata.text, value)
                  .addClass(className.addition)
                  .addClass(className.item)
                ;
                if(settings.hideAdditions) {
                  $userChoice.addClass(className.hidden);
                }
                $userChoices = ($userChoices === undefined)
                  ? $userChoice
                  : $userChoices.add($userChoice)
                ;
                module.verbose('Creating user choices for value', value, $userChoice);
              }
            });
            return $userChoices;
          },
          userLabels: function(value) {
            var
              userValues = module.get.userValues()
            ;
            if(userValues) {
              module.debug('Adding user labels', userValues);
              $.each(userValues, function(index, value) {
                module.verbose('Adding custom user value');
                module.add.label(value, value);
              });
            }
          },
          menu: function() {
            $menu = $('<div />')
              .addClass(className.menu)
              .appendTo($module)
            ;
          },
          sizer: function() {
            $sizer = $('<span />')
              .addClass(className.sizer)
              .insertAfter($search)
            ;
          }
        },

        search: function(query) {
          query = (query !== undefined)
            ? query
            : module.get.query()
          ;
          module.verbose('Searching for query', query);
          if(module.has.minCharacters(query)) {
            module.filter(query);
          }
          else {
            module.hide(null,true);
          }
        },

        select: {
          firstUnfiltered: function() {
            module.verbose('Selecting first non-filtered element');
            module.remove.selectedItem();
            $item
              .not(selector.unselectable)
              .not(selector.addition + selector.hidden)
                .eq(0)
                .addClass(className.selected)
            ;
          },
          nextAvailable: function($selected) {
            $selected = $selected.eq(0);
            var
              $nextAvailable = $selected.nextAll(selector.item).not(selector.unselectable).eq(0),
              $prevAvailable = $selected.prevAll(selector.item).not(selector.unselectable).eq(0),
              hasNext        = ($nextAvailable.length > 0)
            ;
            if(hasNext) {
              module.verbose('Moving selection to', $nextAvailable);
              $nextAvailable.addClass(className.selected);
            }
            else {
              module.verbose('Moving selection to', $prevAvailable);
              $prevAvailable.addClass(className.selected);
            }
          }
        },

        setup: {
          api: function() {
            var
              apiSettings = {
                debug   : settings.debug,
                urlData : {
                  value : module.get.value(),
                  query : module.get.query()
                },
                on    : false
              }
            ;
            module.verbose('First request, initializing API');
            $module
              .api(apiSettings)
            ;
          },
          layout: function() {
            if( $module.is('select') ) {
              module.setup.select();
              module.setup.returnedObject();
            }
            if( !module.has.menu() ) {
              module.create.menu();
            }
            if ( module.is.selection() && module.is.clearable() && !module.has.clearItem() ) {
              module.verbose('Adding clear icon');
              $clear = $('<i />')
                .addClass('remove icon')
                .insertBefore($text)
              ;
            }
            if( module.is.search() && !module.has.search() ) {
              module.verbose('Adding search input');
              $search = $('<input />')
                .addClass(className.search)
                .prop('autocomplete', 'off')
                .insertBefore($text)
              ;
            }
            if( module.is.multiple() && module.is.searchSelection() && !module.has.sizer()) {
              module.create.sizer();
            }
            if(settings.allowTab) {
              module.set.tabbable();
            }
          },
          select: function() {
            var
              selectValues  = module.get.selectValues()
            ;
            module.debug('Dropdown initialized on a select', selectValues);
            if( $module.is('select') ) {
              $input = $module;
            }
            // see if select is placed correctly already
            if($input.parent(selector.dropdown).length > 0) {
              module.debug('UI dropdown already exists. Creating dropdown menu only');
              $module = $input.closest(selector.dropdown);
              if( !module.has.menu() ) {
                module.create.menu();
              }
              $menu = $module.children(selector.menu);
              module.setup.menu(selectValues);
            }
            else {
              module.debug('Creating entire dropdown from select');
              $module = $('<div />')
                .attr('class', $input.attr('class') )
                .addClass(className.selection)
                .addClass(className.dropdown)
                .html( templates.dropdown(selectValues, fields, settings.preserveHTML, settings.className) )
                .insertBefore($input)
              ;
              if($input.hasClass(className.multiple) && $input.prop('multiple') === false) {
                module.error(error.missingMultiple);
                $input.prop('multiple', true);
              }
              if($input.is('[multiple]')) {
                module.set.multiple();
              }
              if ($input.prop('disabled')) {
                module.debug('Disabling dropdown');
                $module.addClass(className.disabled);
              }
              $input
                .removeAttr('required')
                .removeAttr('class')
                .detach()
                .prependTo($module)
              ;
            }
            module.refresh();
          },
          menu: function(values) {
            $menu.html( templates.menu(values, fields,settings.preserveHTML,settings.className));
            $item    = $menu.find(selector.item);
            $divider = settings.hideDividers ? $item.parent().children(selector.divider) : $();
          },
          reference: function() {
            module.debug('Dropdown behavior was called on select, replacing with closest dropdown');
            // replace module reference
            $module  = $module.parent(selector.dropdown);
            instance = $module.data(moduleNamespace);
            element  = $module.get(0);
            module.refresh();
            module.setup.returnedObject();
          },
          returnedObject: function() {
            var
              $firstModules = $allModules.slice(0, elementIndex),
              $lastModules  = $allModules.slice(elementIndex + 1)
            ;
            // adjust all modules to use correct reference
            $allModules = $firstModules.add($module).add($lastModules);
          }
        },

        refresh: function() {
          module.refreshSelectors();
          module.refreshData();
        },

        refreshItems: function() {
          $item    = $menu.find(selector.item);
          $divider = settings.hideDividers ? $item.parent().children(selector.divider) : $();
        },

        refreshSelectors: function() {
          module.verbose('Refreshing selector cache');
          $text   = $module.find(selector.text);
          $search = $module.find(selector.search);
          $input  = $module.find(selector.input);
          $icon   = $module.find(selector.icon);
          $combo  = ($module.prev().find(selector.text).length > 0)
            ? $module.prev().find(selector.text)
            : $module.prev()
          ;
          $menu    = $module.children(selector.menu);
          $item    = $menu.find(selector.item);
          $divider = settings.hideDividers ? $item.parent().children(selector.divider) : $();
        },

        refreshData: function() {
          module.verbose('Refreshing cached metadata');
          $item
            .removeData(metadata.text)
            .removeData(metadata.value)
          ;
        },

        clearData: function() {
          module.verbose('Clearing metadata');
          $item
            .removeData(metadata.text)
            .removeData(metadata.value)
          ;
          $module
            .removeData(metadata.defaultText)
            .removeData(metadata.defaultValue)
            .removeData(metadata.placeholderText)
          ;
        },

        toggle: function() {
          module.verbose('Toggling menu visibility');
          if( !module.is.active() ) {
            module.show();
          }
          else {
            module.hide();
          }
        },

        show: function(callback, preventFocus) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if(!module.can.show() && module.is.remote()) {
            module.debug('No API results retrieved, searching before show');
            module.queryRemote(module.get.query(), module.show);
          }
          if( module.can.show() && !module.is.active() ) {
            module.debug('Showing dropdown');
            if(module.has.message() && !(module.has.maxSelections() || module.has.allResultsFiltered()) ) {
              module.remove.message();
            }
            if(module.is.allFiltered()) {
              return true;
            }
            if(settings.onShow.call(element) !== false) {
              module.animate.show(function() {
                if( module.can.click() ) {
                  module.bind.intent();
                }
                if(module.has.search() && !preventFocus) {
                  module.focusSearch();
                }
                module.set.visible();
                callback.call(element);
              });
            }
          }
        },

        hide: function(callback, preventBlur) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( module.is.active() && !module.is.animatingOutward() ) {
            module.debug('Hiding dropdown');
            if(settings.onHide.call(element) !== false) {
              module.animate.hide(function() {
                module.remove.visible();
                // hidding search focus
                if ( module.is.focusedOnSearch() && preventBlur !== true ) {
                  $search.blur();
                }
                callback.call(element);
              });
            }
          } else if( module.can.click() ) {
              module.unbind.intent();
          }
        },

        hideOthers: function() {
          module.verbose('Finding other dropdowns to hide');
          $allModules
            .not($module)
              .has(selector.menu + '.' + className.visible)
                .dropdown('hide')
          ;
        },

        hideMenu: function() {
          module.verbose('Hiding menu  instantaneously');
          module.remove.active();
          module.remove.visible();
          $menu.transition('hide');
        },

        hideSubMenus: function() {
          var
            $subMenus = $menu.children(selector.item).find(selector.menu)
          ;
          module.verbose('Hiding sub menus', $subMenus);
          $subMenus.transition('hide');
        },

        bind: {
          events: function() {
            if(hasTouch) {
              module.bind.touchEvents();
            }
            module.bind.keyboardEvents();
            module.bind.inputEvents();
            module.bind.mouseEvents();
          },
          touchEvents: function() {
            module.debug('Touch device detected binding additional touch events');
            if( module.is.searchSelection() ) {
              // do nothing special yet
            }
            else if( module.is.single() ) {
              $module
                .on('touchstart' + eventNamespace, module.event.test.toggle)
              ;
            }
            $menu
              .on('touchstart' + eventNamespace, selector.item, module.event.item.mouseenter)
            ;
          },
          keyboardEvents: function() {
            module.verbose('Binding keyboard events');
            $module
              .on('keydown' + eventNamespace, module.event.keydown)
            ;
            if( module.has.search() ) {
              $module
                .on(module.get.inputEvent() + eventNamespace, selector.search, module.event.input)
              ;
            }
            if( module.is.multiple() ) {
              $document
                .on('keydown' + elementNamespace, module.event.document.keydown)
              ;
            }
          },
          inputEvents: function() {
            module.verbose('Binding input change events');
            $module
              .on('change' + eventNamespace, selector.input, module.event.change)
            ;
          },
          mouseEvents: function() {
            module.verbose('Binding mouse events');
            if(module.is.multiple()) {
              $module
                .on('click'   + eventNamespace, selector.label,  module.event.label.click)
                .on('click'   + eventNamespace, selector.remove, module.event.remove.click)
              ;
            }
            if( module.is.searchSelection() ) {
              $module
                .on('mousedown' + eventNamespace, module.event.mousedown)
                .on('mouseup'   + eventNamespace, module.event.mouseup)
                .on('mousedown' + eventNamespace, selector.menu,   module.event.menu.mousedown)
                .on('mouseup'   + eventNamespace, selector.menu,   module.event.menu.mouseup)
                .on('click'     + eventNamespace, selector.icon,   module.event.icon.click)
                .on('click'     + eventNamespace, selector.clearIcon, module.event.clearIcon.click)
                .on('focus'     + eventNamespace, selector.search, module.event.search.focus)
                .on('click'     + eventNamespace, selector.search, module.event.search.focus)
                .on('blur'      + eventNamespace, selector.search, module.event.search.blur)
                .on('click'     + eventNamespace, selector.text,   module.event.text.focus)
              ;
              if(module.is.multiple()) {
                $module
                  .on('click' + eventNamespace, module.event.click)
                ;
              }
            }
            else {
              if(settings.on == 'click') {
                $module
                  .on('click' + eventNamespace, selector.icon, module.event.icon.click)
                  .on('click' + eventNamespace, module.event.test.toggle)
                ;
              }
              else if(settings.on == 'hover') {
                $module
                  .on('mouseenter' + eventNamespace, module.delay.show)
                  .on('mouseleave' + eventNamespace, module.delay.hide)
                ;
              }
              else {
                $module
                  .on(settings.on + eventNamespace, module.toggle)
                ;
              }
              $module
                .on('mousedown' + eventNamespace, module.event.mousedown)
                .on('mouseup'   + eventNamespace, module.event.mouseup)
                .on('focus'     + eventNamespace, module.event.focus)
                .on('click'     + eventNamespace, selector.clearIcon, module.event.clearIcon.click)
              ;
              if(module.has.menuSearch() ) {
                $module
                  .on('blur' + eventNamespace, selector.search, module.event.search.blur)
                ;
              }
              else {
                $module
                  .on('blur' + eventNamespace, module.event.blur)
                ;
              }
            }
            $menu
              .on('mouseenter' + eventNamespace, selector.item, module.event.item.mouseenter)
              .on('mouseleave' + eventNamespace, selector.item, module.event.item.mouseleave)
              .on('click'      + eventNamespace, selector.item, module.event.item.click)
            ;
          },
          intent: function() {
            module.verbose('Binding hide intent event to document');
            if(hasTouch) {
              $document
                .on('touchstart' + elementNamespace, module.event.test.touch)
                .on('touchmove'  + elementNamespace, module.event.test.touch)
              ;
            }
            $document
              .on('click' + elementNamespace, module.event.test.hide)
            ;
          }
        },

        unbind: {
          intent: function() {
            module.verbose('Removing hide intent event from document');
            if(hasTouch) {
              $document
                .off('touchstart' + elementNamespace)
                .off('touchmove' + elementNamespace)
              ;
            }
            $document
              .off('click' + elementNamespace)
            ;
          }
        },

        filter: function(query) {
          var
            searchTerm = (query !== undefined)
              ? query
              : module.get.query(),
            afterFiltered = function() {
              if(module.is.multiple()) {
                module.filterActive();
              }
              if(query || (!query && module.get.activeItem().length == 0)) {
                module.select.firstUnfiltered();
              }
              if( module.has.allResultsFiltered() ) {
                if( settings.onNoResults.call(element, searchTerm) ) {
                  if(settings.allowAdditions) {
                    if(settings.hideAdditions) {
                      module.verbose('User addition with no menu, setting empty style');
                      module.set.empty();
                      module.hideMenu();
                    }
                  }
                  else {
                    module.verbose('All items filtered, showing message', searchTerm);
                    module.add.message(message.noResults);
                  }
                }
                else {
                  module.verbose('All items filtered, hiding dropdown', searchTerm);
                  module.hideMenu();
                }
              }
              else {
                module.remove.empty();
                module.remove.message();
              }
              if(settings.allowAdditions) {
                module.add.userSuggestion(module.escape.htmlEntities(query));
              }
              if(module.is.searchSelection() && module.can.show() && module.is.focusedOnSearch() ) {
                module.show();
              }
            }
          ;
          if(settings.useLabels && module.has.maxSelections()) {
            return;
          }
          if(settings.apiSettings) {
            if( module.can.useAPI() ) {
              module.queryRemote(searchTerm, function() {
                if(settings.filterRemoteData) {
                  module.filterItems(searchTerm);
                }
                var preSelected = $input.val();
                if(!Array.isArray(preSelected)) {
                    preSelected = preSelected && preSelected!=="" ? preSelected.split(settings.delimiter) : [];
                }
                $.each(preSelected,function(index,value){
                  $item.filter('[data-value="'+value+'"]')
                      .addClass(className.filtered)
                  ;
                });
                afterFiltered();
              });
            }
            else {
              module.error(error.noAPI);
            }
          }
          else {
            module.filterItems(searchTerm);
            afterFiltered();
          }
        },

        queryRemote: function(query, callback) {
          var
            apiSettings = {
              errorDuration : false,
              cache         : 'local',
              throttle      : settings.throttle,
              urlData       : {
                query: query
              },
              onError: function() {
                module.add.message(message.serverError);
                callback();
              },
              onFailure: function() {
                module.add.message(message.serverError);
                callback();
              },
              onSuccess : function(response) {
                var
                  values          = response[fields.remoteValues]
                ;
                if (!Array.isArray(values)){
                    values = [];
                }
                module.remove.message();
                module.setup.menu({
                  values: values
                });

                if(values.length===0 && !settings.allowAdditions) {
                  module.add.message(message.noResults);
                }
                callback();
              }
            }
          ;
          if( !$module.api('get request') ) {
            module.setup.api();
          }
          apiSettings = $.extend(true, {}, apiSettings, settings.apiSettings);
          $module
            .api('setting', apiSettings)
            .api('query')
          ;
        },

        filterItems: function(query) {
          var
            searchTerm = module.remove.diacritics(query !== undefined
              ? query
              : module.get.query()
            ),
            results          =  null,
            escapedTerm      = module.escape.string(searchTerm),
            regExpFlags      = (settings.ignoreSearchCase ? 'i' : '') + 'gm',
            beginsWithRegExp = new RegExp('^' + escapedTerm, regExpFlags)
          ;
          // avoid loop if we're matching nothing
          if( module.has.query() ) {
            results = [];

            module.verbose('Searching for matching values', searchTerm);
            $item
              .each(function(){
                var
                  $choice = $(this),
                  text,
                  value
                ;
                if(settings.match === 'both' || settings.match === 'text') {
                  text = module.remove.diacritics(String(module.get.choiceText($choice, false)));
                  if(text.search(beginsWithRegExp) !== -1) {
                    results.push(this);
                    return true;
                  }
                  else if (settings.fullTextSearch === 'exact' && module.exactSearch(searchTerm, text)) {
                    results.push(this);
                    return true;
                  }
                  else if (settings.fullTextSearch === true && module.fuzzySearch(searchTerm, text)) {
                    results.push(this);
                    return true;
                  }
                }
                if(settings.match === 'both' || settings.match === 'value') {
                  value = module.remove.diacritics(String(module.get.choiceValue($choice, text)));
                  if(value.search(beginsWithRegExp) !== -1) {
                    results.push(this);
                    return true;
                  }
                  else if (settings.fullTextSearch === 'exact' && module.exactSearch(searchTerm, value)) {
                    results.push(this);
                    return true;
                  }
                  else if (settings.fullTextSearch === true && module.fuzzySearch(searchTerm, value)) {
                    results.push(this);
                    return true;
                  }
                }
              })
            ;
          }
          module.debug('Showing only matched items', searchTerm);
          module.remove.filteredItem();
          if(results) {
            $item
              .not(results)
              .addClass(className.filtered)
            ;
          }

          if(!module.has.query()) {
            $divider
              .removeClass(className.hidden);
          } else if(settings.hideDividers === true) {
            $divider
              .addClass(className.hidden);
          } else if(settings.hideDividers === 'empty') {
            $divider
              .removeClass(className.hidden)
              .filter(function() {
                // First find the last divider in this divider group
                // Dividers which are direct siblings are considered a group
                var lastDivider = $(this).nextUntil(selector.item);

                return (lastDivider.length ? lastDivider : $(this))
                // Count all non-filtered items until the next divider (or end of the dropdown)
                  .nextUntil(selector.divider)
                  .filter(selector.item + ":not(." + className.filtered + ")")
                  // Hide divider if no items are found
                  .length === 0;
              })
              .addClass(className.hidden);
          }
        },

        fuzzySearch: function(query, term) {
          var
            termLength  = term.length,
            queryLength = query.length
          ;
          query = (settings.ignoreSearchCase ? query.toLowerCase() : query);
          term  = (settings.ignoreSearchCase ? term.toLowerCase() : term);
          if(queryLength > termLength) {
            return false;
          }
          if(queryLength === termLength) {
            return (query === term);
          }
          search: for (var characterIndex = 0, nextCharacterIndex = 0; characterIndex < queryLength; characterIndex++) {
            var
              queryCharacter = query.charCodeAt(characterIndex)
            ;
            while(nextCharacterIndex < termLength) {
              if(term.charCodeAt(nextCharacterIndex++) === queryCharacter) {
                continue search;
              }
            }
            return false;
          }
          return true;
        },
        exactSearch: function (query, term) {
          query = (settings.ignoreSearchCase ? query.toLowerCase() : query);
          term  = (settings.ignoreSearchCase ? term.toLowerCase() : term);
          return term.indexOf(query) > -1;

        },
        filterActive: function() {
          if(settings.useLabels) {
            $item.filter('.' + className.active)
              .addClass(className.filtered)
            ;
          }
        },

        focusSearch: function(skipHandler) {
          if( module.has.search() && !module.is.focusedOnSearch() ) {
            if(skipHandler) {
              $module.off('focus' + eventNamespace, selector.search);
              $search.focus();
              $module.on('focus'  + eventNamespace, selector.search, module.event.search.focus);
            }
            else {
              $search.focus();
            }
          }
        },

        blurSearch: function() {
          if( module.has.search() ) {
            $search.blur();
          }
        },

        forceSelection: function() {
          var
            $currentlySelected = $item.not(className.filtered).filter('.' + className.selected).eq(0),
            $activeItem        = $item.not(className.filtered).filter('.' + className.active).eq(0),
            $selectedItem      = ($currentlySelected.length > 0)
              ? $currentlySelected
              : $activeItem,
            hasSelected = ($selectedItem.length > 0)
          ;
          if(settings.allowAdditions || (hasSelected && !module.is.multiple())) {
            module.debug('Forcing partial selection to selected item', $selectedItem);
            module.event.item.click.call($selectedItem, {}, true);
          }
          else {
            module.remove.searchTerm();
          }
        },

        change: {
          values: function(values) {
            if(!settings.allowAdditions) {
              module.clear();
            }
            module.debug('Creating dropdown with specified values', values);
            module.setup.menu({values: values});
            $.each(values, function(index, item) {
              if(item.selected == true) {
                module.debug('Setting initial selection to', item[fields.value]);
                module.set.selected(item[fields.value]);
                if(!module.is.multiple()) {
                  return false;
                }
              }
            });

            if(module.has.selectInput()) {
              module.disconnect.selectObserver();
              $input.html('');
              $input.append('<option disabled selected value></option>');
              $.each(values, function(index, item) {
                var
                  value = settings.templates.deQuote(item[fields.value]),
                  name = settings.templates.escape(
                    item[fields.name] || item[fields.value],
                    settings.preserveHTML
                  )
                ;
                $input.append('<option value="' + value + '">' + name + '</option>');
              });
              module.observe.select();
            }
          }
        },

        event: {
          change: function() {
            if(!internalChange) {
              module.debug('Input changed, updating selection');
              module.set.selected();
            }
          },
          focus: function() {
            if(settings.showOnFocus && !activated && module.is.hidden() && !pageLostFocus) {
              module.show();
            }
          },
          blur: function(event) {
            pageLostFocus = (document.activeElement === this);
            if(!activated && !pageLostFocus) {
              module.remove.activeLabel();
              module.hide();
            }
          },
          mousedown: function() {
            if(module.is.searchSelection()) {
              // prevent menu hiding on immediate re-focus
              willRefocus = true;
            }
            else {
              // prevents focus callback from occurring on mousedown
              activated = true;
            }
          },
          mouseup: function() {
            if(module.is.searchSelection()) {
              // prevent menu hiding on immediate re-focus
              willRefocus = false;
            }
            else {
              activated = false;
            }
          },
          click: function(event) {
            var
              $target = $(event.target)
            ;
            // focus search
            if($target.is($module)) {
              if(!module.is.focusedOnSearch()) {
                module.focusSearch();
              }
              else {
                module.show();
              }
            }
          },
          search: {
            focus: function(event) {
              activated = true;
              if(module.is.multiple()) {
                module.remove.activeLabel();
              }
              if(settings.showOnFocus || (event.type !== 'focus' && event.type !== 'focusin')) {
                module.search();
              }
            },
            blur: function(event) {
              pageLostFocus = (document.activeElement === this);
              if(module.is.searchSelection() && !willRefocus) {
                if(!itemActivated && !pageLostFocus) {
                  if(settings.forceSelection) {
                    module.forceSelection();
                  } else if(!settings.allowAdditions){
                    module.remove.searchTerm();
                  }
                  module.hide();
                }
              }
              willRefocus = false;
            }
          },
          clearIcon: {
            click: function(event) {
              module.clear();
              if(module.is.searchSelection()) {
                module.remove.searchTerm();
              }
              module.hide();
              event.stopPropagation();
            }
          },
          icon: {
            click: function(event) {
              iconClicked=true;
              if(module.has.search()) {
                if(!module.is.active()) {
                    if(settings.showOnFocus){
                      module.focusSearch();
                    } else {
                      module.toggle();
                    }
                } else {
                  module.blurSearch();
                }
              } else {
                module.toggle();
              }
            }
          },
          text: {
            focus: function(event) {
              activated = true;
              module.focusSearch();
            }
          },
          input: function(event) {
            if(module.is.multiple() || module.is.searchSelection()) {
              module.set.filtered();
            }
            clearTimeout(module.timer);
            module.timer = setTimeout(module.search, settings.delay.search);
          },
          label: {
            click: function(event) {
              var
                $label        = $(this),
                $labels       = $module.find(selector.label),
                $activeLabels = $labels.filter('.' + className.active),
                $nextActive   = $label.nextAll('.' + className.active),
                $prevActive   = $label.prevAll('.' + className.active),
                $range = ($nextActive.length > 0)
                  ? $label.nextUntil($nextActive).add($activeLabels).add($label)
                  : $label.prevUntil($prevActive).add($activeLabels).add($label)
              ;
              if(event.shiftKey) {
                $activeLabels.removeClass(className.active);
                $range.addClass(className.active);
              }
              else if(event.ctrlKey) {
                $label.toggleClass(className.active);
              }
              else {
                $activeLabels.removeClass(className.active);
                $label.addClass(className.active);
              }
              settings.onLabelSelect.apply(this, $labels.filter('.' + className.active));
            }
          },
          remove: {
            click: function() {
              var
                $label = $(this).parent()
              ;
              if( $label.hasClass(className.active) ) {
                // remove all selected labels
                module.remove.activeLabels();
              }
              else {
                // remove this label only
                module.remove.activeLabels( $label );
              }
            }
          },
          test: {
            toggle: function(event) {
              var
                toggleBehavior = (module.is.multiple())
                  ? module.show
                  : module.toggle
              ;
              if(module.is.bubbledLabelClick(event) || module.is.bubbledIconClick(event)) {
                return;
              }
              if( module.determine.eventOnElement(event, toggleBehavior) ) {
                event.preventDefault();
              }
            },
            touch: function(event) {
              module.determine.eventOnElement(event, function() {
                if(event.type == 'touchstart') {
                  module.timer = setTimeout(function() {
                    module.hide();
                  }, settings.delay.touch);
                }
                else if(event.type == 'touchmove') {
                  clearTimeout(module.timer);
                }
              });
              event.stopPropagation();
            },
            hide: function(event) {
              if(module.determine.eventInModule(event, module.hide)){
                if(element.id && $(event.target).attr('for') === element.id){
                  event.preventDefault();
                }
              }
            }
          },
          select: {
            mutation: function(mutations) {
              module.debug('<select> modified, recreating menu');
              if(module.is.selectMutation(mutations)) {
                module.disconnect.selectObserver();
                module.refresh();
                module.setup.select();
                module.set.selected();
                module.observe.select();
              }
            }
          },
          menu: {
            mutation: function(mutations) {
              var
                mutation   = mutations[0],
                $addedNode = mutation.addedNodes
                  ? $(mutation.addedNodes[0])
                  : $(false),
                $removedNode = mutation.removedNodes
                  ? $(mutation.removedNodes[0])
                  : $(false),
                $changedNodes  = $addedNode.add($removedNode),
                isUserAddition = $changedNodes.is(selector.addition) || $changedNodes.closest(selector.addition).length > 0,
                isMessage      = $changedNodes.is(selector.message)  || $changedNodes.closest(selector.message).length > 0
              ;
              if(isUserAddition || isMessage) {
                module.debug('Updating item selector cache');
                module.refreshItems();
              }
              else {
                module.debug('Menu modified, updating selector cache');
                module.refresh();
              }
            },
            mousedown: function() {
              itemActivated = true;
            },
            mouseup: function() {
              itemActivated = false;
            }
          },
          item: {
            mouseenter: function(event) {
              var
                $target        = $(event.target),
                $item          = $(this),
                $subMenu       = $item.children(selector.menu),
                $otherMenus    = $item.siblings(selector.item).children(selector.menu),
                hasSubMenu     = ($subMenu.length > 0),
                isBubbledEvent = ($subMenu.find($target).length > 0)
              ;
              if( !isBubbledEvent && hasSubMenu ) {
                clearTimeout(module.itemTimer);
                module.itemTimer = setTimeout(function() {
                  module.verbose('Showing sub-menu', $subMenu);
                  $.each($otherMenus, function() {
                    module.animate.hide(false, $(this));
                  });
                  module.animate.show(false, $subMenu);
                }, settings.delay.show);
                event.preventDefault();
              }
            },
            mouseleave: function(event) {
              var
                $subMenu = $(this).children(selector.menu)
              ;
              if($subMenu.length > 0) {
                clearTimeout(module.itemTimer);
                module.itemTimer = setTimeout(function() {
                  module.verbose('Hiding sub-menu', $subMenu);
                  module.animate.hide(false, $subMenu);
                }, settings.delay.hide);
              }
            },
            click: function (event, skipRefocus) {
              var
                $choice        = $(this),
                $target        = (event)
                  ? $(event.target)
                  : $(''),
                $subMenu       = $choice.find(selector.menu),
                text           = module.get.choiceText($choice),
                value          = module.get.choiceValue($choice, text),
                hasSubMenu     = ($subMenu.length > 0),
                isBubbledEvent = ($subMenu.find($target).length > 0)
              ;
              // prevents IE11 bug where menu receives focus even though `tabindex=-1`
              if (document.activeElement.tagName.toLowerCase() !== 'input') {
                $(document.activeElement).blur();
              }
              if(!isBubbledEvent && (!hasSubMenu || settings.allowCategorySelection)) {
                if(module.is.searchSelection()) {
                  if(settings.allowAdditions) {
                    module.remove.userAddition();
                  }
                  module.remove.searchTerm();
                  if(!module.is.focusedOnSearch() && !(skipRefocus == true)) {
                    module.focusSearch(true);
                  }
                }
                if(!settings.useLabels) {
                  module.remove.filteredItem();
                  module.set.scrollPosition($choice);
                }
                module.determine.selectAction.call(this, text, value);
              }
            }
          },

          document: {
            // label selection should occur even when element has no focus
            keydown: function(event) {
              var
                pressedKey    = event.which,
                isShortcutKey = module.is.inObject(pressedKey, keys)
              ;
              if(isShortcutKey) {
                var
                  $label            = $module.find(selector.label),
                  $activeLabel      = $label.filter('.' + className.active),
                  activeValue       = $activeLabel.data(metadata.value),
                  labelIndex        = $label.index($activeLabel),
                  labelCount        = $label.length,
                  hasActiveLabel    = ($activeLabel.length > 0),
                  hasMultipleActive = ($activeLabel.length > 1),
                  isFirstLabel      = (labelIndex === 0),
                  isLastLabel       = (labelIndex + 1 == labelCount),
                  isSearch          = module.is.searchSelection(),
                  isFocusedOnSearch = module.is.focusedOnSearch(),
                  isFocused         = module.is.focused(),
                  caretAtStart      = (isFocusedOnSearch && module.get.caretPosition(false) === 0),
                  isSelectedSearch  = (caretAtStart && module.get.caretPosition(true) !== 0),
                  $nextLabel
                ;
                if(isSearch && !hasActiveLabel && !isFocusedOnSearch) {
                  return;
                }

                if(pressedKey == keys.leftArrow) {
                  // activate previous label
                  if((isFocused || caretAtStart) && !hasActiveLabel) {
                    module.verbose('Selecting previous label');
                    $label.last().addClass(className.active);
                  }
                  else if(hasActiveLabel) {
                    if(!event.shiftKey) {
                      module.verbose('Selecting previous label');
                      $label.removeClass(className.active);
                    }
                    else {
                      module.verbose('Adding previous label to selection');
                    }
                    if(isFirstLabel && !hasMultipleActive) {
                      $activeLabel.addClass(className.active);
                    }
                    else {
                      $activeLabel.prev(selector.siblingLabel)
                        .addClass(className.active)
                        .end()
                      ;
                    }
                    event.preventDefault();
                  }
                }
                else if(pressedKey == keys.rightArrow) {
                  // activate first label
                  if(isFocused && !hasActiveLabel) {
                    $label.first().addClass(className.active);
                  }
                  // activate next label
                  if(hasActiveLabel) {
                    if(!event.shiftKey) {
                      module.verbose('Selecting next label');
                      $label.removeClass(className.active);
                    }
                    else {
                      module.verbose('Adding next label to selection');
                    }
                    if(isLastLabel) {
                      if(isSearch) {
                        if(!isFocusedOnSearch) {
                          module.focusSearch();
                        }
                        else {
                          $label.removeClass(className.active);
                        }
                      }
                      else if(hasMultipleActive) {
                        $activeLabel.next(selector.siblingLabel).addClass(className.active);
                      }
                      else {
                        $activeLabel.addClass(className.active);
                      }
                    }
                    else {
                      $activeLabel.next(selector.siblingLabel).addClass(className.active);
                    }
                    event.preventDefault();
                  }
                }
                else if(pressedKey == keys.deleteKey || pressedKey == keys.backspace) {
                  if(hasActiveLabel) {
                    module.verbose('Removing active labels');
                    if(isLastLabel) {
                      if(isSearch && !isFocusedOnSearch) {
                        module.focusSearch();
                      }
                    }
                    $activeLabel.last().next(selector.siblingLabel).addClass(className.active);
                    module.remove.activeLabels($activeLabel);
                    event.preventDefault();
                  }
                  else if(caretAtStart && !isSelectedSearch && !hasActiveLabel && pressedKey == keys.backspace) {
                    module.verbose('Removing last label on input backspace');
                    $activeLabel = $label.last().addClass(className.active);
                    module.remove.activeLabels($activeLabel);
                  }
                }
                else {
                  $activeLabel.removeClass(className.active);
                }
              }
            }
          },

          keydown: function(event) {
            var
              pressedKey    = event.which,
              isShortcutKey = module.is.inObject(pressedKey, keys)
            ;
            if(isShortcutKey) {
              var
                $currentlySelected = $item.not(selector.unselectable).filter('.' + className.selected).eq(0),
                $activeItem        = $menu.children('.' + className.active).eq(0),
                $selectedItem      = ($currentlySelected.length > 0)
                  ? $currentlySelected
                  : $activeItem,
                $visibleItems = ($selectedItem.length > 0)
                  ? $selectedItem.siblings(':not(.' + className.filtered +')').addBack()
                  : $menu.children(':not(.' + className.filtered +')'),
                $subMenu              = $selectedItem.children(selector.menu),
                $parentMenu           = $selectedItem.closest(selector.menu),
                inVisibleMenu         = ($parentMenu.hasClass(className.visible) || $parentMenu.hasClass(className.animating) || $parentMenu.parent(selector.menu).length > 0),
                hasSubMenu            = ($subMenu.length> 0),
                hasSelectedItem       = ($selectedItem.length > 0),
                selectedIsSelectable  = ($selectedItem.not(selector.unselectable).length > 0),
                delimiterPressed      = (pressedKey == keys.delimiter && settings.allowAdditions && module.is.multiple()),
                isAdditionWithoutMenu = (settings.allowAdditions && settings.hideAdditions && (pressedKey == keys.enter || delimiterPressed) && selectedIsSelectable),
                $nextItem,
                isSubMenuItem,
                newIndex
              ;
              // allow selection with menu closed
              if(isAdditionWithoutMenu) {
                module.verbose('Selecting item from keyboard shortcut', $selectedItem);
                module.event.item.click.call($selectedItem, event);
                if(module.is.searchSelection()) {
                  module.remove.searchTerm();
                }
                if(module.is.multiple()){
                    event.preventDefault();
                }
              }

              // visible menu keyboard shortcuts
              if( module.is.visible() ) {

                // enter (select or open sub-menu)
                if(pressedKey == keys.enter || delimiterPressed) {
                  if(pressedKey == keys.enter && hasSelectedItem && hasSubMenu && !settings.allowCategorySelection) {
                    module.verbose('Pressed enter on unselectable category, opening sub menu');
                    pressedKey = keys.rightArrow;
                  }
                  else if(selectedIsSelectable) {
                    module.verbose('Selecting item from keyboard shortcut', $selectedItem);
                    module.event.item.click.call($selectedItem, event);
                    if(module.is.searchSelection()) {
                      module.remove.searchTerm();
                      if(module.is.multiple()) {
                          $search.focus();
                      }
                    }
                  }
                  event.preventDefault();
                }

                // sub-menu actions
                if(hasSelectedItem) {

                  if(pressedKey == keys.leftArrow) {

                    isSubMenuItem = ($parentMenu[0] !== $menu[0]);

                    if(isSubMenuItem) {
                      module.verbose('Left key pressed, closing sub-menu');
                      module.animate.hide(false, $parentMenu);
                      $selectedItem
                        .removeClass(className.selected)
                      ;
                      $parentMenu
                        .closest(selector.item)
                          .addClass(className.selected)
                      ;
                      event.preventDefault();
                    }
                  }

                  // right arrow (show sub-menu)
                  if(pressedKey == keys.rightArrow) {
                    if(hasSubMenu) {
                      module.verbose('Right key pressed, opening sub-menu');
                      module.animate.show(false, $subMenu);
                      $selectedItem
                        .removeClass(className.selected)
                      ;
                      $subMenu
                        .find(selector.item).eq(0)
                          .addClass(className.selected)
                      ;
                      event.preventDefault();
                    }
                  }
                }

                // up arrow (traverse menu up)
                if(pressedKey == keys.upArrow) {
                  $nextItem = (hasSelectedItem && inVisibleMenu)
                    ? $selectedItem.prevAll(selector.item + ':not(' + selector.unselectable + ')').eq(0)
                    : $item.eq(0)
                  ;
                  if($visibleItems.index( $nextItem ) < 0) {
                    module.verbose('Up key pressed but reached top of current menu');
                    event.preventDefault();
                    return;
                  }
                  else {
                    module.verbose('Up key pressed, changing active item');
                    $selectedItem
                      .removeClass(className.selected)
                    ;
                    $nextItem
                      .addClass(className.selected)
                    ;
                    module.set.scrollPosition($nextItem);
                    if(settings.selectOnKeydown && module.is.single()) {
                      module.set.selectedItem($nextItem);
                    }
                  }
                  event.preventDefault();
                }

                // down arrow (traverse menu down)
                if(pressedKey == keys.downArrow) {
                  $nextItem = (hasSelectedItem && inVisibleMenu)
                    ? $nextItem = $selectedItem.nextAll(selector.item + ':not(' + selector.unselectable + ')').eq(0)
                    : $item.eq(0)
                  ;
                  if($nextItem.length === 0) {
                    module.verbose('Down key pressed but reached bottom of current menu');
                    event.preventDefault();
                    return;
                  }
                  else {
                    module.verbose('Down key pressed, changing active item');
                    $item
                      .removeClass(className.selected)
                    ;
                    $nextItem
                      .addClass(className.selected)
                    ;
                    module.set.scrollPosition($nextItem);
                    if(settings.selectOnKeydown && module.is.single()) {
                      module.set.selectedItem($nextItem);
                    }
                  }
                  event.preventDefault();
                }

                // page down (show next page)
                if(pressedKey == keys.pageUp) {
                  module.scrollPage('up');
                  event.preventDefault();
                }
                if(pressedKey == keys.pageDown) {
                  module.scrollPage('down');
                  event.preventDefault();
                }

                // escape (close menu)
                if(pressedKey == keys.escape) {
                  module.verbose('Escape key pressed, closing dropdown');
                  module.hide();
                }

              }
              else {
                // delimiter key
                if(delimiterPressed) {
                  event.preventDefault();
                }
                // down arrow (open menu)
                if(pressedKey == keys.downArrow && !module.is.visible()) {
                  module.verbose('Down key pressed, showing dropdown');
                  module.show();
                  event.preventDefault();
                }
              }
            }
            else {
              if( !module.has.search() ) {
                module.set.selectedLetter( String.fromCharCode(pressedKey) );
              }
            }
          }
        },

        trigger: {
          change: function() {
            var
              events       = document.createEvent('HTMLEvents'),
              inputElement = $input[0]
            ;
            if(inputElement) {
              module.verbose('Triggering native change event');
              events.initEvent('change', true, false);
              inputElement.dispatchEvent(events);
            }
          }
        },

        determine: {
          selectAction: function(text, value) {
            selectActionActive = true;
            module.verbose('Determining action', settings.action);
            if( $.isFunction( module.action[settings.action] ) ) {
              module.verbose('Triggering preset action', settings.action, text, value);
              module.action[ settings.action ].call(element, text, value, this);
            }
            else if( $.isFunction(settings.action) ) {
              module.verbose('Triggering user action', settings.action, text, value);
              settings.action.call(element, text, value, this);
            }
            else {
              module.error(error.action, settings.action);
            }
            selectActionActive = false;
          },
          eventInModule: function(event, callback) {
            var
              $target    = $(event.target),
              inDocument = ($target.closest(document.documentElement).length > 0),
              inModule   = ($target.closest($module).length > 0)
            ;
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if(inDocument && !inModule) {
              module.verbose('Triggering event', callback);
              callback();
              return true;
            }
            else {
              module.verbose('Event occurred in dropdown, canceling callback');
              return false;
            }
          },
          eventOnElement: function(event, callback) {
            var
              $target      = $(event.target),
              $label       = $target.closest(selector.siblingLabel),
              inVisibleDOM = document.body.contains(event.target),
              notOnLabel   = ($module.find($label).length === 0 || !(module.is.multiple() && settings.useLabels)),
              notInMenu    = ($target.closest($menu).length === 0)
            ;
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if(inVisibleDOM && notOnLabel && notInMenu) {
              module.verbose('Triggering event', callback);
              callback();
              return true;
            }
            else {
              module.verbose('Event occurred in dropdown menu, canceling callback');
              return false;
            }
          }
        },

        action: {

          nothing: function() {},

          activate: function(text, value, element) {
            value = (value !== undefined)
              ? value
              : text
            ;
            if( module.can.activate( $(element) ) ) {
              module.set.selected(value, $(element));
              if(!module.is.multiple()) {
                module.hideAndClear();
              }
            }
          },

          select: function(text, value, element) {
            value = (value !== undefined)
              ? value
              : text
            ;
            if( module.can.activate( $(element) ) ) {
              module.set.value(value, text, $(element));
              if(!module.is.multiple()) {
                module.hideAndClear();
              }
            }
          },

          combo: function(text, value, element) {
            value = (value !== undefined)
              ? value
              : text
            ;
            module.set.selected(value, $(element));
            module.hideAndClear();
          },

          hide: function(text, value, element) {
            module.set.value(value, text, $(element));
            module.hideAndClear();
          }

        },

        get: {
          id: function() {
            return id;
          },
          defaultText: function() {
            return $module.data(metadata.defaultText);
          },
          defaultValue: function() {
            return $module.data(metadata.defaultValue);
          },
          placeholderText: function() {
            if(settings.placeholder != 'auto' && typeof settings.placeholder == 'string') {
              return settings.placeholder;
            }
            return $module.data(metadata.placeholderText) || '';
          },
          text: function() {
            return $text.text();
          },
          query: function() {
            return $.trim($search.val());
          },
          searchWidth: function(value) {
            value = (value !== undefined)
              ? value
              : $search.val()
            ;
            $sizer.text(value);
            // prevent rounding issues
            return Math.ceil( $sizer.width() + 1);
          },
          selectionCount: function() {
            var
              values = module.get.values(),
              count
            ;
            count = ( module.is.multiple() )
              ? Array.isArray(values)
                ? values.length
                : 0
              : (module.get.value() !== '')
                ? 1
                : 0
            ;
            return count;
          },
          transition: function($subMenu) {
            return (settings.transition == 'auto')
              ? module.is.upward($subMenu)
                ? 'slide up'
                : 'slide down'
              : settings.transition
            ;
          },
          userValues: function() {
            var
              values = module.get.values()
            ;
            if(!values) {
              return false;
            }
            values = Array.isArray(values)
              ? values
              : [values]
            ;
            return $.grep(values, function(value) {
              return (module.get.item(value) === false);
            });
          },
          uniqueArray: function(array) {
            return $.grep(array, function (value, index) {
                return $.inArray(value, array) === index;
            });
          },
          caretPosition: function(returnEndPos) {
            var
              input = $search.get(0),
              range,
              rangeLength
            ;
            if(returnEndPos && 'selectionEnd' in input){
              return input.selectionEnd;
            }
            else if(!returnEndPos && 'selectionStart' in input) {
              return input.selectionStart;
            }
            if (document.selection) {
              input.focus();
              range       = document.selection.createRange();
              rangeLength = range.text.length;
              if(returnEndPos) {
                return rangeLength;
              }
              range.moveStart('character', -input.value.length);
              return range.text.length - rangeLength;
            }
          },
          value: function() {
            var
              value = ($input.length > 0)
                ? $input.val()
                : $module.data(metadata.value),
              isEmptyMultiselect = (Array.isArray(value) && value.length === 1 && value[0] === '')
            ;
            // prevents placeholder element from being selected when multiple
            return (value === undefined || isEmptyMultiselect)
              ? ''
              : value
            ;
          },
          values: function() {
            var
              value = module.get.value()
            ;
            if(value === '') {
              return '';
            }
            return ( !module.has.selectInput() && module.is.multiple() )
              ? (typeof value == 'string') // delimited string
                ? module.escape.htmlEntities(value).split(settings.delimiter)
                : ''
              : value
            ;
          },
          remoteValues: function() {
            var
              values = module.get.values(),
              remoteValues = false
            ;
            if(values) {
              if(typeof values == 'string') {
                values = [values];
              }
              $.each(values, function(index, value) {
                var
                  name = module.read.remoteData(value)
                ;
                module.verbose('Restoring value from session data', name, value);
                if(name) {
                  if(!remoteValues) {
                    remoteValues = {};
                  }
                  remoteValues[value] = name;
                }
              });
            }
            return remoteValues;
          },
          choiceText: function($choice, preserveHTML) {
            preserveHTML = (preserveHTML !== undefined)
              ? preserveHTML
              : settings.preserveHTML
            ;
            if($choice) {
              if($choice.find(selector.menu).length > 0) {
                module.verbose('Retrieving text of element with sub-menu');
                $choice = $choice.clone();
                $choice.find(selector.menu).remove();
                $choice.find(selector.menuIcon).remove();
              }
              return ($choice.data(metadata.text) !== undefined)
                ? $choice.data(metadata.text)
                : (preserveHTML)
                  ? $.trim($choice.html())
                  : $.trim($choice.text())
              ;
            }
          },
          choiceValue: function($choice, choiceText) {
            choiceText = choiceText || module.get.choiceText($choice);
            if(!$choice) {
              return false;
            }
            return ($choice.data(metadata.value) !== undefined)
              ? String( $choice.data(metadata.value) )
              : (typeof choiceText === 'string')
                ? $.trim(
                  settings.ignoreSearchCase
                  ? choiceText.toLowerCase()
                  : choiceText
                )
                : String(choiceText)
            ;
          },
          inputEvent: function() {
            var
              input = $search[0]
            ;
            if(input) {
              return (input.oninput !== undefined)
                ? 'input'
                : (input.onpropertychange !== undefined)
                  ? 'propertychange'
                  : 'keyup'
              ;
            }
            return false;
          },
          selectValues: function() {
            var
              select = {},
              oldGroup = []
            ;
            select.values = [];
            $module
              .find('option')
                .each(function() {
                  var
                    $option  = $(this),
                    name     = $option.html(),
                    disabled = $option.attr('disabled'),
                    value    = ( $option.attr('value') !== undefined )
                      ? $option.attr('value')
                      : name,
                    group = $option.parent('optgroup')
                  ;
                  if(settings.placeholder === 'auto' && value === '') {
                    select.placeholder = name;
                  }
                  else {
                    if(group.length !== oldGroup.length || group[0] !== oldGroup[0]) {
                      select.values.push({
                        type: 'header',
                        divider: settings.headerDivider,
                        name: group.attr('label') || ''
                      });
                      oldGroup = group;
                    }
                    select.values.push({
                      name     : name,
                      value    : value,
                      disabled : disabled
                    });
                  }
                })
            ;
            if(settings.placeholder && settings.placeholder !== 'auto') {
              module.debug('Setting placeholder value to', settings.placeholder);
              select.placeholder = settings.placeholder;
            }
            if(settings.sortSelect) {
              if(settings.sortSelect === true) {
                select.values.sort(function(a, b) {
                  return a.name.localeCompare(b.name);
                });
              } else if(settings.sortSelect === 'natural') {
                select.values.sort(function(a, b) {
                  return (a.name.toLowerCase().localeCompare(b.name.toLowerCase()));
                });
              } else if($.isFunction(settings.sortSelect)) {
                select.values.sort(settings.sortSelect);
              }
              module.debug('Retrieved and sorted values from select', select);
            }
            else {
              module.debug('Retrieved values from select', select);
            }
            return select;
          },
          activeItem: function() {
            return $item.filter('.'  + className.active);
          },
          selectedItem: function() {
            var
              $selectedItem = $item.not(selector.unselectable).filter('.'  + className.selected)
            ;
            return ($selectedItem.length > 0)
              ? $selectedItem
              : $item.eq(0)
            ;
          },
          itemWithAdditions: function(value) {
            var
              $items       = module.get.item(value),
              $userItems   = module.create.userChoice(value),
              hasUserItems = ($userItems && $userItems.length > 0)
            ;
            if(hasUserItems) {
              $items = ($items.length > 0)
                ? $items.add($userItems)
                : $userItems
              ;
            }
            return $items;
          },
          item: function(value, strict) {
            var
              $selectedItem = false,
              shouldSearch,
              isMultiple
            ;
            value = (value !== undefined)
              ? value
              : ( module.get.values() !== undefined)
                ? module.get.values()
                : module.get.text()
            ;
            isMultiple = (module.is.multiple() && Array.isArray(value));
            shouldSearch = (isMultiple)
              ? (value.length > 0)
              : (value !== undefined && value !== null)
            ;
            strict     = (value === '' || value === false  || value === true)
              ? true
              : strict || false
            ;
            if(shouldSearch) {
              $item
                .each(function() {
                  var
                    $choice       = $(this),
                    optionText    = module.get.choiceText($choice),
                    optionValue   = module.get.choiceValue($choice, optionText)
                  ;
                  // safe early exit
                  if(optionValue === null || optionValue === undefined) {
                    return;
                  }
                  if(isMultiple) {
                    if($.inArray( String(optionValue), value) !== -1) {
                      $selectedItem = ($selectedItem)
                        ? $selectedItem.add($choice)
                        : $choice
                      ;
                    }
                  }
                  else if(strict) {
                    module.verbose('Ambiguous dropdown value using strict type check', $choice, value);
                    if( optionValue === value) {
                      $selectedItem = $choice;
                      return true;
                    }
                  }
                  else {
                    if(settings.ignoreCase) {
                      optionValue = optionValue.toLowerCase();
                      value = value.toLowerCase();
                    }
                    if( String(optionValue) == String(value)) {
                      module.verbose('Found select item by value', optionValue, value);
                      $selectedItem = $choice;
                      return true;
                    }
                  }
                })
              ;
            }
            return $selectedItem;
          }
        },

        check: {
          maxSelections: function(selectionCount) {
            if(settings.maxSelections) {
              selectionCount = (selectionCount !== undefined)
                ? selectionCount
                : module.get.selectionCount()
              ;
              if(selectionCount >= settings.maxSelections) {
                module.debug('Maximum selection count reached');
                if(settings.useLabels) {
                  $item.addClass(className.filtered);
                  module.add.message(message.maxSelections);
                }
                return true;
              }
              else {
                module.verbose('No longer at maximum selection count');
                module.remove.message();
                module.remove.filteredItem();
                if(module.is.searchSelection()) {
                  module.filterItems();
                }
                return false;
              }
            }
            return true;
          }
        },

        restore: {
          defaults: function(preventChangeTrigger) {
            module.clear(preventChangeTrigger);
            module.restore.defaultText();
            module.restore.defaultValue();
          },
          defaultText: function() {
            var
              defaultText     = module.get.defaultText(),
              placeholderText = module.get.placeholderText
            ;
            if(defaultText === placeholderText) {
              module.debug('Restoring default placeholder text', defaultText);
              module.set.placeholderText(defaultText);
            }
            else {
              module.debug('Restoring default text', defaultText);
              module.set.text(defaultText);
            }
          },
          placeholderText: function() {
            module.set.placeholderText();
          },
          defaultValue: function() {
            var
              defaultValue = module.get.defaultValue()
            ;
            if(defaultValue !== undefined) {
              module.debug('Restoring default value', defaultValue);
              if(defaultValue !== '') {
                module.set.value(defaultValue);
                module.set.selected();
              }
              else {
                module.remove.activeItem();
                module.remove.selectedItem();
              }
            }
          },
          labels: function() {
            if(settings.allowAdditions) {
              if(!settings.useLabels) {
                module.error(error.labels);
                settings.useLabels = true;
              }
              module.debug('Restoring selected values');
              module.create.userLabels();
            }
            module.check.maxSelections();
          },
          selected: function() {
            module.restore.values();
            if(module.is.multiple()) {
              module.debug('Restoring previously selected values and labels');
              module.restore.labels();
            }
            else {
              module.debug('Restoring previously selected values');
            }
          },
          values: function() {
            // prevents callbacks from occurring on initial load
            module.set.initialLoad();
            if(settings.apiSettings && settings.saveRemoteData && module.get.remoteValues()) {
              module.restore.remoteValues();
            }
            else {
              module.set.selected();
            }
            var value = module.get.value();
            if(value && value !== '' && !(Array.isArray(value) && value.length === 0)) {
              $input.removeClass(className.noselection);
            } else {
              $input.addClass(className.noselection);
            }
            module.remove.initialLoad();
          },
          remoteValues: function() {
            var
              values = module.get.remoteValues()
            ;
            module.debug('Recreating selected from session data', values);
            if(values) {
              if( module.is.single() ) {
                $.each(values, function(value, name) {
                  module.set.text(name);
                });
              }
              else {
                $.each(values, function(value, name) {
                  module.add.label(value, name);
                });
              }
            }
          }
        },

        read: {
          remoteData: function(value) {
            var
              name
            ;
            if(window.Storage === undefined) {
              module.error(error.noStorage);
              return;
            }
            name = sessionStorage.getItem(value);
            return (name !== undefined)
              ? name
              : false
            ;
          }
        },

        save: {
          defaults: function() {
            module.save.defaultText();
            module.save.placeholderText();
            module.save.defaultValue();
          },
          defaultValue: function() {
            var
              value = module.get.value()
            ;
            module.verbose('Saving default value as', value);
            $module.data(metadata.defaultValue, value);
          },
          defaultText: function() {
            var
              text = module.get.text()
            ;
            module.verbose('Saving default text as', text);
            $module.data(metadata.defaultText, text);
          },
          placeholderText: function() {
            var
              text
            ;
            if(settings.placeholder !== false && $text.hasClass(className.placeholder)) {
              text = module.get.text();
              module.verbose('Saving placeholder text as', text);
              $module.data(metadata.placeholderText, text);
            }
          },
          remoteData: function(name, value) {
            if(window.Storage === undefined) {
              module.error(error.noStorage);
              return;
            }
            module.verbose('Saving remote data to session storage', value, name);
            sessionStorage.setItem(value, name);
          }
        },

        clear: function(preventChangeTrigger) {
          if(module.is.multiple() && settings.useLabels) {
            module.remove.labels();
          }
          else {
            module.remove.activeItem();
            module.remove.selectedItem();
            module.remove.filteredItem();
          }
          module.set.placeholderText();
          module.clearValue(preventChangeTrigger);
        },

        clearValue: function(preventChangeTrigger) {
          module.set.value('', null, null, preventChangeTrigger);
        },

        scrollPage: function(direction, $selectedItem) {
          var
            $currentItem  = $selectedItem || module.get.selectedItem(),
            $menu         = $currentItem.closest(selector.menu),
            menuHeight    = $menu.outerHeight(),
            currentScroll = $menu.scrollTop(),
            itemHeight    = $item.eq(0).outerHeight(),
            itemsPerPage  = Math.floor(menuHeight / itemHeight),
            maxScroll     = $menu.prop('scrollHeight'),
            newScroll     = (direction == 'up')
              ? currentScroll - (itemHeight * itemsPerPage)
              : currentScroll + (itemHeight * itemsPerPage),
            $selectableItem = $item.not(selector.unselectable),
            isWithinRange,
            $nextSelectedItem,
            elementIndex
          ;
          elementIndex      = (direction == 'up')
            ? $selectableItem.index($currentItem) - itemsPerPage
            : $selectableItem.index($currentItem) + itemsPerPage
          ;
          isWithinRange = (direction == 'up')
            ? (elementIndex >= 0)
            : (elementIndex < $selectableItem.length)
          ;
          $nextSelectedItem = (isWithinRange)
            ? $selectableItem.eq(elementIndex)
            : (direction == 'up')
              ? $selectableItem.first()
              : $selectableItem.last()
          ;
          if($nextSelectedItem.length > 0) {
            module.debug('Scrolling page', direction, $nextSelectedItem);
            $currentItem
              .removeClass(className.selected)
            ;
            $nextSelectedItem
              .addClass(className.selected)
            ;
            if(settings.selectOnKeydown && module.is.single()) {
              module.set.selectedItem($nextSelectedItem);
            }
            $menu
              .scrollTop(newScroll)
            ;
          }
        },

        set: {
          filtered: function() {
            var
              isMultiple       = module.is.multiple(),
              isSearch         = module.is.searchSelection(),
              isSearchMultiple = (isMultiple && isSearch),
              searchValue      = (isSearch)
                ? module.get.query()
                : '',
              hasSearchValue   = (typeof searchValue === 'string' && searchValue.length > 0),
              searchWidth      = module.get.searchWidth(),
              valueIsSet       = searchValue !== ''
            ;
            if(isMultiple && hasSearchValue) {
              module.verbose('Adjusting input width', searchWidth, settings.glyphWidth);
              $search.css('width', searchWidth);
            }
            if(hasSearchValue || (isSearchMultiple && valueIsSet)) {
              module.verbose('Hiding placeholder text');
              $text.addClass(className.filtered);
            }
            else if(!isMultiple || (isSearchMultiple && !valueIsSet)) {
              module.verbose('Showing placeholder text');
              $text.removeClass(className.filtered);
            }
          },
          empty: function() {
            $module.addClass(className.empty);
          },
          loading: function() {
            $module.addClass(className.loading);
          },
          placeholderText: function(text) {
            text = text || module.get.placeholderText();
            module.debug('Setting placeholder text', text);
            module.set.text(text);
            $text.addClass(className.placeholder);
          },
          tabbable: function() {
            if( module.is.searchSelection() ) {
              module.debug('Added tabindex to searchable dropdown');
              $search
                .val('')
                .attr('tabindex', 0)
              ;
              $menu
                .attr('tabindex', -1)
              ;
            }
            else {
              module.debug('Added tabindex to dropdown');
              if( $module.attr('tabindex') === undefined) {
                $module
                  .attr('tabindex', 0)
                ;
                $menu
                  .attr('tabindex', -1)
                ;
              }
            }
          },
          initialLoad: function() {
            module.verbose('Setting initial load');
            initialLoad = true;
          },
          activeItem: function($item) {
            if( settings.allowAdditions && $item.filter(selector.addition).length > 0 ) {
              $item.addClass(className.filtered);
            }
            else {
              $item.addClass(className.active);
            }
          },
          partialSearch: function(text) {
            var
              length = module.get.query().length
            ;
            $search.val( text.substr(0, length));
          },
          scrollPosition: function($item, forceScroll) {
            var
              edgeTolerance = 5,
              $menu,
              hasActive,
              offset,
              itemHeight,
              itemOffset,
              menuOffset,
              menuScroll,
              menuHeight,
              abovePage,
              belowPage
            ;

            $item       = $item || module.get.selectedItem();
            $menu       = $item.closest(selector.menu);
            hasActive   = ($item && $item.length > 0);
            forceScroll = (forceScroll !== undefined)
              ? forceScroll
              : false
            ;
            if(module.get.activeItem().length === 0){
              forceScroll = false;
            }
            if($item && $menu.length > 0 && hasActive) {
              itemOffset = $item.position().top;

              $menu.addClass(className.loading);
              menuScroll = $menu.scrollTop();
              menuOffset = $menu.offset().top;
              itemOffset = $item.offset().top;
              offset     = menuScroll - menuOffset + itemOffset;
              if(!forceScroll) {
                menuHeight = $menu.height();
                belowPage  = menuScroll + menuHeight < (offset + edgeTolerance);
                abovePage  = ((offset - edgeTolerance) < menuScroll);
              }
              module.debug('Scrolling to active item', offset);
              if(forceScroll || abovePage || belowPage) {
                $menu.scrollTop(offset);
              }
              $menu.removeClass(className.loading);
            }
          },
          text: function(text) {
            if(settings.action === 'combo') {
              module.debug('Changing combo button text', text, $combo);
              if(settings.preserveHTML) {
                $combo.html(text);
              }
              else {
                $combo.text(text);
              }
            }
            else if(settings.action === 'activate') {
              if(text !== module.get.placeholderText()) {
                $text.removeClass(className.placeholder);
              }
              module.debug('Changing text', text, $text);
              $text
                .removeClass(className.filtered)
              ;
              if(settings.preserveHTML) {
                $text.html(text);
              }
              else {
                $text.text(text);
              }
            }
          },
          selectedItem: function($item) {
            var
              value      = module.get.choiceValue($item),
              searchText = module.get.choiceText($item, false),
              text       = module.get.choiceText($item, true)
            ;
            module.debug('Setting user selection to item', $item);
            module.remove.activeItem();
            module.set.partialSearch(searchText);
            module.set.activeItem($item);
            module.set.selected(value, $item);
            module.set.text(text);
          },
          selectedLetter: function(letter) {
            var
              $selectedItem         = $item.filter('.' + className.selected),
              alreadySelectedLetter = $selectedItem.length > 0 && module.has.firstLetter($selectedItem, letter),
              $nextValue            = false,
              $nextItem
            ;
            // check next of same letter
            if(alreadySelectedLetter) {
              $nextItem = $selectedItem.nextAll($item).eq(0);
              if( module.has.firstLetter($nextItem, letter) ) {
                $nextValue  = $nextItem;
              }
            }
            // check all values
            if(!$nextValue) {
              $item
                .each(function(){
                  if(module.has.firstLetter($(this), letter)) {
                    $nextValue = $(this);
                    return false;
                  }
                })
              ;
            }
            // set next value
            if($nextValue) {
              module.verbose('Scrolling to next value with letter', letter);
              module.set.scrollPosition($nextValue);
              $selectedItem.removeClass(className.selected);
              $nextValue.addClass(className.selected);
              if(settings.selectOnKeydown && module.is.single()) {
                module.set.selectedItem($nextValue);
              }
            }
          },
          direction: function($menu) {
            if(settings.direction == 'auto') {
              // reset position, remove upward if it's base menu
              if (!$menu) {
                module.remove.upward();
              } else if (module.is.upward($menu)) {
                //we need make sure when make assertion openDownward for $menu, $menu does not have upward class
                module.remove.upward($menu);
              }

              if(module.can.openDownward($menu)) {
                module.remove.upward($menu);
              }
              else {
                module.set.upward($menu);
              }
              if(!module.is.leftward($menu) && !module.can.openRightward($menu)) {
                module.set.leftward($menu);
              }
            }
            else if(settings.direction == 'upward') {
              module.set.upward($menu);
            }
          },
          upward: function($currentMenu) {
            var $element = $currentMenu || $module;
            $element.addClass(className.upward);
          },
          leftward: function($currentMenu) {
            var $element = $currentMenu || $menu;
            $element.addClass(className.leftward);
          },
          value: function(value, text, $selected, preventChangeTrigger) {
            if(value !== undefined && value !== '' && !(Array.isArray(value) && value.length === 0)) {
              $input.removeClass(className.noselection);
            } else {
              $input.addClass(className.noselection);
            }
            var
              escapedValue = module.escape.value(value),
              hasInput     = ($input.length > 0),
              currentValue = module.get.values(),
              stringValue  = (value !== undefined)
                ? String(value)
                : value,
              newValue
            ;
            if(hasInput) {
              if(!settings.allowReselection && stringValue == currentValue) {
                module.verbose('Skipping value update already same value', value, currentValue);
                if(!module.is.initialLoad()) {
                  return;
                }
              }

              if( module.is.single() && module.has.selectInput() && module.can.extendSelect() ) {
                module.debug('Adding user option', value);
                module.add.optionValue(value);
              }
              module.debug('Updating input value', escapedValue, currentValue);
              internalChange = true;
              $input
                .val(escapedValue)
              ;
              if(settings.fireOnInit === false && module.is.initialLoad()) {
                module.debug('Input native change event ignored on initial load');
              }
              else if(preventChangeTrigger !== true) {
                module.trigger.change();
              }
              internalChange = false;
            }
            else {
              module.verbose('Storing value in metadata', escapedValue, $input);
              if(escapedValue !== currentValue) {
                $module.data(metadata.value, stringValue);
              }
            }
            if(settings.fireOnInit === false && module.is.initialLoad()) {
              module.verbose('No callback on initial load', settings.onChange);
            }
            else if(preventChangeTrigger !== true) {
              settings.onChange.call(element, value, text, $selected);
            }
          },
          active: function() {
            $module
              .addClass(className.active)
            ;
          },
          multiple: function() {
            $module.addClass(className.multiple);
          },
          visible: function() {
            $module.addClass(className.visible);
          },
          exactly: function(value, $selectedItem) {
            module.debug('Setting selected to exact values');
            module.clear();
            module.set.selected(value, $selectedItem);
          },
          selected: function(value, $selectedItem) {
            var
              isMultiple = module.is.multiple()
            ;
            $selectedItem = (settings.allowAdditions)
              ? $selectedItem || module.get.itemWithAdditions(value)
              : $selectedItem || module.get.item(value)
            ;
            if(!$selectedItem) {
              return;
            }
            module.debug('Setting selected menu item to', $selectedItem);
            if(module.is.multiple()) {
              module.remove.searchWidth();
            }
            if(module.is.single()) {
              module.remove.activeItem();
              module.remove.selectedItem();
            }
            else if(settings.useLabels) {
              module.remove.selectedItem();
            }
            // select each item
            $selectedItem
              .each(function() {
                var
                  $selected      = $(this),
                  selectedText   = module.get.choiceText($selected),
                  selectedValue  = module.get.choiceValue($selected, selectedText),

                  isFiltered     = $selected.hasClass(className.filtered),
                  isActive       = $selected.hasClass(className.active),
                  isUserValue    = $selected.hasClass(className.addition),
                  shouldAnimate  = (isMultiple && $selectedItem.length == 1)
                ;
                if(isMultiple) {
                  if(!isActive || isUserValue) {
                    if(settings.apiSettings && settings.saveRemoteData) {
                      module.save.remoteData(selectedText, selectedValue);
                    }
                    if(settings.useLabels) {
                      module.add.label(selectedValue, selectedText, shouldAnimate);
                      module.add.value(selectedValue, selectedText, $selected);
                      module.set.activeItem($selected);
                      module.filterActive();
                      module.select.nextAvailable($selectedItem);
                    }
                    else {
                      module.add.value(selectedValue, selectedText, $selected);
                      module.set.text(module.add.variables(message.count));
                      module.set.activeItem($selected);
                    }
                  }
                  else if(!isFiltered && (settings.useLabels || selectActionActive)) {
                    module.debug('Selected active value, removing label');
                    module.remove.selected(selectedValue);
                  }
                }
                else {
                  if(settings.apiSettings && settings.saveRemoteData) {
                    module.save.remoteData(selectedText, selectedValue);
                  }
                  module.set.text(selectedText);
                  module.set.value(selectedValue, selectedText, $selected);
                  $selected
                    .addClass(className.active)
                    .addClass(className.selected)
                  ;
                }
              })
            ;
            module.remove.searchTerm();
          }
        },

        add: {
          label: function(value, text, shouldAnimate) {
            var
              $next  = module.is.searchSelection()
                ? $search
                : $text,
              escapedValue = module.escape.value(value),
              $label
            ;
            if(settings.ignoreCase) {
              escapedValue = escapedValue.toLowerCase();
            }
            $label =  $('<a />')
              .addClass(className.label)
              .attr('data-' + metadata.value, escapedValue)
              .html(templates.label(escapedValue, text, settings.preserveHTML, settings.className))
            ;
            $label = settings.onLabelCreate.call($label, escapedValue, text);

            if(module.has.label(value)) {
              module.debug('User selection already exists, skipping', escapedValue);
              return;
            }
            if(settings.label.variation) {
              $label.addClass(settings.label.variation);
            }
            if(shouldAnimate === true) {
              module.debug('Animating in label', $label);
              $label
                .addClass(className.hidden)
                .insertBefore($next)
                .transition({
                    animation  : settings.label.transition,
                    debug      : settings.debug,
                    verbose    : settings.verbose,
                    duration   : settings.label.duration
                })
              ;
            }
            else {
              module.debug('Adding selection label', $label);
              $label
                .insertBefore($next)
              ;
            }
          },
          message: function(message) {
            var
              $message = $menu.children(selector.message),
              html     = settings.templates.message(module.add.variables(message))
            ;
            if($message.length > 0) {
              $message
                .html(html)
              ;
            }
            else {
              $message = $('<div/>')
                .html(html)
                .addClass(className.message)
                .appendTo($menu)
              ;
            }
          },
          optionValue: function(value) {
            var
              escapedValue = module.escape.value(value),
              $option      = $input.find('option[value="' + module.escape.string(escapedValue) + '"]'),
              hasOption    = ($option.length > 0)
            ;
            if(hasOption) {
              return;
            }
            // temporarily disconnect observer
            module.disconnect.selectObserver();
            if( module.is.single() ) {
              module.verbose('Removing previous user addition');
              $input.find('option.' + className.addition).remove();
            }
            $('<option/>')
              .prop('value', escapedValue)
              .addClass(className.addition)
              .html(value)
              .appendTo($input)
            ;
            module.verbose('Adding user addition as an <option>', value);
            module.observe.select();
          },
          userSuggestion: function(value) {
            var
              $addition         = $menu.children(selector.addition),
              $existingItem     = module.get.item(value),
              alreadyHasValue   = $existingItem && $existingItem.not(selector.addition).length,
              hasUserSuggestion = $addition.length > 0,
              html
            ;
            if(settings.useLabels && module.has.maxSelections()) {
              return;
            }
            if(value === '' || alreadyHasValue) {
              $addition.remove();
              return;
            }
            if(hasUserSuggestion) {
              $addition
                .data(metadata.value, value)
                .data(metadata.text, value)
                .attr('data-' + metadata.value, value)
                .attr('data-' + metadata.text, value)
                .removeClass(className.filtered)
              ;
              if(!settings.hideAdditions) {
                html = settings.templates.addition( module.add.variables(message.addResult, value) );
                $addition
                  .html(html)
                ;
              }
              module.verbose('Replacing user suggestion with new value', $addition);
            }
            else {
              $addition = module.create.userChoice(value);
              $addition
                .prependTo($menu)
              ;
              module.verbose('Adding item choice to menu corresponding with user choice addition', $addition);
            }
            if(!settings.hideAdditions || module.is.allFiltered()) {
              $addition
                .addClass(className.selected)
                .siblings()
                .removeClass(className.selected)
              ;
            }
            module.refreshItems();
          },
          variables: function(message, term) {
            var
              hasCount    = (message.search('{count}') !== -1),
              hasMaxCount = (message.search('{maxCount}') !== -1),
              hasTerm     = (message.search('{term}') !== -1),
              count,
              query
            ;
            module.verbose('Adding templated variables to message', message);
            if(hasCount) {
              count  = module.get.selectionCount();
              message = message.replace('{count}', count);
            }
            if(hasMaxCount) {
              count  = module.get.selectionCount();
              message = message.replace('{maxCount}', settings.maxSelections);
            }
            if(hasTerm) {
              query   = term || module.get.query();
              message = message.replace('{term}', query);
            }
            return message;
          },
          value: function(addedValue, addedText, $selectedItem) {
            var
              currentValue = module.get.values(),
              newValue
            ;
            if(module.has.value(addedValue)) {
              module.debug('Value already selected');
              return;
            }
            if(addedValue === '') {
              module.debug('Cannot select blank values from multiselect');
              return;
            }
            // extend current array
            if(Array.isArray(currentValue)) {
              newValue = currentValue.concat([addedValue]);
              newValue = module.get.uniqueArray(newValue);
            }
            else {
              newValue = [addedValue];
            }
            // add values
            if( module.has.selectInput() ) {
              if(module.can.extendSelect()) {
                module.debug('Adding value to select', addedValue, newValue, $input);
                module.add.optionValue(addedValue);
              }
            }
            else {
              newValue = newValue.join(settings.delimiter);
              module.debug('Setting hidden input to delimited value', newValue, $input);
            }

            if(settings.fireOnInit === false && module.is.initialLoad()) {
              module.verbose('Skipping onadd callback on initial load', settings.onAdd);
            }
            else {
              settings.onAdd.call(element, addedValue, addedText, $selectedItem);
            }
            module.set.value(newValue, addedText, $selectedItem);
            module.check.maxSelections();
          },
        },

        remove: {
          active: function() {
            $module.removeClass(className.active);
          },
          activeLabel: function() {
            $module.find(selector.label).removeClass(className.active);
          },
          empty: function() {
            $module.removeClass(className.empty);
          },
          loading: function() {
            $module.removeClass(className.loading);
          },
          initialLoad: function() {
            initialLoad = false;
          },
          upward: function($currentMenu) {
            var $element = $currentMenu || $module;
            $element.removeClass(className.upward);
          },
          leftward: function($currentMenu) {
            var $element = $currentMenu || $menu;
            $element.removeClass(className.leftward);
          },
          visible: function() {
            $module.removeClass(className.visible);
          },
          activeItem: function() {
            $item.removeClass(className.active);
          },
          filteredItem: function() {
            if(settings.useLabels && module.has.maxSelections() ) {
              return;
            }
            if(settings.useLabels && module.is.multiple()) {
              $item.not('.' + className.active).removeClass(className.filtered);
            }
            else {
              $item.removeClass(className.filtered);
            }
            if(settings.hideDividers) {
              $divider.removeClass(className.hidden);
            }
            module.remove.empty();
          },
          optionValue: function(value) {
            var
              escapedValue = module.escape.value(value),
              $option      = $input.find('option[value="' + module.escape.string(escapedValue) + '"]'),
              hasOption    = ($option.length > 0)
            ;
            if(!hasOption || !$option.hasClass(className.addition)) {
              return;
            }
            // temporarily disconnect observer
            if(selectObserver) {
              selectObserver.disconnect();
              module.verbose('Temporarily disconnecting mutation observer');
            }
            $option.remove();
            module.verbose('Removing user addition as an <option>', escapedValue);
            if(selectObserver) {
              selectObserver.observe($input[0], {
                childList : true,
                subtree   : true
              });
            }
          },
          message: function() {
            $menu.children(selector.message).remove();
          },
          searchWidth: function() {
            $search.css('width', '');
          },
          searchTerm: function() {
            module.verbose('Cleared search term');
            $search.val('');
            module.set.filtered();
          },
          userAddition: function() {
            $item.filter(selector.addition).remove();
          },
          selected: function(value, $selectedItem) {
            $selectedItem = (settings.allowAdditions)
              ? $selectedItem || module.get.itemWithAdditions(value)
              : $selectedItem || module.get.item(value)
            ;

            if(!$selectedItem) {
              return false;
            }

            $selectedItem
              .each(function() {
                var
                  $selected     = $(this),
                  selectedText  = module.get.choiceText($selected),
                  selectedValue = module.get.choiceValue($selected, selectedText)
                ;
                if(module.is.multiple()) {
                  if(settings.useLabels) {
                    module.remove.value(selectedValue, selectedText, $selected);
                    module.remove.label(selectedValue);
                  }
                  else {
                    module.remove.value(selectedValue, selectedText, $selected);
                    if(module.get.selectionCount() === 0) {
                      module.set.placeholderText();
                    }
                    else {
                      module.set.text(module.add.variables(message.count));
                    }
                  }
                }
                else {
                  module.remove.value(selectedValue, selectedText, $selected);
                }
                $selected
                  .removeClass(className.filtered)
                  .removeClass(className.active)
                ;
                if(settings.useLabels) {
                  $selected.removeClass(className.selected);
                }
              })
            ;
          },
          selectedItem: function() {
            $item.removeClass(className.selected);
          },
          value: function(removedValue, removedText, $removedItem) {
            var
              values = module.get.values(),
              newValue
            ;
            if( module.has.selectInput() ) {
              module.verbose('Input is <select> removing selected option', removedValue);
              newValue = module.remove.arrayValue(removedValue, values);
              module.remove.optionValue(removedValue);
            }
            else {
              module.verbose('Removing from delimited values', removedValue);
              newValue = module.remove.arrayValue(removedValue, values);
              newValue = newValue.join(settings.delimiter);
            }
            if(settings.fireOnInit === false && module.is.initialLoad()) {
              module.verbose('No callback on initial load', settings.onRemove);
            }
            else {
              settings.onRemove.call(element, removedValue, removedText, $removedItem);
            }
            module.set.value(newValue, removedText, $removedItem);
            module.check.maxSelections();
          },
          arrayValue: function(removedValue, values) {
            if( !Array.isArray(values) ) {
              values = [values];
            }
            values = $.grep(values, function(value){
              return (removedValue != value);
            });
            module.verbose('Removed value from delimited string', removedValue, values);
            return values;
          },
          label: function(value, shouldAnimate) {
            var
              $labels       = $module.find(selector.label),
              $removedLabel = $labels.filter('[data-' + metadata.value + '="' + module.escape.string(settings.ignoreCase ? value.toLowerCase() : value) +'"]')
            ;
            module.verbose('Removing label', $removedLabel);
            $removedLabel.remove();
          },
          activeLabels: function($activeLabels) {
            $activeLabels = $activeLabels || $module.find(selector.label).filter('.' + className.active);
            module.verbose('Removing active label selections', $activeLabels);
            module.remove.labels($activeLabels);
          },
          labels: function($labels) {
            $labels = $labels || $module.find(selector.label);
            module.verbose('Removing labels', $labels);
            $labels
              .each(function(){
                var
                  $label      = $(this),
                  value       = $label.data(metadata.value),
                  stringValue = (value !== undefined)
                    ? String(value)
                    : value,
                  isUserValue = module.is.userValue(stringValue)
                ;
                if(settings.onLabelRemove.call($label, value) === false) {
                  module.debug('Label remove callback cancelled removal');
                  return;
                }
                module.remove.message();
                if(isUserValue) {
                  module.remove.value(stringValue);
                  module.remove.label(stringValue);
                }
                else {
                  // selected will also remove label
                  module.remove.selected(stringValue);
                }
              })
            ;
          },
          tabbable: function() {
            if( module.is.searchSelection() ) {
              module.debug('Searchable dropdown initialized');
              $search
                .removeAttr('tabindex')
              ;
              $menu
                .removeAttr('tabindex')
              ;
            }
            else {
              module.debug('Simple selection dropdown initialized');
              $module
                .removeAttr('tabindex')
              ;
              $menu
                .removeAttr('tabindex')
              ;
            }
          },
          diacritics: function(text) {
            return settings.ignoreDiacritics ?  text.normalize('NFD').replace(/[\u0300-\u036f]/g, '') : text;
          }
        },

        has: {
          menuSearch: function() {
            return (module.has.search() && $search.closest($menu).length > 0);
          },
          clearItem: function() {
            return ($clear.length > 0);
          },
          search: function() {
            return ($search.length > 0);
          },
          sizer: function() {
            return ($sizer.length > 0);
          },
          selectInput: function() {
            return ( $input.is('select') );
          },
          minCharacters: function(searchTerm) {
            if(settings.minCharacters && !iconClicked) {
              searchTerm = (searchTerm !== undefined)
                ? String(searchTerm)
                : String(module.get.query())
              ;
              return (searchTerm.length >= settings.minCharacters);
            }
            iconClicked=false;
            return true;
          },
          firstLetter: function($item, letter) {
            var
              text,
              firstLetter
            ;
            if(!$item || $item.length === 0 || typeof letter !== 'string') {
              return false;
            }
            text        = module.get.choiceText($item, false);
            letter      = letter.toLowerCase();
            firstLetter = String(text).charAt(0).toLowerCase();
            return (letter == firstLetter);
          },
          input: function() {
            return ($input.length > 0);
          },
          items: function() {
            return ($item.length > 0);
          },
          menu: function() {
            return ($menu.length > 0);
          },
          message: function() {
            return ($menu.children(selector.message).length !== 0);
          },
          label: function(value) {
            var
              escapedValue = module.escape.value(value),
              $labels      = $module.find(selector.label)
            ;
            if(settings.ignoreCase) {
              escapedValue = escapedValue.toLowerCase();
            }
            return ($labels.filter('[data-' + metadata.value + '="' + module.escape.string(escapedValue) +'"]').length > 0);
          },
          maxSelections: function() {
            return (settings.maxSelections && module.get.selectionCount() >= settings.maxSelections);
          },
          allResultsFiltered: function() {
            var
              $normalResults = $item.not(selector.addition)
            ;
            return ($normalResults.filter(selector.unselectable).length === $normalResults.length);
          },
          userSuggestion: function() {
            return ($menu.children(selector.addition).length > 0);
          },
          query: function() {
            return (module.get.query() !== '');
          },
          value: function(value) {
            return (settings.ignoreCase)
              ? module.has.valueIgnoringCase(value)
              : module.has.valueMatchingCase(value)
            ;
          },
          valueMatchingCase: function(value) {
            var
              values   = module.get.values(),
              hasValue = Array.isArray(values)
               ? values && ($.inArray(value, values) !== -1)
               : (values == value)
            ;
            return (hasValue)
              ? true
              : false
            ;
          },
          valueIgnoringCase: function(value) {
            var
              values   = module.get.values(),
              hasValue = false
            ;
            if(!Array.isArray(values)) {
              values = [values];
            }
            $.each(values, function(index, existingValue) {
              if(String(value).toLowerCase() == String(existingValue).toLowerCase()) {
                hasValue = true;
                return false;
              }
            });
            return hasValue;
          }
        },

        is: {
          active: function() {
            return $module.hasClass(className.active);
          },
          animatingInward: function() {
            return $menu.transition('is inward');
          },
          animatingOutward: function() {
            return $menu.transition('is outward');
          },
          bubbledLabelClick: function(event) {
            return $(event.target).is('select, input') && $module.closest('label').length > 0;
          },
          bubbledIconClick: function(event) {
            return $(event.target).closest($icon).length > 0;
          },
          alreadySetup: function() {
            return ($module.is('select') && $module.parent(selector.dropdown).data(moduleNamespace) !== undefined && $module.prev().length === 0);
          },
          animating: function($subMenu) {
            return ($subMenu)
              ? $subMenu.transition && $subMenu.transition('is animating')
              : $menu.transition    && $menu.transition('is animating')
            ;
          },
          leftward: function($subMenu) {
            var $selectedMenu = $subMenu || $menu;
            return $selectedMenu.hasClass(className.leftward);
          },
          clearable: function() {
            return ($module.hasClass(className.clearable) || settings.clearable);
          },
          disabled: function() {
            return $module.hasClass(className.disabled);
          },
          focused: function() {
            return (document.activeElement === $module[0]);
          },
          focusedOnSearch: function() {
            return (document.activeElement === $search[0]);
          },
          allFiltered: function() {
            return( (module.is.multiple() || module.has.search()) && !(settings.hideAdditions == false && module.has.userSuggestion()) && !module.has.message() && module.has.allResultsFiltered() );
          },
          hidden: function($subMenu) {
            return !module.is.visible($subMenu);
          },
          initialLoad: function() {
            return initialLoad;
          },
          inObject: function(needle, object) {
            var
              found = false
            ;
            $.each(object, function(index, property) {
              if(property == needle) {
                found = true;
                return true;
              }
            });
            return found;
          },
          multiple: function() {
            return $module.hasClass(className.multiple);
          },
          remote: function() {
            return settings.apiSettings && module.can.useAPI();
          },
          single: function() {
            return !module.is.multiple();
          },
          selectMutation: function(mutations) {
            var
              selectChanged = false
            ;
            $.each(mutations, function(index, mutation) {
              if($(mutation.target).is('select') || $(mutation.addedNodes).is('select')) {
                selectChanged = true;
                return false;
              }
            });
            return selectChanged;
          },
          search: function() {
            return $module.hasClass(className.search);
          },
          searchSelection: function() {
            return ( module.has.search() && $search.parent(selector.dropdown).length === 1 );
          },
          selection: function() {
            return $module.hasClass(className.selection);
          },
          userValue: function(value) {
            return ($.inArray(value, module.get.userValues()) !== -1);
          },
          upward: function($menu) {
            var $element = $menu || $module;
            return $element.hasClass(className.upward);
          },
          visible: function($subMenu) {
            return ($subMenu)
              ? $subMenu.hasClass(className.visible)
              : $menu.hasClass(className.visible)
            ;
          },
          verticallyScrollableContext: function() {
            var
              overflowY = ($context.get(0) !== window)
                ? $context.css('overflow-y')
                : false
            ;
            return (overflowY == 'auto' || overflowY == 'scroll');
          },
          horizontallyScrollableContext: function() {
            var
              overflowX = ($context.get(0) !== window)
                ? $context.css('overflow-X')
                : false
            ;
            return (overflowX == 'auto' || overflowX == 'scroll');
          }
        },

        can: {
          activate: function($item) {
            if(settings.useLabels) {
              return true;
            }
            if(!module.has.maxSelections()) {
              return true;
            }
            if(module.has.maxSelections() && $item.hasClass(className.active)) {
              return true;
            }
            return false;
          },
          openDownward: function($subMenu) {
            var
              $currentMenu    = $subMenu || $menu,
              canOpenDownward = true,
              onScreen        = {},
              calculations
            ;
            $currentMenu
              .addClass(className.loading)
            ;
            calculations = {
              context: {
                offset    : ($context.get(0) === window)
                  ? { top: 0, left: 0}
                  : $context.offset(),
                scrollTop : $context.scrollTop(),
                height    : $context.outerHeight()
              },
              menu : {
                offset: $currentMenu.offset(),
                height: $currentMenu.outerHeight()
              }
            };
            if(module.is.verticallyScrollableContext()) {
              calculations.menu.offset.top += calculations.context.scrollTop;
            }
            onScreen = {
              above : (calculations.context.scrollTop) <= calculations.menu.offset.top - calculations.context.offset.top - calculations.menu.height,
              below : (calculations.context.scrollTop + calculations.context.height) >= calculations.menu.offset.top - calculations.context.offset.top + calculations.menu.height
            };
            if(onScreen.below) {
              module.verbose('Dropdown can fit in context downward', onScreen);
              canOpenDownward = true;
            }
            else if(!onScreen.below && !onScreen.above) {
              module.verbose('Dropdown cannot fit in either direction, favoring downward', onScreen);
              canOpenDownward = true;
            }
            else {
              module.verbose('Dropdown cannot fit below, opening upward', onScreen);
              canOpenDownward = false;
            }
            $currentMenu.removeClass(className.loading);
            return canOpenDownward;
          },
          openRightward: function($subMenu) {
            var
              $currentMenu     = $subMenu || $menu,
              canOpenRightward = true,
              isOffscreenRight = false,
              calculations
            ;
            $currentMenu
              .addClass(className.loading)
            ;
            calculations = {
              context: {
                offset     : ($context.get(0) === window)
                  ? { top: 0, left: 0}
                  : $context.offset(),
                scrollLeft : $context.scrollLeft(),
                width      : $context.outerWidth()
              },
              menu: {
                offset : $currentMenu.offset(),
                width  : $currentMenu.outerWidth()
              }
            };
            if(module.is.horizontallyScrollableContext()) {
              calculations.menu.offset.left += calculations.context.scrollLeft;
            }
            isOffscreenRight = (calculations.menu.offset.left - calculations.context.offset.left + calculations.menu.width >= calculations.context.scrollLeft + calculations.context.width);
            if(isOffscreenRight) {
              module.verbose('Dropdown cannot fit in context rightward', isOffscreenRight);
              canOpenRightward = false;
            }
            $currentMenu.removeClass(className.loading);
            return canOpenRightward;
          },
          click: function() {
            return (hasTouch || settings.on == 'click');
          },
          extendSelect: function() {
            return settings.allowAdditions || settings.apiSettings;
          },
          show: function() {
            return !module.is.disabled() && (module.has.items() || module.has.message());
          },
          useAPI: function() {
            return $.fn.api !== undefined;
          }
        },

        animate: {
          show: function(callback, $subMenu) {
            var
              $currentMenu = $subMenu || $menu,
              start = ($subMenu)
                ? function() {}
                : function() {
                  module.hideSubMenus();
                  module.hideOthers();
                  module.set.active();
                },
              transition
            ;
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            module.verbose('Doing menu show animation', $currentMenu);
            module.set.direction($subMenu);
            transition = module.get.transition($subMenu);
            if( module.is.selection() ) {
              module.set.scrollPosition(module.get.selectedItem(), true);
            }
            if( module.is.hidden($currentMenu) || module.is.animating($currentMenu) ) {
              if(transition == 'none') {
                start();
                $currentMenu.transition('show');
                callback.call(element);
              }
              else if($.fn.transition !== undefined && $module.transition('is supported')) {
                $currentMenu
                  .transition({
                    animation  : transition + ' in',
                    debug      : settings.debug,
                    verbose    : settings.verbose,
                    duration   : settings.duration,
                    queue      : true,
                    onStart    : start,
                    onComplete : function() {
                      callback.call(element);
                    }
                  })
                ;
              }
              else {
                module.error(error.noTransition, transition);
              }
            }
          },
          hide: function(callback, $subMenu) {
            var
              $currentMenu = $subMenu || $menu,
              start = ($subMenu)
                ? function() {}
                : function() {
                  if( module.can.click() ) {
                    module.unbind.intent();
                  }
                  module.remove.active();
                },
              transition = module.get.transition($subMenu)
            ;
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if( module.is.visible($currentMenu) || module.is.animating($currentMenu) ) {
              module.verbose('Doing menu hide animation', $currentMenu);

              if(transition == 'none') {
                start();
                $currentMenu.transition('hide');
                callback.call(element);
              }
              else if($.fn.transition !== undefined && $module.transition('is supported')) {
                $currentMenu
                  .transition({
                    animation  : transition + ' out',
                    duration   : settings.duration,
                    debug      : settings.debug,
                    verbose    : settings.verbose,
                    queue      : false,
                    onStart    : start,
                    onComplete : function() {
                      callback.call(element);
                    }
                  })
                ;
              }
              else {
                module.error(error.transition);
              }
            }
          }
        },

        hideAndClear: function() {
          module.remove.searchTerm();
          if( module.has.maxSelections() ) {
            return;
          }
          if(module.has.search()) {
            module.hide(function() {
              module.remove.filteredItem();
            });
          }
          else {
            module.hide();
          }
        },

        delay: {
          show: function() {
            module.verbose('Delaying show event to ensure user intent');
            clearTimeout(module.timer);
            module.timer = setTimeout(module.show, settings.delay.show);
          },
          hide: function() {
            module.verbose('Delaying hide event to ensure user intent');
            clearTimeout(module.timer);
            module.timer = setTimeout(module.hide, settings.delay.hide);
          }
        },

        escape: {
          value: function(value) {
            var
              multipleValues = Array.isArray(value),
              stringValue    = (typeof value === 'string'),
              isUnparsable   = (!stringValue && !multipleValues),
              hasQuotes      = (stringValue && value.search(regExp.quote) !== -1),
              values         = []
            ;
            if(isUnparsable || !hasQuotes) {
              return value;
            }
            module.debug('Encoding quote values for use in select', value);
            if(multipleValues) {
              $.each(value, function(index, value){
                values.push(value.replace(regExp.quote, '&quot;'));
              });
              return values;
            }
            return value.replace(regExp.quote, '&quot;');
          },
          string: function(text) {
            text =  String(text);
            return text.replace(regExp.escape, '\\$&');
          },
          htmlEntities: function(string) {
              var
                  badChars     = /[&<>"'`]/g,
                  shouldEscape = /[&<>"'`]/,
                  escape       = {
                      "&": "&amp;",
                      "<": "&lt;",
                      ">": "&gt;",
                      '"': "&quot;",
                      "'": "&#x27;",
                      "`": "&#x60;"
                  },
                  escapedChar  = function(chr) {
                      return escape[chr];
                  }
              ;
              if(shouldEscape.test(string)) {
                  return string.replace(badChars, escapedChar);
              }
              return string;
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;
  return (returnedValue !== undefined)
    ? returnedValue
    : $allModules
  ;
};

$.fn.dropdown.settings = {

  silent                 : false,
  debug                  : false,
  verbose                : false,
  performance            : true,

  on                     : 'click',    // what event should show menu action on item selection
  action                 : 'activate', // action on item selection (nothing, activate, select, combo, hide, function(){})

  values                 : false,      // specify values to use for dropdown

  clearable              : false,      // whether the value of the dropdown can be cleared

  apiSettings            : false,
  selectOnKeydown        : true,       // Whether selection should occur automatically when keyboard shortcuts used
  minCharacters          : 0,          // Minimum characters required to trigger API call

  filterRemoteData       : false,      // Whether API results should be filtered after being returned for query term
  saveRemoteData         : true,       // Whether remote name/value pairs should be stored in sessionStorage to allow remote data to be restored on page refresh

  throttle               : 200,        // How long to wait after last user input to search remotely

  context                : window,     // Context to use when determining if on screen
  direction              : 'auto',     // Whether dropdown should always open in one direction
  keepOnScreen           : true,       // Whether dropdown should check whether it is on screen before showing

  match                  : 'both',     // what to match against with search selection (both, text, or label)
  fullTextSearch         : false,      // search anywhere in value (set to 'exact' to require exact matches)
  ignoreDiacritics       : false,      // match results also if they contain diacritics of the same base character (for example searching for "a" will also match "á" or "â" or "à", etc...)
  hideDividers           : false,      // Whether to hide any divider elements (specified in selector.divider) that are sibling to any items when searched (set to true will hide all dividers, set to 'empty' will hide them when they are not followed by a visible item)

  placeholder            : 'auto',     // whether to convert blank <select> values to placeholder text
  preserveHTML           : true,       // preserve html when selecting value
  sortSelect             : false,      // sort selection on init

  forceSelection         : true,       // force a choice on blur with search selection

  allowAdditions         : false,      // whether multiple select should allow user added values
  ignoreCase             : false,      // whether to consider case sensitivity when creating labels
  ignoreSearchCase       : true,       // whether to consider case sensitivity when filtering items
  hideAdditions          : true,       // whether or not to hide special message prompting a user they can enter a value

  maxSelections          : false,      // When set to a number limits the number of selections to this count
  useLabels              : true,       // whether multiple select should filter currently active selections from choices
  delimiter              : ',',        // when multiselect uses normal <input> the values will be delimited with this character

  showOnFocus            : true,       // show menu on focus
  allowReselection       : false,      // whether current value should trigger callbacks when reselected
  allowTab               : true,       // add tabindex to element
  allowCategorySelection : false,      // allow elements with sub-menus to be selected

  fireOnInit             : false,      // Whether callbacks should fire when initializing dropdown values

  transition             : 'auto',     // auto transition will slide down or up based on direction
  duration               : 200,        // duration of transition

  glyphWidth             : 1.037,      // widest glyph width in em (W is 1.037 em) used to calculate multiselect input width

  headerDivider          : true,       // whether option headers should have an additional divider line underneath when converted from <select> <optgroup>

  // label settings on multi-select
  label: {
    transition : 'scale',
    duration   : 200,
    variation  : false
  },

  // delay before event
  delay : {
    hide   : 300,
    show   : 200,
    search : 20,
    touch  : 50
  },

  /* Callbacks */
  onChange      : function(value, text, $selected){},
  onAdd         : function(value, text, $selected){},
  onRemove      : function(value, text, $selected){},

  onLabelSelect : function($selectedLabels){},
  onLabelCreate : function(value, text) { return $(this); },
  onLabelRemove : function(value) { return true; },
  onNoResults   : function(searchTerm) { return true; },
  onShow        : function(){},
  onHide        : function(){},

  /* Component */
  name           : 'Dropdown',
  namespace      : 'dropdown',

  message: {
    addResult     : 'Add <b>{term}</b>',
    count         : '{count} selected',
    maxSelections : 'Max {maxCount} selections',
    noResults     : 'No results found.',
    serverError   : 'There was an error contacting the server'
  },

  error : {
    action          : 'You called a dropdown action that was not defined',
    alreadySetup    : 'Once a select has been initialized behaviors must be called on the created ui dropdown',
    labels          : 'Allowing user additions currently requires the use of labels.',
    missingMultiple : '<select> requires multiple property to be set to correctly preserve multiple values',
    method          : 'The method you called is not defined.',
    noAPI           : 'The API module is required to load resources remotely',
    noStorage       : 'Saving remote data requires session storage',
    noTransition    : 'This module requires ui transitions <https://github.com/Semantic-Org/UI-Transition>',
    noNormalize     : '"ignoreDiacritics" setting will be ignored. Browser does not support String().normalize(). You may consider including <https://cdn.jsdelivr.net/npm/unorm@1.4.1/lib/unorm.min.js> as a polyfill.'
  },

  regExp : {
    escape   : /[-[\]{}()*+?.,\\^$|#\s:=@]/g,
    quote    : /"/g
  },

  metadata : {
    defaultText     : 'defaultText',
    defaultValue    : 'defaultValue',
    placeholderText : 'placeholder',
    text            : 'text',
    value           : 'value'
  },

  // property names for remote query
  fields: {
    remoteValues : 'results',  // grouping for api results
    values       : 'values',   // grouping for all dropdown values
    disabled     : 'disabled', // whether value should be disabled
    name         : 'name',     // displayed dropdown text
    value        : 'value',    // actual dropdown value
    text         : 'text',     // displayed text when selected
    type         : 'type',     // type of dropdown element
    image        : 'image',    // optional image path
    imageClass   : 'imageClass', // optional individual class for image
    icon         : 'icon',     // optional icon name
    iconClass    : 'iconClass', // optional individual class for icon (for example to use flag instead)
    class        : 'class',    // optional individual class for item/header
    divider      : 'divider'   // optional divider append for group headers
  },

  keys : {
    backspace  : 8,
    delimiter  : 188, // comma
    deleteKey  : 46,
    enter      : 13,
    escape     : 27,
    pageUp     : 33,
    pageDown   : 34,
    leftArrow  : 37,
    upArrow    : 38,
    rightArrow : 39,
    downArrow  : 40
  },

  selector : {
    addition     : '.addition',
    divider      : '.divider, .header',
    dropdown     : '.ui.dropdown',
    hidden       : '.hidden',
    icon         : '> .dropdown.icon',
    input        : '> input[type="hidden"], > select',
    item         : '.item',
    label        : '> .label',
    remove       : '> .label > .delete.icon',
    siblingLabel : '.label',
    menu         : '.menu',
    message      : '.message',
    menuIcon     : '.dropdown.icon',
    search       : 'input.search, .menu > .search > input, .menu input.search',
    sizer        : '> input.sizer',
    text         : '> .text:not(.icon)',
    unselectable : '.disabled, .filtered',
    clearIcon    : '> .remove.icon'
  },

  className : {
    active      : 'active',
    addition    : 'addition',
    animating   : 'animating',
    disabled    : 'disabled',
    empty       : 'empty',
    dropdown    : 'ui dropdown',
    filtered    : 'filtered',
    hidden      : 'hidden transition',
    icon        : 'icon',
    image       : 'image',
    item        : 'item',
    label       : 'ui label',
    loading     : 'loading',
    menu        : 'menu',
    message     : 'message',
    multiple    : 'multiple',
    placeholder : 'default',
    sizer       : 'sizer',
    search      : 'search',
    selected    : 'selected',
    selection   : 'selection',
    upward      : 'upward',
    leftward    : 'left',
    visible     : 'visible',
    clearable   : 'clearable',
    noselection : 'noselection',
    delete      : 'delete',
    header      : 'header',
    divider     : 'divider',
    groupIcon   : ''
  }

};

/* Templates */
$.fn.dropdown.settings.templates = {
  deQuote: function(string) {
      return String(string).replace(/"/g,"");
  },
  escape: function(string, preserveHTML) {
    if (preserveHTML){
      return string;
    }
    var
        badChars     = /[&<>"'`]/g,
        shouldEscape = /[&<>"'`]/,
        escape       = {
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#x27;",
          "`": "&#x60;"
        },
        escapedChar  = function(chr) {
          return escape[chr];
        }
    ;
    if(shouldEscape.test(string)) {
      return string.replace(badChars, escapedChar);
    }
    return string;
  },
  // generates dropdown from select values
  dropdown: function(select, fields, preserveHTML, className) {
    var
      placeholder = select.placeholder || false,
      html        = '',
      escape = $.fn.dropdown.settings.templates.escape
    ;
    html +=  '<i class="dropdown icon"></i>';
    if(placeholder) {
      html += '<div class="default text">' + escape(placeholder,preserveHTML) + '</div>';
    }
    else {
      html += '<div class="text"></div>';
    }
    html += '<div class="'+className.menu+'">';
    html += $.fn.dropdown.settings.templates.menu(select, fields, preserveHTML,className);
    html += '</div>';
    return html;
  },

  // generates just menu from select
  menu: function(response, fields, preserveHTML, className) {
    var
      values = response[fields.values] || [],
      html   = '',
      escape = $.fn.dropdown.settings.templates.escape,
      deQuote = $.fn.dropdown.settings.templates.deQuote
    ;
    $.each(values, function(index, option) {
      var
        itemType = (option[fields.type])
          ? option[fields.type]
          : 'item'
      ;

      if( itemType === 'item' ) {
        var
          maybeText = (option[fields.text])
            ? ' data-text="' + deQuote(option[fields.text]) + '"'
            : '',
          maybeDisabled = (option[fields.disabled])
            ? className.disabled+' '
            : ''
        ;
        html += '<div class="'+ maybeDisabled + (option[fields.class] ? deQuote(option[fields.class]) : className.item)+'" data-value="' + deQuote(option[fields.value]) + '"' + maybeText + '>';
        if(option[fields.image]) {
          html += '<img class="'+(option[fields.imageClass] ? deQuote(option[fields.imageClass]) : className.image)+'" src="' + deQuote(option[fields.image]) + '">';
        }
        if(option[fields.icon]) {
          html += '<i class="'+deQuote(option[fields.icon])+' '+(option[fields.iconClass] ? deQuote(option[fields.iconClass]) : className.icon)+'"></i>';
        }
        html +=   escape(option[fields.name] || option[fields.value],preserveHTML);
        html += '</div>';
      } else if (itemType === 'header') {
        var groupName = escape(option[fields.name],preserveHTML),
            groupIcon = option[fields.icon] ? deQuote(option[fields.icon]) : className.groupIcon
        ;
        if(groupName !== '' || groupIcon !== '') {
          html += '<div class="' + (option[fields.class] ? deQuote(option[fields.class]) : className.header) + '">';
          if (groupIcon !== '') {
            html += '<i class="' + groupIcon + ' ' + (option[fields.iconClass] ? deQuote(option[fields.iconClass]) : className.icon) + '"></i>';
          }
          html += groupName;
          html += '</div>';
        }
        if(option[fields.divider]){
          html += '<div class="'+className.divider+'"></div>';
        }
      }
    });
    return html;
  },

  // generates label for multiselect
  label: function(value, text, preserveHTML, className) {
    var
        escape = $.fn.dropdown.settings.templates.escape;
    return escape(text,preserveHTML) + '<i class="'+className.delete+' icon"></i>';
  },


  // generates messages like "No results"
  message: function(message) {
    return message;
  },

  // generates user addition to selection menu
  addition: function(choice) {
    return choice;
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Embed
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

"use strict";

$.isFunction = $.isFunction || function(obj) {
  return typeof obj === "function" && typeof obj.nodeType !== "number";
};

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.embed = function(parameters) {

  var
    $allModules     = $(this),

    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.embed.settings, parameters)
          : $.extend({}, $.fn.embed.settings),

        selector        = settings.selector,
        className       = settings.className,
        sources         = settings.sources,
        error           = settings.error,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        templates       = settings.templates,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $placeholder    = $module.find(selector.placeholder),
        $icon           = $module.find(selector.icon),
        $embed          = $module.find(selector.embed),

        element         = this,
        instance        = $module.data(moduleNamespace),
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing embed');
          module.determine.autoplay();
          module.create();
          module.bind.events();
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous instance of embed');
          module.reset();
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
        },

        refresh: function() {
          module.verbose('Refreshing selector cache');
          $placeholder = $module.find(selector.placeholder);
          $icon        = $module.find(selector.icon);
          $embed       = $module.find(selector.embed);
        },

        bind: {
          events: function() {
            if( module.has.placeholder() ) {
              module.debug('Adding placeholder events');
              $module
                .on('click' + eventNamespace, selector.placeholder, module.createAndShow)
                .on('click' + eventNamespace, selector.icon, module.createAndShow)
              ;
            }
          }
        },

        create: function() {
          var
            placeholder = module.get.placeholder()
          ;
          if(placeholder) {
            module.createPlaceholder();
          }
          else {
            module.createAndShow();
          }
        },

        createPlaceholder: function(placeholder) {
          var
            icon  = module.get.icon(),
            url   = module.get.url(),
            embed = module.generate.embed(url)
          ;
          placeholder = placeholder || module.get.placeholder();
          $module.html( templates.placeholder(placeholder, icon) );
          module.debug('Creating placeholder for embed', placeholder, icon);
        },

        createEmbed: function(url) {
          module.refresh();
          url = url || module.get.url();
          $embed = $('<div/>')
            .addClass(className.embed)
            .html( module.generate.embed(url) )
            .appendTo($module)
          ;
          settings.onCreate.call(element, url);
          module.debug('Creating embed object', $embed);
        },

        changeEmbed: function(url) {
          $embed
            .html( module.generate.embed(url) )
          ;
        },

        createAndShow: function() {
          module.createEmbed();
          module.show();
        },

        // sets new embed
        change: function(source, id, url) {
          module.debug('Changing video to ', source, id, url);
          $module
            .data(metadata.source, source)
            .data(metadata.id, id)
          ;
          if(url) {
            $module.data(metadata.url, url);
          }
          else {
            $module.removeData(metadata.url);
          }
          if(module.has.embed()) {
            module.changeEmbed();
          }
          else {
            module.create();
          }
        },

        // clears embed
        reset: function() {
          module.debug('Clearing embed and showing placeholder');
          module.remove.data();
          module.remove.active();
          module.remove.embed();
          module.showPlaceholder();
          settings.onReset.call(element);
        },

        // shows current embed
        show: function() {
          module.debug('Showing embed');
          module.set.active();
          settings.onDisplay.call(element);
        },

        hide: function() {
          module.debug('Hiding embed');
          module.showPlaceholder();
        },

        showPlaceholder: function() {
          module.debug('Showing placeholder image');
          module.remove.active();
          settings.onPlaceholderDisplay.call(element);
        },

        get: {
          id: function() {
            return settings.id || $module.data(metadata.id);
          },
          placeholder: function() {
            return settings.placeholder || $module.data(metadata.placeholder);
          },
          icon: function() {
            return (settings.icon)
              ? settings.icon
              : ($module.data(metadata.icon) !== undefined)
                ? $module.data(metadata.icon)
                : module.determine.icon()
            ;
          },
          source: function(url) {
            return (settings.source)
              ? settings.source
              : ($module.data(metadata.source) !== undefined)
                ? $module.data(metadata.source)
                : module.determine.source()
            ;
          },
          type: function() {
            var source = module.get.source();
            return (sources[source] !== undefined)
              ? sources[source].type
              : false
            ;
          },
          url: function() {
            return (settings.url)
              ? settings.url
              : ($module.data(metadata.url) !== undefined)
                ? $module.data(metadata.url)
                : module.determine.url()
            ;
          }
        },

        determine: {
          autoplay: function() {
            if(module.should.autoplay()) {
              settings.autoplay = true;
            }
          },
          source: function(url) {
            var
              matchedSource = false
            ;
            url = url || module.get.url();
            if(url) {
              $.each(sources, function(name, source) {
                if(url.search(source.domain) !== -1) {
                  matchedSource = name;
                  return false;
                }
              });
            }
            return matchedSource;
          },
          icon: function() {
            var
              source = module.get.source()
            ;
            return (sources[source] !== undefined)
              ? sources[source].icon
              : false
            ;
          },
          url: function() {
            var
              id     = settings.id     || $module.data(metadata.id),
              source = settings.source || $module.data(metadata.source),
              url
            ;
            url = (sources[source] !== undefined)
              ? sources[source].url.replace('{id}', id)
              : false
            ;
            if(url) {
              $module.data(metadata.url, url);
            }
            return url;
          }
        },


        set: {
          active: function() {
            $module.addClass(className.active);
          }
        },

        remove: {
          data: function() {
            $module
              .removeData(metadata.id)
              .removeData(metadata.icon)
              .removeData(metadata.placeholder)
              .removeData(metadata.source)
              .removeData(metadata.url)
            ;
          },
          active: function() {
            $module.removeClass(className.active);
          },
          embed: function() {
            $embed.empty();
          }
        },

        encode: {
          parameters: function(parameters) {
            var
              urlString = [],
              index
            ;
            for (index in parameters) {
              urlString.push( encodeURIComponent(index) + '=' + encodeURIComponent( parameters[index] ) );
            }
            return urlString.join('&amp;');
          }
        },

        generate: {
          embed: function(url) {
            module.debug('Generating embed html');
            var
              source = module.get.source(),
              html,
              parameters
            ;
            url = module.get.url(url);
            if(url) {
              parameters = module.generate.parameters(source);
              html       = templates.iframe(url, parameters);
            }
            else {
              module.error(error.noURL, $module);
            }
            return html;
          },
          parameters: function(source, extraParameters) {
            var
              parameters = (sources[source] && sources[source].parameters !== undefined)
                ? sources[source].parameters(settings)
                : {}
            ;
            extraParameters = extraParameters || settings.parameters;
            if(extraParameters) {
              parameters = $.extend({}, parameters, extraParameters);
            }
            parameters = settings.onEmbed(parameters);
            return module.encode.parameters(parameters);
          }
        },

        has: {
          embed: function() {
            return ($embed.length > 0);
          },
          placeholder: function() {
            return settings.placeholder || $module.data(metadata.placeholder);
          }
        },

        should: {
          autoplay: function() {
            return (settings.autoplay === 'auto')
              ? (settings.placeholder || $module.data(metadata.placeholder) !== undefined)
              : settings.autoplay
            ;
          }
        },

        is: {
          video: function() {
            return module.get.type() == 'video';
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;
  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.embed.settings = {

  name        : 'Embed',
  namespace   : 'embed',

  silent      : false,
  debug       : false,
  verbose     : false,
  performance : true,

  icon     : false,
  source   : false,
  url      : false,
  id       : false,

  // standard video settings
  autoplay  : 'auto',
  color     : '#444444',
  hd        : true,
  brandedUI : false,

  // additional parameters to include with the embed
  parameters: false,

  onDisplay            : function() {},
  onPlaceholderDisplay : function() {},
  onReset              : function() {},
  onCreate             : function(url) {},
  onEmbed              : function(parameters) {
    return parameters;
  },

  metadata    : {
    id          : 'id',
    icon        : 'icon',
    placeholder : 'placeholder',
    source      : 'source',
    url         : 'url'
  },

  error : {
    noURL  : 'No URL specified',
    method : 'The method you called is not defined'
  },

  className : {
    active : 'active',
    embed  : 'embed'
  },

  selector : {
    embed       : '.embed',
    placeholder : '.placeholder',
    icon        : '.icon'
  },

  sources: {
    youtube: {
      name   : 'youtube',
      type   : 'video',
      icon   : 'video play',
      domain : 'youtube.com',
      url    : '//www.youtube.com/embed/{id}',
      parameters: function(settings) {
        return {
          autohide       : !settings.brandedUI,
          autoplay       : settings.autoplay,
          color          : settings.color || undefined,
          hq             : settings.hd,
          jsapi          : settings.api,
          modestbranding : !settings.brandedUI
        };
      }
    },
    vimeo: {
      name   : 'vimeo',
      type   : 'video',
      icon   : 'video play',
      domain : 'vimeo.com',
      url    : '//player.vimeo.com/video/{id}',
      parameters: function(settings) {
        return {
          api      : settings.api,
          autoplay : settings.autoplay,
          byline   : settings.brandedUI,
          color    : settings.color || undefined,
          portrait : settings.brandedUI,
          title    : settings.brandedUI
        };
      }
    }
  },

  templates: {
    iframe : function(url, parameters) {
      var src = url;
      if (parameters) {
          src += '?' + parameters;
      }
      return ''
        + '<iframe src="' + src + '"'
        + ' width="100%" height="100%"'
        + ' webkitAllowFullScreen mozallowfullscreen allowFullScreen></iframe>'
      ;
    },
    placeholder : function(image, icon) {
      var
        html = ''
      ;
      if(icon) {
        html += '<i class="' + icon + ' icon"></i>';
      }
      if(image) {
        html += '<img class="placeholder" src="' + image + '">';
      }
      return html;
    }
  },

  // NOT YET IMPLEMENTED
  api     : false,
  onPause : function() {},
  onPlay  : function() {},
  onStop  : function() {}

};



})( jQuery, window, document );

/*!
 * # Fomantic-UI - Modal
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

$.fn.modal = function(parameters) {
  var
    $allModules    = $(this),
    $window        = $(window),
    $document      = $(document),
    $body          = $('body'),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    requestAnimationFrame = window.requestAnimationFrame
      || window.mozRequestAnimationFrame
      || window.webkitRequestAnimationFrame
      || window.msRequestAnimationFrame
      || function(callback) { setTimeout(callback, 0); },

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings    = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.modal.settings, parameters)
          : $.extend({}, $.fn.modal.settings),

        selector        = settings.selector,
        className       = settings.className,
        namespace       = settings.namespace,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $context        = $(settings.context),
        $close          = $module.find(selector.close),

        $allModals,
        $otherModals,
        $focusedElement,
        $dimmable,
        $dimmer,

        element         = this,
        instance        = $module.data(moduleNamespace),

        ignoreRepeatedEvents = false,

        initialMouseDownInModal,
        initialMouseDownInScrollbar,
        initialBodyMargin = '',
        tempBodyMargin = '',

        elementEventNamespace,
        id,
        observer,
        module
      ;
      module  = {

        initialize: function() {
          module.cache = {};
          module.verbose('Initializing dimmer', $context);

          module.create.id();
          module.create.dimmer();

          if ( settings.allowMultiple ) {
            module.create.innerDimmer();
          }
          if (!settings.centered){
            $module.addClass('top aligned');
          }
          module.refreshModals();

          module.bind.events();
          if(settings.observeChanges) {
            module.observeChanges();
          }
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of modal');
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        create: {
          dimmer: function() {
            var
              defaultSettings = {
                debug      : settings.debug,
                dimmerName : 'modals'
              },
              dimmerSettings = $.extend(true, defaultSettings, settings.dimmerSettings)
            ;
            if($.fn.dimmer === undefined) {
              module.error(error.dimmer);
              return;
            }
            module.debug('Creating dimmer');
            $dimmable = $context.dimmer(dimmerSettings);
            if(settings.detachable) {
              module.verbose('Modal is detachable, moving content into dimmer');
              $dimmable.dimmer('add content', $module);
            }
            else {
              module.set.undetached();
            }
            $dimmer = $dimmable.dimmer('get dimmer');
          },
          id: function() {
            id = (Math.random().toString(16) + '000000000').substr(2, 8);
            elementEventNamespace = '.' + id;
            module.verbose('Creating unique id for element', id);
          },
          innerDimmer: function() {
            if ( $module.find(selector.dimmer).length == 0 ) {
              $module.prepend('<div class="ui inverted dimmer"></div>');
            }
          }
        },

        destroy: function() {
          if (observer) {
            observer.disconnect();
          }
          module.verbose('Destroying previous modal');
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
          $window.off(elementEventNamespace);
          $dimmer.off(elementEventNamespace);
          $close.off(eventNamespace);
          $context.dimmer('destroy');
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            observer = new MutationObserver(function(mutations) {
              module.debug('DOM tree modified, refreshing');
              module.refresh();
            });
            observer.observe(element, {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', observer);
          }
        },

        refresh: function() {
          module.remove.scrolling();
          module.cacheSizes();
          if(!module.can.useFlex()) {
            module.set.modalOffset();
          }
          module.set.screenHeight();
          module.set.type();
        },

        refreshModals: function() {
          $otherModals = $module.siblings(selector.modal);
          $allModals   = $otherModals.add($module);
        },

        attachEvents: function(selector, event) {
          var
            $toggle = $(selector)
          ;
          event = $.isFunction(module[event])
            ? module[event]
            : module.toggle
          ;
          if($toggle.length > 0) {
            module.debug('Attaching modal events to element', selector, event);
            $toggle
              .off(eventNamespace)
              .on('click' + eventNamespace, event)
            ;
          }
          else {
            module.error(error.notFound, selector);
          }
        },

        bind: {
          events: function() {
            module.verbose('Attaching events');
            $module
              .on('click' + eventNamespace, selector.close, module.event.close)
              .on('click' + eventNamespace, selector.approve, module.event.approve)
              .on('click' + eventNamespace, selector.deny, module.event.deny)
            ;
            $window
              .on('resize' + elementEventNamespace, module.event.resize)
            ;
          },
          scrollLock: function() {
            // touch events default to passive, due to changes in chrome to optimize mobile perf
            $dimmable.get(0).addEventListener('touchmove', module.event.preventScroll, { passive: false });
          }
        },

        unbind: {
          scrollLock: function() {
            $dimmable.get(0).removeEventListener('touchmove', module.event.preventScroll, { passive: false });
          }
        },

        get: {
          id: function() {
            return (Math.random().toString(16) + '000000000').substr(2, 8);
          }
        },

        event: {
          approve: function() {
            if(ignoreRepeatedEvents || settings.onApprove.call(element, $(this)) === false) {
              module.verbose('Approve callback returned false cancelling hide');
              return;
            }
            ignoreRepeatedEvents = true;
            module.hide(function() {
              ignoreRepeatedEvents = false;
            });
          },
          preventScroll: function(event) {
            if(event.target.className.indexOf('dimmer') !== -1) {
              event.preventDefault();
            }
          },
          deny: function() {
            if(ignoreRepeatedEvents || settings.onDeny.call(element, $(this)) === false) {
              module.verbose('Deny callback returned false cancelling hide');
              return;
            }
            ignoreRepeatedEvents = true;
            module.hide(function() {
              ignoreRepeatedEvents = false;
            });
          },
          close: function() {
            module.hide();
          },
          mousedown: function(event) {
            var
              $target   = $(event.target),
              isRtl = module.is.rtl();
            ;
            initialMouseDownInModal = ($target.closest(selector.modal).length > 0);
            if(initialMouseDownInModal) {
              module.verbose('Mouse down event registered inside the modal');
            }
            initialMouseDownInScrollbar = module.is.scrolling() && ((!isRtl && $(window).outerWidth() - settings.scrollbarWidth <= event.clientX) || (isRtl && settings.scrollbarWidth >= event.clientX));
            if(initialMouseDownInScrollbar) {
              module.verbose('Mouse down event registered inside the scrollbar');
            }
          },
          mouseup: function(event) {
            if(!settings.closable) {
              module.verbose('Dimmer clicked but closable setting is disabled');
              return;
            }
            if(initialMouseDownInModal) {
              module.debug('Dimmer clicked but mouse down was initially registered inside the modal');
              return;
            }
            if(initialMouseDownInScrollbar){
              module.debug('Dimmer clicked but mouse down was initially registered inside the scrollbar');
              return;
            }
            var
              $target   = $(event.target),
              isInModal = ($target.closest(selector.modal).length > 0),
              isInDOM   = $.contains(document.documentElement, event.target)
            ;
            if(!isInModal && isInDOM && module.is.active() && $module.hasClass(className.front) ) {
              module.debug('Dimmer clicked, hiding all modals');
              if(settings.allowMultiple) {
                if(!module.hideAll()) {
                  return;
                }
              }
              else if(!module.hide()){
                  return;
              }
              module.remove.clickaway();
            }
          },
          debounce: function(method, delay) {
            clearTimeout(module.timer);
            module.timer = setTimeout(method, delay);
          },
          keyboard: function(event) {
            var
              keyCode   = event.which,
              escapeKey = 27
            ;
            if(keyCode == escapeKey) {
              if(settings.closable) {
                module.debug('Escape key pressed hiding modal');
                if ( $module.hasClass(className.front) ) {
                  module.hide();
                }
              }
              else {
                module.debug('Escape key pressed, but closable is set to false');
              }
              event.preventDefault();
            }
          },
          resize: function() {
            if( $dimmable.dimmer('is active') && ( module.is.animating() || module.is.active() ) ) {
              requestAnimationFrame(module.refresh);
            }
          }
        },

        toggle: function() {
          if( module.is.active() || module.is.animating() ) {
            module.hide();
          }
          else {
            module.show();
          }
        },

        show: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          module.refreshModals();
          module.set.dimmerSettings();
          module.set.dimmerStyles();

          module.showModal(callback);
        },

        hide: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          module.refreshModals();
          return module.hideModal(callback);
        },

        showModal: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( module.is.animating() || !module.is.active() ) {
            module.showDimmer();
            module.cacheSizes();
            module.set.bodyMargin();
            if(module.can.useFlex()) {
              module.remove.legacy();
            }
            else {
              module.set.legacy();
              module.set.modalOffset();
              module.debug('Using non-flex legacy modal positioning.');
            }
            module.set.screenHeight();
            module.set.type();
            module.set.clickaway();

            if( !settings.allowMultiple && module.others.active() ) {
              module.hideOthers(module.showModal);
            }
            else {
              ignoreRepeatedEvents = false;
              if( settings.allowMultiple ) {
                if ( module.others.active() ) {
                  $otherModals.filter('.' + className.active).find(selector.dimmer).addClass('active');
                }

                if ( settings.detachable ) {
                  $module.detach().appendTo($dimmer);
                }
              }
              settings.onShow.call(element);
              if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
                module.debug('Showing modal with css animations');
                $module
                  .transition({
                    debug       : settings.debug,
                    animation   : settings.transition + ' in',
                    queue       : settings.queue,
                    duration    : settings.duration,
                    useFailSafe : true,
                    onComplete : function() {
                      settings.onVisible.apply(element);
                      if(settings.keyboardShortcuts) {
                        module.add.keyboardShortcuts();
                      }
                      module.save.focus();
                      module.set.active();
                      if(settings.autofocus) {
                        module.set.autofocus();
                      }
                      callback();
                    }
                  })
                ;
              }
              else {
                module.error(error.noTransition);
              }
            }
          }
          else {
            module.debug('Modal is already visible');
          }
        },

        hideModal: function(callback, keepDimmed, hideOthersToo) {
          var
            $previousModal = $otherModals.filter('.' + className.active).last()
          ;
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          module.debug('Hiding modal');
          if(settings.onHide.call(element, $(this)) === false) {
            module.verbose('Hide callback returned false cancelling hide');
            ignoreRepeatedEvents = false;
            return false;
          }

          if( module.is.animating() || module.is.active() ) {
            if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
              module.remove.active();
              $module
                .transition({
                  debug       : settings.debug,
                  animation   : settings.transition + ' out',
                  queue       : settings.queue,
                  duration    : settings.duration,
                  useFailSafe : true,
                  onStart     : function() {
                    if(!module.others.active() && !module.others.animating() && !keepDimmed) {
                      module.hideDimmer();
                    }
                    if( settings.keyboardShortcuts && !module.others.active() ) {
                      module.remove.keyboardShortcuts();
                    }
                  },
                  onComplete : function() {
                    module.unbind.scrollLock();
                    if ( settings.allowMultiple ) {
                      $previousModal.addClass(className.front);
                      $module.removeClass(className.front);

                      if ( hideOthersToo ) {
                        $allModals.find(selector.dimmer).removeClass('active');
                      }
                      else {
                        $previousModal.find(selector.dimmer).removeClass('active');
                      }
                    }
                    settings.onHidden.call(element);
                    module.remove.dimmerStyles();
                    module.restore.focus();
                    callback();
                  }
                })
              ;
            }
            else {
              module.error(error.noTransition);
            }
          }
        },

        showDimmer: function() {
          if($dimmable.dimmer('is animating') || !$dimmable.dimmer('is active') ) {
            module.save.bodyMargin();
            module.debug('Showing dimmer');
            $dimmable.dimmer('show');
          }
          else {
            module.debug('Dimmer already visible');
          }
        },

        hideDimmer: function() {
          if( $dimmable.dimmer('is animating') || ($dimmable.dimmer('is active')) ) {
            module.unbind.scrollLock();
            $dimmable.dimmer('hide', function() {
              module.restore.bodyMargin();
              module.remove.clickaway();
              module.remove.screenHeight();
            });
          }
          else {
            module.debug('Dimmer is not visible cannot hide');
            return;
          }
        },

        hideAll: function(callback) {
          var
            $visibleModals = $allModals.filter('.' + className.active + ', .' + className.animating)
          ;
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( $visibleModals.length > 0 ) {
            module.debug('Hiding all visible modals');
            var hideOk = true;
//check in reverse order trying to hide most top displayed modal first
            $($visibleModals.get().reverse()).each(function(index,element){
                if(hideOk){
                    hideOk = $(element).modal('hide modal', callback, false, true);
                }
            });
            if(hideOk) {
              module.hideDimmer();
            }
            return hideOk;
          }
        },

        hideOthers: function(callback) {
          var
            $visibleModals = $otherModals.filter('.' + className.active + ', .' + className.animating)
          ;
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( $visibleModals.length > 0 ) {
            module.debug('Hiding other modals', $otherModals);
            $visibleModals
              .modal('hide modal', callback, true)
            ;
          }
        },

        others: {
          active: function() {
            return ($otherModals.filter('.' + className.active).length > 0);
          },
          animating: function() {
            return ($otherModals.filter('.' + className.animating).length > 0);
          }
        },


        add: {
          keyboardShortcuts: function() {
            module.verbose('Adding keyboard shortcuts');
            $document
              .on('keyup' + eventNamespace, module.event.keyboard)
            ;
          }
        },

        save: {
          focus: function() {
            var
              $activeElement = $(document.activeElement),
              inCurrentModal = $activeElement.closest($module).length > 0
            ;
            if(!inCurrentModal) {
              $focusedElement = $(document.activeElement).blur();
            }
          },
          bodyMargin: function() {
            initialBodyMargin = $body.css('margin-'+(module.can.leftBodyScrollbar() ? 'left':'right'));
            var bodyMarginRightPixel = parseInt(initialBodyMargin.replace(/[^\d.]/g, '')),
                bodyScrollbarWidth = window.innerWidth - document.documentElement.clientWidth;
            tempBodyMargin = bodyMarginRightPixel + bodyScrollbarWidth;
          }
        },

        restore: {
          focus: function() {
            if($focusedElement && $focusedElement.length > 0 && settings.restoreFocus) {
              $focusedElement.focus();
            }
          },
          bodyMargin: function() {
            var position = module.can.leftBodyScrollbar() ? 'left':'right';
            $body.css('margin-'+position, initialBodyMargin);
            $body.find(selector.bodyFixed.replace('right',position)).css('padding-'+position, initialBodyMargin);
          }
        },

        remove: {
          active: function() {
            $module.removeClass(className.active);
          },
          legacy: function() {
            $module.removeClass(className.legacy);
          },
          clickaway: function() {
            if (!settings.detachable) {
              $module
                  .off('mousedown' + elementEventNamespace)
              ;
            }           
            $dimmer
              .off('mousedown' + elementEventNamespace)
            ;
            $dimmer
              .off('mouseup' + elementEventNamespace)
            ;
          },
          dimmerStyles: function() {
            $dimmer.removeClass(className.inverted);
            $dimmable.removeClass(className.blurring);
          },
          bodyStyle: function() {
            if($body.attr('style') === '') {
              module.verbose('Removing style attribute');
              $body.removeAttr('style');
            }
          },
          screenHeight: function() {
            module.debug('Removing page height');
            $body
              .css('height', '')
            ;
          },
          keyboardShortcuts: function() {
            module.verbose('Removing keyboard shortcuts');
            $document
              .off('keyup' + eventNamespace)
            ;
          },
          scrolling: function() {
            $dimmable.removeClass(className.scrolling);
            $module.removeClass(className.scrolling);
          }
        },

        cacheSizes: function() {
          $module.addClass(className.loading);
          var
            scrollHeight = $module.prop('scrollHeight'),
            modalWidth   = $module.outerWidth(),
            modalHeight  = $module.outerHeight()
          ;
          if(module.cache.pageHeight === undefined || modalHeight !== 0) {
            $.extend(module.cache, {
              pageHeight    : $(document).outerHeight(),
              width         : modalWidth,
              height        : modalHeight + settings.offset,
              scrollHeight  : scrollHeight + settings.offset,
              contextHeight : (settings.context == 'body')
                ? $(window).height()
                : $dimmable.height(),
            });
            module.cache.topOffset = -(module.cache.height / 2);
          }
          $module.removeClass(className.loading);
          module.debug('Caching modal and container sizes', module.cache);
        },

        can: {
          leftBodyScrollbar: function(){
            if(module.cache.leftBodyScrollbar === undefined) {
              module.cache.leftBodyScrollbar = module.is.rtl() && ((module.is.iframe && !module.is.firefox()) || module.is.safari() || module.is.edge() || module.is.ie());
            }
            return module.cache.leftBodyScrollbar;
          },
          useFlex: function() {
            return settings.useFlex && settings.detachable && !module.is.ie();
          },
          fit: function() {
            var
              contextHeight  = module.cache.contextHeight,
              verticalCenter = module.cache.contextHeight / 2,
              topOffset      = module.cache.topOffset,
              scrollHeight   = module.cache.scrollHeight,
              height         = module.cache.height,
              paddingHeight  = settings.padding,
              startPosition  = (verticalCenter + topOffset)
            ;
            return (scrollHeight > height)
              ? (startPosition + scrollHeight + paddingHeight < contextHeight)
              : (height + (paddingHeight * 2) < contextHeight)
            ;
          }
        },

        is: {
          active: function() {
            return $module.hasClass(className.active);
          },
          ie: function() {
            if(module.cache.isIE === undefined) {
              var
                  isIE11 = (!(window.ActiveXObject) && 'ActiveXObject' in window),
                  isIE = ('ActiveXObject' in window)
              ;
              module.cache.isIE = (isIE11 || isIE);
            }
            return module.cache.isIE;
          },
          animating: function() {
            return $module.transition('is supported')
              ? $module.transition('is animating')
              : $module.is(':visible')
            ;
          },
          scrolling: function() {
            return $dimmable.hasClass(className.scrolling);
          },
          modernBrowser: function() {
            // appName for IE11 reports 'Netscape' can no longer use
            return !(window.ActiveXObject || 'ActiveXObject' in window);
          },
          rtl: function() {
            if(module.cache.isRTL === undefined) {
              module.cache.isRTL = $body.attr('dir') === 'rtl' || $body.css('direction') === 'rtl';
            }
            return module.cache.isRTL;
          },
          safari: function() {
            if(module.cache.isSafari === undefined) {
              module.cache.isSafari = /constructor/i.test(window.HTMLElement) || !!window.ApplePaySession;
            }
            return module.cache.isSafari;
          },
          edge: function(){
            if(module.cache.isEdge === undefined) {
              module.cache.isEdge = !!window.setImmediate && !module.is.ie();
            }
            return module.cache.isEdge;
          },
          firefox: function(){
            if(module.cache.isFirefox === undefined) {
                module.cache.isFirefox = !!window.InstallTrigger;
            }
            return module.cache.isFirefox;
          },
          iframe: function() {
              return !(self === top);
          }
        },

        set: {
          autofocus: function() {
            var
              $inputs    = $module.find('[tabindex], :input').filter(':visible').filter(function() {
                return $(this).closest('.disabled').length === 0;
              }),
              $autofocus = $inputs.filter('[autofocus]'),
              $input     = ($autofocus.length > 0)
                ? $autofocus.first()
                : $inputs.first()
            ;
            if($input.length > 0) {
              $input.focus();
            }
          },
          bodyMargin: function() {
            var position = module.can.leftBodyScrollbar() ? 'left':'right';
            if(settings.detachable || module.can.fit()) {
              $body.css('margin-'+position, tempBodyMargin + 'px');
            }
            $body.find(selector.bodyFixed.replace('right',position)).css('padding-'+position, tempBodyMargin + 'px');
          },
          clickaway: function() {
            if (!settings.detachable) {
              $module
                .on('mousedown' + elementEventNamespace, module.event.mousedown)
              ;
            }
            $dimmer
              .on('mousedown' + elementEventNamespace, module.event.mousedown)
            ;
            $dimmer
              .on('mouseup' + elementEventNamespace, module.event.mouseup)
            ;
          },
          dimmerSettings: function() {
            if($.fn.dimmer === undefined) {
              module.error(error.dimmer);
              return;
            }
            var
              defaultSettings = {
                debug      : settings.debug,
                dimmerName : 'modals',
                closable   : 'auto',
                useFlex    : module.can.useFlex(),
                duration   : {
                  show     : settings.duration,
                  hide     : settings.duration
                }
              },
              dimmerSettings = $.extend(true, defaultSettings, settings.dimmerSettings)
            ;
            if(settings.inverted) {
              dimmerSettings.variation = (dimmerSettings.variation !== undefined)
                ? dimmerSettings.variation + ' inverted'
                : 'inverted'
              ;
            }
            $context.dimmer('setting', dimmerSettings);
          },
          dimmerStyles: function() {
            if(settings.inverted) {
              $dimmer.addClass(className.inverted);
            }
            else {
              $dimmer.removeClass(className.inverted);
            }
            if(settings.blurring) {
              $dimmable.addClass(className.blurring);
            }
            else {
              $dimmable.removeClass(className.blurring);
            }
          },
          modalOffset: function() {
            if (!settings.detachable) {
              var canFit = module.can.fit();
              $module
                .css({
                  top: (!$module.hasClass('aligned') && canFit)
                    ? $(document).scrollTop() + (module.cache.contextHeight - module.cache.height) / 2
                    : !canFit || $module.hasClass('top')
                      ? $(document).scrollTop() + settings.padding
                      : $(document).scrollTop() + (module.cache.contextHeight - module.cache.height - settings.padding),
                  marginLeft: -(module.cache.width / 2)
                }) 
              ;
            } else {
              $module
                .css({
                  marginTop: (!$module.hasClass('aligned') && module.can.fit())
                    ? -(module.cache.height / 2)
                    : settings.padding / 2,
                  marginLeft: -(module.cache.width / 2)
                }) 
              ;
            }
            module.verbose('Setting modal offset for legacy mode');
          },
          screenHeight: function() {
            if( module.can.fit() ) {
              $body.css('height', '');
            }
            else if(!$module.hasClass('bottom')) {
              module.debug('Modal is taller than page content, resizing page height');
              $body
                .css('height', module.cache.height + (settings.padding * 2) )
              ;
            }
          },
          active: function() {
            $module.addClass(className.active + ' ' + className.front);
            $otherModals.filter('.' + className.active).removeClass(className.front);
          },
          scrolling: function() {
            $dimmable.addClass(className.scrolling);
            $module.addClass(className.scrolling);
            module.unbind.scrollLock();
          },
          legacy: function() {
            $module.addClass(className.legacy);
          },
          type: function() {
            if(module.can.fit()) {
              module.verbose('Modal fits on screen');
              if(!module.others.active() && !module.others.animating()) {
                module.remove.scrolling();
                module.bind.scrollLock();
              }
            }
            else if (!$module.hasClass('bottom')){
              module.verbose('Modal cannot fit on screen setting to scrolling');
              module.set.scrolling();
            } else {
                module.verbose('Bottom aligned modal not fitting on screen is unsupported for scrolling');
            }
          },
          undetached: function() {
            $dimmable.addClass(className.undetached);
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.modal.settings = {

  name           : 'Modal',
  namespace      : 'modal',

  useFlex        : 'auto',
  offset         : 0,

  silent         : false,
  debug          : false,
  verbose        : false,
  performance    : true,

  observeChanges : false,

  allowMultiple  : false,
  detachable     : true,
  closable       : true,
  autofocus      : true,
  restoreFocus   : true,

  inverted       : false,
  blurring       : false,

  centered       : true,

  dimmerSettings : {
    closable : false,
    useCSS   : true
  },

  // whether to use keyboard shortcuts
  keyboardShortcuts: true,

  context    : 'body',

  queue      : false,
  duration   : 500,
  transition : 'scale',

  // padding with edge of page
  padding    : 50,
  scrollbarWidth: 10,

  // called before show animation
  onShow     : function(){},

  // called after show animation
  onVisible  : function(){},

  // called before hide animation
  onHide     : function(){ return true; },

  // called after hide animation
  onHidden   : function(){},

  // called after approve selector match
  onApprove  : function(){ return true; },

  // called after deny selector match
  onDeny     : function(){ return true; },

  selector    : {
    close    : '> .close',
    approve  : '.actions .positive, .actions .approve, .actions .ok',
    deny     : '.actions .negative, .actions .deny, .actions .cancel',
    modal    : '.ui.modal',
    dimmer   : '> .ui.dimmer',
    bodyFixed: '> .ui.fixed.menu, > .ui.right.toast-container, > .ui.right.sidebar'
  },
  error : {
    dimmer    : 'UI Dimmer, a required component is not included in this page',
    method    : 'The method you called is not defined.',
    notFound  : 'The element you specified could not be found'
  },
  className : {
    active     : 'active',
    animating  : 'animating',
    blurring   : 'blurring',
    inverted   : 'inverted',
    legacy     : 'legacy',
    loading    : 'loading',
    scrolling  : 'scrolling',
    undetached : 'undetached',
    front      : 'front'
  }
};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Nag
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

$.fn.nag = function(parameters) {
  var
    $allModules    = $(this),
    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.nag.settings, parameters)
          : $.extend({}, $.fn.nag.settings),

        selector        = settings.selector,
        error           = settings.error,
        namespace       = settings.namespace,

        eventNamespace  = '.' + namespace,
        moduleNamespace = namespace + '-module',

        $module         = $(this),

        $context        = (settings.context)
          ? $(settings.context)
          : $('body'),

        element         = this,
        instance        = $module.data(moduleNamespace),

        module
      ;
      module = {

        initialize: function() {
          module.verbose('Initializing element');

          $module
            .on('click' + eventNamespace, selector.close, module.dismiss)
            .data(moduleNamespace, module)
          ;

          if(settings.detachable && $module.parent()[0] !== $context[0]) {
            $module
              .detach()
              .prependTo($context)
            ;
          }

          if(settings.displayTime > 0) {
            setTimeout(module.hide, settings.displayTime);
          }
          module.show();
        },

        destroy: function() {
          module.verbose('Destroying instance');
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
        },

        show: function() {
          if( module.should.show() && !$module.is(':visible') ) {
            module.debug('Showing nag', settings.animation.show);
            if(settings.animation.show == 'fade') {
              $module
                .fadeIn(settings.duration, settings.easing)
              ;
            }
            else {
              $module
                .slideDown(settings.duration, settings.easing)
              ;
            }
          }
        },

        hide: function() {
          module.debug('Showing nag', settings.animation.hide);
          if(settings.animation.show == 'fade') {
            $module
              .fadeIn(settings.duration, settings.easing)
            ;
          }
          else {
            $module
              .slideUp(settings.duration, settings.easing)
            ;
          }
        },

        onHide: function() {
          module.debug('Removing nag', settings.animation.hide);
          $module.remove();
          if (settings.onHide) {
            settings.onHide();
          }
        },

        dismiss: function(event) {
          if(settings.storageMethod) {
            module.storage.set(settings.key, settings.value);
          }
          module.hide();
          event.stopImmediatePropagation();
          event.preventDefault();
        },

        should: {
          show: function() {
            if(settings.persist) {
              module.debug('Persistent nag is set, can show nag');
              return true;
            }
            if( module.storage.get(settings.key) != settings.value.toString() ) {
              module.debug('Stored value is not set, can show nag', module.storage.get(settings.key));
              return true;
            }
            module.debug('Stored value is set, cannot show nag', module.storage.get(settings.key));
            return false;
          }
        },

        get: {
          storageOptions: function() {
            var
              options = {}
            ;
            if(settings.expires) {
              options.expires = settings.expires;
            }
            if(settings.domain) {
              options.domain = settings.domain;
            }
            if(settings.path) {
              options.path = settings.path;
            }
            return options;
          }
        },

        clear: function() {
          module.storage.remove(settings.key);
        },

        storage: {
          set: function(key, value) {
            var
              options = module.get.storageOptions()
            ;
            if(settings.storageMethod == 'localstorage' && window.localStorage !== undefined) {
              window.localStorage.setItem(key, value);
              module.debug('Value stored using local storage', key, value);
            }
            else if(settings.storageMethod == 'sessionstorage' && window.sessionStorage !== undefined) {
              window.sessionStorage.setItem(key, value);
              module.debug('Value stored using session storage', key, value);
            }
            else if($.cookie !== undefined) {
              $.cookie(key, value, options);
              module.debug('Value stored using cookie', key, value, options);
            }
            else {
              module.error(error.noCookieStorage);
              return;
            }
          },
          get: function(key, value) {
            var
              storedValue
            ;
            if(settings.storageMethod == 'localstorage' && window.localStorage !== undefined) {
              storedValue = window.localStorage.getItem(key);
            }
            else if(settings.storageMethod == 'sessionstorage' && window.sessionStorage !== undefined) {
              storedValue = window.sessionStorage.getItem(key);
            }
            // get by cookie
            else if($.cookie !== undefined) {
              storedValue = $.cookie(key);
            }
            else {
              module.error(error.noCookieStorage);
            }
            if(storedValue == 'undefined' || storedValue == 'null' || storedValue === undefined || storedValue === null) {
              storedValue = undefined;
            }
            return storedValue;
          },
          remove: function(key) {
            var
              options = module.get.storageOptions()
            ;
            if(settings.storageMethod == 'localstorage' && window.localStorage !== undefined) {
              window.localStorage.removeItem(key);
            }
            else if(settings.storageMethod == 'sessionstorage' && window.sessionStorage !== undefined) {
              window.sessionStorage.removeItem(key);
            }
            // store by cookie
            else if($.cookie !== undefined) {
              $.removeCookie(key, options);
            }
            else {
              module.error(error.noStorage);
            }
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.nag.settings = {

  name        : 'Nag',

  silent      : false,
  debug       : false,
  verbose     : false,
  performance : true,

  namespace   : 'Nag',

  // allows cookie to be overridden
  persist     : false,

  // set to zero to require manually dismissal, otherwise hides on its own
  displayTime : 0,

  animation   : {
    show : 'slide',
    hide : 'slide'
  },

  context       : false,
  detachable    : false,

  expires       : 30,
  domain        : false,
  path          : '/',

  // type of storage to use
  storageMethod : 'cookie',

  // value to store in dismissed localstorage/cookie
  key           : 'nag',
  value         : 'dismiss',

  error: {
    noCookieStorage : '$.cookie is not included. A storage solution is required.',
    noStorage       : 'Neither $.cookie or store is defined. A storage solution is required for storing state',
    method          : 'The method you called is not defined.'
  },

  className     : {
    bottom : 'bottom',
    fixed  : 'fixed'
  },

  selector      : {
    close : '.close.icon'
  },

  speed         : 500,
  easing        : 'easeOutQuad',

  onHide: function() {}

};

// Adds easing
$.extend( $.easing, {
  easeOutQuad: function (x, t, b, c, d) {
    return -c *(t/=d)*(t-2) + b;
  }
});

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Popup
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

$.fn.popup = function(parameters) {
  var
    $allModules    = $(this),
    $document      = $(document),
    $window        = $(window),
    $body          = $('body'),

    moduleSelector = $allModules.selector || '',

    clickEvent      = ('ontouchstart' in document.documentElement)
        ? 'touchstart'
        : 'click',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.popup.settings, parameters)
          : $.extend({}, $.fn.popup.settings),

        selector           = settings.selector,
        className          = settings.className,
        error              = settings.error,
        metadata           = settings.metadata,
        namespace          = settings.namespace,

        eventNamespace     = '.' + settings.namespace,
        moduleNamespace    = 'module-' + namespace,

        $module            = $(this),
        $context           = $(settings.context),
        $scrollContext     = $(settings.scrollContext),
        $boundary          = $(settings.boundary),
        $target            = (settings.target)
          ? $(settings.target)
          : $module,

        $popup,
        $offsetParent,

        searchDepth        = 0,
        triedPositions     = false,
        openedWithTouch    = false,

        element            = this,
        instance           = $module.data(moduleNamespace),

        documentObserver,
        elementNamespace,
        id,
        module
      ;

      module = {

        // binds events
        initialize: function() {
          module.debug('Initializing', $module);
          module.createID();
          module.bind.events();
          if(!module.exists() && settings.preserve) {
            module.create();
          }
          if(settings.observeChanges) {
            module.observeChanges();
          }
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance', module);
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            documentObserver = new MutationObserver(module.event.documentChanged);
            documentObserver.observe(document, {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', documentObserver);
          }
        },

        refresh: function() {
          if(settings.popup) {
            $popup = $(settings.popup).eq(0);
          }
          else {
            if(settings.inline) {
              $popup = $target.nextAll(selector.popup).eq(0);
              settings.popup = $popup;
            }
          }
          if(settings.popup) {
            $popup.addClass(className.loading);
            $offsetParent = module.get.offsetParent();
            $popup.removeClass(className.loading);
            if(settings.movePopup && module.has.popup() && module.get.offsetParent($popup)[0] !== $offsetParent[0]) {
              module.debug('Moving popup to the same offset parent as target');
              $popup
                .detach()
                .appendTo($offsetParent)
              ;
            }
          }
          else {
            $offsetParent = (settings.inline)
              ? module.get.offsetParent($target)
              : module.has.popup()
                ? module.get.offsetParent($popup)
                : $body
            ;
          }
          if( $offsetParent.is('html') && $offsetParent[0] !== $body[0] ) {
            module.debug('Setting page as offset parent');
            $offsetParent = $body;
          }
          if( module.get.variation() ) {
            module.set.variation();
          }
        },

        reposition: function() {
          module.refresh();
          module.set.position();
        },

        destroy: function() {
          module.debug('Destroying previous module');
          if(documentObserver) {
            documentObserver.disconnect();
          }
          // remove element only if was created dynamically
          if($popup && !settings.preserve) {
            module.removePopup();
          }
          // clear all timeouts
          clearTimeout(module.hideTimer);
          clearTimeout(module.showTimer);
          // remove events
          module.unbind.close();
          module.unbind.events();
          $module
            .removeData(moduleNamespace)
          ;
        },

        event: {
          start:  function(event) {
            var
              delay = ($.isPlainObject(settings.delay))
                ? settings.delay.show
                : settings.delay
            ;
            clearTimeout(module.hideTimer);
            if(!openedWithTouch || (openedWithTouch && settings.addTouchEvents) ) {
              module.showTimer = setTimeout(module.show, delay);
            }
          },
          end:  function() {
            var
              delay = ($.isPlainObject(settings.delay))
                ? settings.delay.hide
                : settings.delay
            ;
            clearTimeout(module.showTimer);
            module.hideTimer = setTimeout(module.hide, delay);
          },
          touchstart: function(event) {
            openedWithTouch = true;
            if(settings.addTouchEvents) {
              module.show();
            }
          },
          resize: function() {
            if( module.is.visible() ) {
              module.set.position();
            }
          },
          documentChanged: function(mutations) {
            [].forEach.call(mutations, function(mutation) {
              if(mutation.removedNodes) {
                [].forEach.call(mutation.removedNodes, function(node) {
                  if(node == element || $(node).find(element).length > 0) {
                    module.debug('Element removed from DOM, tearing down events');
                    module.destroy();
                  }
                });
              }
            });
          },
          hideGracefully: function(event) {
            var
              $target = $(event.target),
              isInDOM = $.contains(document.documentElement, event.target),
              inPopup = ($target.closest(selector.popup).length > 0)
            ;
            // don't close on clicks inside popup
            if(event && !inPopup && isInDOM) {
              module.debug('Click occurred outside popup hiding popup');
              module.hide();
            }
            else {
              module.debug('Click was inside popup, keeping popup open');
            }
          }
        },

        // generates popup html from metadata
        create: function() {
          var
            html      = module.get.html(),
            title     = module.get.title(),
            content   = module.get.content()
          ;

          if(html || content || title) {
            module.debug('Creating pop-up html');
            if(!html) {
              html = settings.templates.popup({
                title   : title,
                content : content
              });
            }
            $popup = $('<div/>')
              .addClass(className.popup)
              .data(metadata.activator, $module)
              .html(html)
            ;
            if(settings.inline) {
              module.verbose('Inserting popup element inline', $popup);
              $popup
                .insertAfter($module)
              ;
            }
            else {
              module.verbose('Appending popup element to body', $popup);
              $popup
                .appendTo( $context )
              ;
            }
            module.refresh();
            module.set.variation();

            if(settings.hoverable) {
              module.bind.popup();
            }
            settings.onCreate.call($popup, element);
          }
          else if(settings.popup) {
            $(settings.popup).data(metadata.activator, $module);
            module.verbose('Used popup specified in settings');
            module.refresh();
            if(settings.hoverable) {
              module.bind.popup();
            }
          }
          else if($target.next(selector.popup).length !== 0) {
            module.verbose('Pre-existing popup found');
            settings.inline = true;
            settings.popup  = $target.next(selector.popup).data(metadata.activator, $module);
            module.refresh();
            if(settings.hoverable) {
              module.bind.popup();
            }
          }
          else {
            module.debug('No content specified skipping display', element);
          }
        },

        createID: function() {
          id = (Math.random().toString(16) + '000000000').substr(2, 8);
          elementNamespace = '.' + id;
          module.verbose('Creating unique id for element', id);
        },

        // determines popup state
        toggle: function() {
          module.debug('Toggling pop-up');
          if( module.is.hidden() ) {
            module.debug('Popup is hidden, showing pop-up');
            module.unbind.close();
            module.show();
          }
          else {
            module.debug('Popup is visible, hiding pop-up');
            module.hide();
          }
        },

        show: function(callback) {
          callback = callback || function(){};
          module.debug('Showing pop-up', settings.transition);
          if(module.is.hidden() && !( module.is.active() && module.is.dropdown()) ) {
            if( !module.exists() ) {
              module.create();
            }
            if(settings.onShow.call($popup, element) === false) {
              module.debug('onShow callback returned false, cancelling popup animation');
              return;
            }
            else if(!settings.preserve && !settings.popup) {
              module.refresh();
            }
            if( $popup && module.set.position() ) {
              module.save.conditions();
              if(settings.exclusive) {
                module.hideAll();
              }
              module.animate.show(callback);
            }
          }
        },


        hide: function(callback) {
          callback = callback || function(){};
          if( module.is.visible() || module.is.animating() ) {
            if(settings.onHide.call($popup, element) === false) {
              module.debug('onHide callback returned false, cancelling popup animation');
              return;
            }
            module.remove.visible();
            module.unbind.close();
            module.restore.conditions();
            module.animate.hide(callback);
          }
        },

        hideAll: function() {
          $(selector.popup)
            .filter('.' + className.popupVisible)
            .each(function() {
              $(this)
                .data(metadata.activator)
                  .popup('hide')
              ;
            })
          ;
        },
        exists: function() {
          if(!$popup) {
            return false;
          }
          if(settings.inline || settings.popup) {
            return ( module.has.popup() );
          }
          else {
            return ( $popup.closest($context).length >= 1 )
              ? true
              : false
            ;
          }
        },

        removePopup: function() {
          if( module.has.popup() && !settings.popup) {
            module.debug('Removing popup', $popup);
            $popup.remove();
            $popup = undefined;
            settings.onRemove.call($popup, element);
          }
        },

        save: {
          conditions: function() {
            module.cache = {
              title: $module.attr('title')
            };
            if (module.cache.title) {
              $module.removeAttr('title');
            }
            module.verbose('Saving original attributes', module.cache.title);
          }
        },
        restore: {
          conditions: function() {
            if(module.cache && module.cache.title) {
              $module.attr('title', module.cache.title);
              module.verbose('Restoring original attributes', module.cache.title);
            }
            return true;
          }
        },
        supports: {
          svg: function() {
            return (typeof SVGGraphicsElement !== 'undefined');
          }
        },
        animate: {
          show: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
              module.set.visible();
              $popup
                .transition({
                  animation  : settings.transition + ' in',
                  queue      : false,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  duration   : settings.duration,
                  onComplete : function() {
                    module.bind.close();
                    callback.call($popup, element);
                    settings.onVisible.call($popup, element);
                  }
                })
              ;
            }
            else {
              module.error(error.noTransition);
            }
          },
          hide: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            module.debug('Hiding pop-up');
            if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
              $popup
                .transition({
                  animation  : settings.transition + ' out',
                  queue      : false,
                  duration   : settings.duration,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  onComplete : function() {
                    module.reset();
                    callback.call($popup, element);
                    settings.onHidden.call($popup, element);
                  }
                })
              ;
            }
            else {
              module.error(error.noTransition);
            }
          }
        },

        change: {
          content: function(html) {
            $popup.html(html);
          }
        },

        get: {
          html: function() {
            $module.removeData(metadata.html);
            return $module.data(metadata.html) || settings.html;
          },
          title: function() {
            $module.removeData(metadata.title);
            return $module.data(metadata.title) || settings.title;
          },
          content: function() {
            $module.removeData(metadata.content);
            return $module.data(metadata.content) || settings.content || $module.attr('title');
          },
          variation: function() {
            $module.removeData(metadata.variation);
            return $module.data(metadata.variation) || settings.variation;
          },
          popup: function() {
            return $popup;
          },
          popupOffset: function() {
            return $popup.offset();
          },
          calculations: function() {
            var
              $popupOffsetParent = module.get.offsetParent($popup),
              targetElement      = $target[0],
              isWindow           = ($boundary[0] == window),
              targetPosition     = (settings.inline || (settings.popup && settings.movePopup))
                ? $target.position()
                : $target.offset(),
              screenPosition = (isWindow)
                ? { top: 0, left: 0 }
                : $boundary.offset(),
              calculations   = {},
              scroll = (isWindow)
                ? { top: $window.scrollTop(), left: $window.scrollLeft() }
                : { top: 0, left: 0},
              screen
            ;
            calculations = {
              // element which is launching popup
              target : {
                element : $target[0],
                width   : $target.outerWidth(),
                height  : $target.outerHeight(),
                top     : targetPosition.top,
                left    : targetPosition.left,
                margin  : {}
              },
              // popup itself
              popup : {
                width  : $popup.outerWidth(),
                height : $popup.outerHeight()
              },
              // offset container (or 3d context)
              parent : {
                width  : $offsetParent.outerWidth(),
                height : $offsetParent.outerHeight()
              },
              // screen boundaries
              screen : {
                top  : screenPosition.top,
                left : screenPosition.left,
                scroll: {
                  top  : scroll.top,
                  left : scroll.left
                },
                width  : $boundary.width(),
                height : $boundary.height()
              }
            };

            // if popup offset context is not same as target, then adjust calculations
            if($popupOffsetParent.get(0) !== $offsetParent.get(0)) {
              var
                popupOffset        = $popupOffsetParent.offset()
              ;
              calculations.target.top -= popupOffset.top;
              calculations.target.left -= popupOffset.left;
              calculations.parent.width = $popupOffsetParent.outerWidth();
              calculations.parent.height = $popupOffsetParent.outerHeight();
            }

            // add in container calcs if fluid
            if( settings.setFluidWidth && module.is.fluid() ) {
              calculations.container = {
                width: $popup.parent().outerWidth()
              };
              calculations.popup.width = calculations.container.width;
            }

            // add in margins if inline
            calculations.target.margin.top = (settings.inline)
              ? parseInt( window.getComputedStyle(targetElement).getPropertyValue('margin-top'), 10)
              : 0
            ;
            calculations.target.margin.left = (settings.inline)
              ? module.is.rtl()
                ? parseInt( window.getComputedStyle(targetElement).getPropertyValue('margin-right'), 10)
                : parseInt( window.getComputedStyle(targetElement).getPropertyValue('margin-left'), 10)
              : 0
            ;
            // calculate screen boundaries
            screen = calculations.screen;
            calculations.boundary = {
              top    : screen.top + screen.scroll.top,
              bottom : screen.top + screen.scroll.top + screen.height,
              left   : screen.left + screen.scroll.left,
              right  : screen.left + screen.scroll.left + screen.width
            };
            return calculations;
          },
          id: function() {
            return id;
          },
          startEvent: function() {
            if(settings.on == 'hover') {
              return 'mouseenter';
            }
            else if(settings.on == 'focus') {
              return 'focus';
            }
            return false;
          },
          scrollEvent: function() {
            return 'scroll';
          },
          endEvent: function() {
            if(settings.on == 'hover') {
              return 'mouseleave';
            }
            else if(settings.on == 'focus') {
              return 'blur';
            }
            return false;
          },
          distanceFromBoundary: function(offset, calculations) {
            var
              distanceFromBoundary = {},
              popup,
              boundary
            ;
            calculations = calculations || module.get.calculations();

            // shorthand
            popup        = calculations.popup;
            boundary     = calculations.boundary;

            if(offset) {
              distanceFromBoundary = {
                top    : (offset.top - boundary.top),
                left   : (offset.left - boundary.left),
                right  : (boundary.right - (offset.left + popup.width) ),
                bottom : (boundary.bottom - (offset.top + popup.height) )
              };
              module.verbose('Distance from boundaries determined', offset, distanceFromBoundary);
            }
            return distanceFromBoundary;
          },
          offsetParent: function($element) {
            var
              element = ($element !== undefined)
                ? $element[0]
                : $target[0],
              parentNode = element.parentNode,
              $node    = $(parentNode)
            ;
            if(parentNode) {
              var
                is2D     = ($node.css('transform') === 'none'),
                isStatic = ($node.css('position') === 'static'),
                isBody   = $node.is('body')
              ;
              while(parentNode && !isBody && isStatic && is2D) {
                parentNode = parentNode.parentNode;
                $node    = $(parentNode);
                is2D     = ($node.css('transform') === 'none');
                isStatic = ($node.css('position') === 'static');
                isBody   = $node.is('body');
              }
            }
            return ($node && $node.length > 0)
              ? $node
              : $()
            ;
          },
          positions: function() {
            return {
              'top left'      : false,
              'top center'    : false,
              'top right'     : false,
              'bottom left'   : false,
              'bottom center' : false,
              'bottom right'  : false,
              'left center'   : false,
              'right center'  : false
            };
          },
          nextPosition: function(position) {
            var
              positions          = position.split(' '),
              verticalPosition   = positions[0],
              horizontalPosition = positions[1],
              opposite = {
                top    : 'bottom',
                bottom : 'top',
                left   : 'right',
                right  : 'left'
              },
              adjacent = {
                left   : 'center',
                center : 'right',
                right  : 'left'
              },
              backup = {
                'top left'      : 'top center',
                'top center'    : 'top right',
                'top right'     : 'right center',
                'right center'  : 'bottom right',
                'bottom right'  : 'bottom center',
                'bottom center' : 'bottom left',
                'bottom left'   : 'left center',
                'left center'   : 'top left'
              },
              adjacentsAvailable = (verticalPosition == 'top' || verticalPosition == 'bottom'),
              oppositeTried = false,
              adjacentTried = false,
              nextPosition  = false
            ;
            if(!triedPositions) {
              module.verbose('All available positions available');
              triedPositions = module.get.positions();
            }

            module.debug('Recording last position tried', position);
            triedPositions[position] = true;

            if(settings.prefer === 'opposite') {
              nextPosition  = [opposite[verticalPosition], horizontalPosition];
              nextPosition  = nextPosition.join(' ');
              oppositeTried = (triedPositions[nextPosition] === true);
              module.debug('Trying opposite strategy', nextPosition);
            }
            if((settings.prefer === 'adjacent') && adjacentsAvailable ) {
              nextPosition  = [verticalPosition, adjacent[horizontalPosition]];
              nextPosition  = nextPosition.join(' ');
              adjacentTried = (triedPositions[nextPosition] === true);
              module.debug('Trying adjacent strategy', nextPosition);
            }
            if(adjacentTried || oppositeTried) {
              module.debug('Using backup position', nextPosition);
              nextPosition = backup[position];
            }
            return nextPosition;
          }
        },

        set: {
          position: function(position, calculations) {

            // exit conditions
            if($target.length === 0 || $popup.length === 0) {
              module.error(error.notFound);
              return;
            }
            var
              offset,
              distanceAway,
              target,
              popup,
              parent,
              positioning,
              popupOffset,
              distanceFromBoundary
            ;

            calculations = calculations || module.get.calculations();
            position     = position     || $module.data(metadata.position) || settings.position;

            offset       = $module.data(metadata.offset) || settings.offset;
            distanceAway = settings.distanceAway;

            // shorthand
            target = calculations.target;
            popup  = calculations.popup;
            parent = calculations.parent;

            if(module.should.centerArrow(calculations)) {
              module.verbose('Adjusting offset to center arrow on small target element');
              if(position == 'top left' || position == 'bottom left') {
                offset += (target.width / 2);
                offset -= settings.arrowPixelsFromEdge;
              }
              if(position == 'top right' || position == 'bottom right') {
                offset -= (target.width / 2);
                offset += settings.arrowPixelsFromEdge;
              }
            }

            if(target.width === 0 && target.height === 0 && !module.is.svg(target.element)) {
              module.debug('Popup target is hidden, no action taken');
              return false;
            }

            if(settings.inline) {
              module.debug('Adding margin to calculation', target.margin);
              if(position == 'left center' || position == 'right center') {
                offset       +=  target.margin.top;
                distanceAway += -target.margin.left;
              }
              else if (position == 'top left' || position == 'top center' || position == 'top right') {
                offset       += target.margin.left;
                distanceAway -= target.margin.top;
              }
              else {
                offset       += target.margin.left;
                distanceAway += target.margin.top;
              }
            }

            module.debug('Determining popup position from calculations', position, calculations);

            if (module.is.rtl()) {
              position = position.replace(/left|right/g, function (match) {
                return (match == 'left')
                  ? 'right'
                  : 'left'
                ;
              });
              module.debug('RTL: Popup position updated', position);
            }

            // if last attempt use specified last resort position
            if(searchDepth == settings.maxSearchDepth && typeof settings.lastResort === 'string') {
              position = settings.lastResort;
            }

            switch (position) {
              case 'top left':
                positioning = {
                  top    : 'auto',
                  bottom : parent.height - target.top + distanceAway,
                  left   : target.left + offset,
                  right  : 'auto'
                };
              break;
              case 'top center':
                positioning = {
                  bottom : parent.height - target.top + distanceAway,
                  left   : target.left + (target.width / 2) - (popup.width / 2) + offset,
                  top    : 'auto',
                  right  : 'auto'
                };
              break;
              case 'top right':
                positioning = {
                  bottom :  parent.height - target.top + distanceAway,
                  right  :  parent.width - target.left - target.width - offset,
                  top    : 'auto',
                  left   : 'auto'
                };
              break;
              case 'left center':
                positioning = {
                  top    : target.top + (target.height / 2) - (popup.height / 2) + offset,
                  right  : parent.width - target.left + distanceAway,
                  left   : 'auto',
                  bottom : 'auto'
                };
              break;
              case 'right center':
                positioning = {
                  top    : target.top + (target.height / 2) - (popup.height / 2) + offset,
                  left   : target.left + target.width + distanceAway,
                  bottom : 'auto',
                  right  : 'auto'
                };
              break;
              case 'bottom left':
                positioning = {
                  top    : target.top + target.height + distanceAway,
                  left   : target.left + offset,
                  bottom : 'auto',
                  right  : 'auto'
                };
              break;
              case 'bottom center':
                positioning = {
                  top    : target.top + target.height + distanceAway,
                  left   : target.left + (target.width / 2) - (popup.width / 2) + offset,
                  bottom : 'auto',
                  right  : 'auto'
                };
              break;
              case 'bottom right':
                positioning = {
                  top    : target.top + target.height + distanceAway,
                  right  : parent.width - target.left  - target.width - offset,
                  left   : 'auto',
                  bottom : 'auto'
                };
              break;
            }
            if(positioning === undefined) {
              module.error(error.invalidPosition, position);
            }

            module.debug('Calculated popup positioning values', positioning);

            // tentatively place on stage
            $popup
              .css(positioning)
              .removeClass(className.position)
              .addClass(position)
              .addClass(className.loading)
            ;

            popupOffset = module.get.popupOffset();

            // see if any boundaries are surpassed with this tentative position
            distanceFromBoundary = module.get.distanceFromBoundary(popupOffset, calculations);

            if(!settings.forcePosition && module.is.offstage(distanceFromBoundary, position) ) {
              module.debug('Position is outside viewport', position);
              if(searchDepth < settings.maxSearchDepth) {
                searchDepth++;
                position = module.get.nextPosition(position);
                module.debug('Trying new position', position);
                return ($popup)
                  ? module.set.position(position, calculations)
                  : false
                ;
              }
              else {
                if(settings.lastResort) {
                  module.debug('No position found, showing with last position');
                }
                else {
                  module.debug('Popup could not find a position to display', $popup);
                  module.error(error.cannotPlace, element);
                  module.remove.attempts();
                  module.remove.loading();
                  module.reset();
                  settings.onUnplaceable.call($popup, element);
                  return false;
                }
              }
            }
            module.debug('Position is on stage', position);
            module.remove.attempts();
            module.remove.loading();
            if( settings.setFluidWidth && module.is.fluid() ) {
              module.set.fluidWidth(calculations);
            }
            return true;
          },

          fluidWidth: function(calculations) {
            calculations = calculations || module.get.calculations();
            module.debug('Automatically setting element width to parent width', calculations.parent.width);
            $popup.css('width', calculations.container.width);
          },

          variation: function(variation) {
            variation = variation || module.get.variation();
            if(variation && module.has.popup() ) {
              module.verbose('Adding variation to popup', variation);
              $popup.addClass(variation);
            }
          },

          visible: function() {
            $module.addClass(className.visible);
          }
        },

        remove: {
          loading: function() {
            $popup.removeClass(className.loading);
          },
          variation: function(variation) {
            variation = variation || module.get.variation();
            if(variation) {
              module.verbose('Removing variation', variation);
              $popup.removeClass(variation);
            }
          },
          visible: function() {
            $module.removeClass(className.visible);
          },
          attempts: function() {
            module.verbose('Resetting all searched positions');
            searchDepth    = 0;
            triedPositions = false;
          }
        },

        bind: {
          events: function() {
            module.debug('Binding popup events to module');
            if(settings.on == 'click') {
              $module
                .on(clickEvent + eventNamespace, module.toggle)
              ;
            }
            if(settings.on == 'hover') {
              $module
                .on('touchstart' + eventNamespace, module.event.touchstart)
              ;
            }
            if( module.get.startEvent() ) {
              $module
                .on(module.get.startEvent() + eventNamespace, module.event.start)
                .on(module.get.endEvent() + eventNamespace, module.event.end)
              ;
            }
            if(settings.target) {
              module.debug('Target set to element', $target);
            }
            $window.on('resize' + elementNamespace, module.event.resize);
          },
          popup: function() {
            module.verbose('Allowing hover events on popup to prevent closing');
            if( $popup && module.has.popup() ) {
              $popup
                .on('mouseenter' + eventNamespace, module.event.start)
                .on('mouseleave' + eventNamespace, module.event.end)
              ;
            }
          },
          close: function() {
            if(settings.hideOnScroll === true || (settings.hideOnScroll == 'auto' && settings.on != 'click')) {
              module.bind.closeOnScroll();
            }
            if(module.is.closable()) {
              module.bind.clickaway();
            }
            else if(settings.on == 'hover' && openedWithTouch) {
              module.bind.touchClose();
            }
          },
          closeOnScroll: function() {
            module.verbose('Binding scroll close event to document');
            $scrollContext
              .one(module.get.scrollEvent() + elementNamespace, module.event.hideGracefully)
            ;
          },
          touchClose: function() {
            module.verbose('Binding popup touchclose event to document');
            $document
              .on('touchstart' + elementNamespace, function(event) {
                module.verbose('Touched away from popup');
                module.event.hideGracefully.call(element, event);
              })
            ;
          },
          clickaway: function() {
            module.verbose('Binding popup close event to document');
            $document
              .on(clickEvent + elementNamespace, function(event) {
                module.verbose('Clicked away from popup');
                module.event.hideGracefully.call(element, event);
              })
            ;
          }
        },

        unbind: {
          events: function() {
            $window
              .off(elementNamespace)
            ;
            $module
              .off(eventNamespace)
            ;
          },
          close: function() {
            $document
              .off(elementNamespace)
            ;
            $scrollContext
              .off(elementNamespace)
            ;
          },
        },

        has: {
          popup: function() {
            return ($popup && $popup.length > 0);
          }
        },

        should: {
          centerArrow: function(calculations) {
            return !module.is.basic() && calculations.target.width <= (settings.arrowPixelsFromEdge * 2);
          },
        },

        is: {
          closable: function() {
            if(settings.closable == 'auto') {
              if(settings.on == 'hover') {
                return false;
              }
              return true;
            }
            return settings.closable;
          },
          offstage: function(distanceFromBoundary, position) {
            var
              offstage = []
            ;
            // return boundaries that have been surpassed
            $.each(distanceFromBoundary, function(direction, distance) {
              if(distance < -settings.jitter) {
                module.debug('Position exceeds allowable distance from edge', direction, distance, position);
                offstage.push(direction);
              }
            });
            if(offstage.length > 0) {
              return true;
            }
            else {
              return false;
            }
          },
          svg: function(element) {
            return module.supports.svg() && (element instanceof SVGGraphicsElement);
          },
          basic: function() {
            return $module.hasClass(className.basic);
          },
          active: function() {
            return $module.hasClass(className.active);
          },
          animating: function() {
            return ($popup !== undefined && $popup.hasClass(className.animating) );
          },
          fluid: function() {
            return ($popup !== undefined && $popup.hasClass(className.fluid));
          },
          visible: function() {
            return ($popup !== undefined && $popup.hasClass(className.popupVisible));
          },
          dropdown: function() {
            return $module.hasClass(className.dropdown);
          },
          hidden: function() {
            return !module.is.visible();
          },
          rtl: function () {
            return $module.attr('dir') === 'rtl' || $module.css('direction') === 'rtl';
          }
        },

        reset: function() {
          module.remove.visible();
          if(settings.preserve) {
            if($.fn.transition !== undefined) {
              $popup
                .transition('remove transition')
              ;
            }
          }
          else {
            module.removePopup();
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
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.popup.settings = {

  name           : 'Popup',

  // module settings
  silent         : false,
  debug          : false,
  verbose        : false,
  performance    : true,
  namespace      : 'popup',

  // whether it should use dom mutation observers
  observeChanges : true,

  // callback only when element added to dom
  onCreate       : function(){},

  // callback before element removed from dom
  onRemove       : function(){},

  // callback before show animation
  onShow         : function(){},

  // callback after show animation
  onVisible      : function(){},

  // callback before hide animation
  onHide         : function(){},

  // callback when popup cannot be positioned in visible screen
  onUnplaceable  : function(){},

  // callback after hide animation
  onHidden       : function(){},

  // when to show popup
  on             : 'hover',

  // element to use to determine if popup is out of boundary
  boundary       : window,

  // whether to add touchstart events when using hover
  addTouchEvents : true,

  // default position relative to element
  position       : 'top left',

  // if given position should be used regardless if popup fits
  forcePosition  : false,

  // name of variation to use
  variation      : '',

  // whether popup should be moved to context
  movePopup      : true,

  // element which popup should be relative to
  target         : false,

  // jq selector or element that should be used as popup
  popup          : false,

  // popup should remain inline next to activator
  inline         : false,

  // popup should be removed from page on hide
  preserve       : false,

  // popup should not close when being hovered on
  hoverable      : false,

  // explicitly set content
  content        : false,

  // explicitly set html
  html           : false,

  // explicitly set title
  title          : false,

  // whether automatically close on clickaway when on click
  closable       : true,

  // automatically hide on scroll
  hideOnScroll   : 'auto',

  // hide other popups on show
  exclusive      : false,

  // context to attach popups
  context        : 'body',

  // context for binding scroll events
  scrollContext  : window,

  // position to prefer when calculating new position
  prefer         : 'opposite',

  // specify position to appear even if it doesn't fit
  lastResort     : false,

  // number of pixels from edge of popup to pointing arrow center (used from centering)
  arrowPixelsFromEdge: 20,

  // delay used to prevent accidental refiring of animations due to user error
  delay : {
    show : 50,
    hide : 70
  },

  // whether fluid variation should assign width explicitly
  setFluidWidth  : true,

  // transition settings
  duration       : 200,
  transition     : 'scale',

  // distance away from activating element in px
  distanceAway   : 0,

  // number of pixels an element is allowed to be "offstage" for a position to be chosen (allows for rounding)
  jitter         : 2,

  // offset on aligning axis from calculated position
  offset         : 0,

  // maximum times to look for a position before failing (9 positions total)
  maxSearchDepth : 15,

  error: {
    invalidPosition : 'The position you specified is not a valid position',
    cannotPlace     : 'Popup does not fit within the boundaries of the viewport',
    method          : 'The method you called is not defined.',
    noTransition    : 'This module requires ui transitions <https://github.com/Semantic-Org/UI-Transition>',
    notFound        : 'The target or popup you specified does not exist on the page'
  },

  metadata: {
    activator : 'activator',
    content   : 'content',
    html      : 'html',
    offset    : 'offset',
    position  : 'position',
    title     : 'title',
    variation : 'variation'
  },

  className   : {
    active       : 'active',
    basic        : 'basic',
    animating    : 'animating',
    dropdown     : 'dropdown',
    fluid        : 'fluid',
    loading      : 'loading',
    popup        : 'ui popup',
    position     : 'top left center bottom right',
    visible      : 'visible',
    popupVisible : 'visible'
  },

  selector    : {
    popup    : '.ui.popup'
  },

  templates: {
    escape: function(string) {
      var
        badChars     = /[&<>"'`]/g,
        shouldEscape = /[&<>"'`]/,
        escape       = {
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#x27;",
          "`": "&#x60;"
        },
        escapedChar  = function(chr) {
          return escape[chr];
        }
      ;
      if(shouldEscape.test(string)) {
        return string.replace(badChars, escapedChar);
      }
      return string;
    },
    popup: function(text) {
      var
        html   = '',
        escape = $.fn.popup.settings.templates.escape
      ;
      if(typeof text !== undefined) {
        if(typeof text.title !== undefined && text.title) {
          text.title = escape(text.title);
          html += '<div class="header">' + text.title + '</div>';
        }
        if(typeof text.content !== undefined && text.content) {
          text.content = escape(text.content);
          html += '<div class="content">' + text.content + '</div>';
        }
      }
      return html;
    }
  }

};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Progress
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

$.fn.progress = function(parameters) {
  var
    $allModules    = $(this),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.progress.settings, parameters)
          : $.extend({}, $.fn.progress.settings),

        className       = settings.className,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        selector        = settings.selector,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $bars           = $(this).find(selector.bar),
        $progresses     = $(this).find(selector.progress),
        $label          = $(this).find(selector.label),

        element         = this,
        instance        = $module.data(moduleNamespace),

        animating = false,
        transitionEnd,
        module
      ;
      module = {
        helper: {
          sum: function (nums) {
            return Array.isArray(nums) ? nums.reduce(function (left, right) {
              return left + Number(right);
            }, 0) : 0;
          },
          /**
           * Derive precision for multiple progress with total and values.
           *
           * This helper dervices a precision that is sufficiently large to show minimum value of multiple progress.
           *
           * Example1
           * - total: 1122
           * - values: [325, 111, 74, 612]
           * - min ratio: 74/1122 = 0.0659...
           * - required precision:  100
           *
           * Example2
           * - total: 10541
           * - values: [3235, 1111, 74, 6121]
           * - min ratio: 74/10541 = 0.0070...
           * - required precision:   1000
           *
           * @param min A minimum value within multiple values
           * @param total A total amount of multiple values
           * @returns {number} A precison. Could be 1, 10, 100, ... 1e+10.
           */
          derivePrecision: function(min, total) {
            var precisionPower = 0
            var precision = 1;
            var ratio = min / total;
            while (precisionPower < 10) {
              ratio = ratio * precision;
              if (ratio > 1) {
                break;
              }
              precision = Math.pow(10, precisionPower++);
            }
            return precision;
          },
          forceArray: function (element) {
            return Array.isArray(element)
              ? element
              : !isNaN(element)
                ? [element]
                : typeof element == 'string'
                  ? element.split(',')
                  : []
              ;
          }
        },

        initialize: function() {
          module.set.duration();
          module.set.transitionEvent();
          module.debug(element);

          module.read.metadata();
          module.read.settings();

          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of progress', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },
        destroy: function() {
          module.verbose('Destroying previous progress for', $module);
          clearInterval(instance.interval);
          module.remove.state();
          $module.removeData(moduleNamespace);
          instance = undefined;
        },

        reset: function() {
          module.remove.nextValue();
          module.update.progress(0);
        },

        complete: function(keepState) {
          if(module.percent === undefined || module.percent < 100) {
            module.remove.progressPoll();
            if(keepState !== true){
                module.set.percent(100);
            }
          }
        },

        read: {
          metadata: function() {
            var
              data = {
                percent : module.helper.forceArray($module.data(metadata.percent)),
                total   : $module.data(metadata.total),
                value   : module.helper.forceArray($module.data(metadata.value))
              }
            ;
            if(data.total) {
              module.debug('Total value set from metadata', data.total);
              module.set.total(data.total);
            }
            if(data.value.length > 0) {
              module.debug('Current value set from metadata', data.value);
              module.set.value(data.value);
              module.set.progress(data.value);
            }
            if(data.percent.length > 0) {
              module.debug('Current percent value set from metadata', data.percent);
              module.set.percent(data.percent);
            }
          },
          settings: function() {
            if(settings.total !== false) {
              module.debug('Current total set in settings', settings.total);
              module.set.total(settings.total);
            }
            if(settings.value !== false) {
              module.debug('Current value set in settings', settings.value);
              module.set.value(settings.value);
              module.set.progress(module.value);
            }
            if(settings.percent !== false) {
              module.debug('Current percent set in settings', settings.percent);
              module.set.percent(settings.percent);
            }
          }
        },

        bind: {
          transitionEnd: function(callback) {
            var
              transitionEnd = module.get.transitionEnd()
            ;
            $bars
              .one(transitionEnd + eventNamespace, function(event) {
                clearTimeout(module.failSafeTimer);
                callback.call(this, event);
              })
            ;
            module.failSafeTimer = setTimeout(function() {
              $bars.triggerHandler(transitionEnd);
            }, settings.duration + settings.failSafeDelay);
            module.verbose('Adding fail safe timer', module.timer);
          }
        },

        increment: function(incrementValue) {
          var
            startValue,
            newValue
          ;
          if( module.has.total() ) {
            startValue     = module.get.value();
            incrementValue = incrementValue || 1;
          }
          else {
            startValue     = module.get.percent();
            incrementValue = incrementValue || module.get.randomValue();
          }
          newValue = startValue + incrementValue;
          module.debug('Incrementing percentage by', startValue, newValue, incrementValue);
          newValue = module.get.normalizedValue(newValue);
          module.set.progress(newValue);
        },
        decrement: function(decrementValue) {
          var
            total     = module.get.total(),
            startValue,
            newValue
          ;
          if(total) {
            startValue     =  module.get.value();
            decrementValue =  decrementValue || 1;
            newValue       =  startValue - decrementValue;
            module.debug('Decrementing value by', decrementValue, startValue);
          }
          else {
            startValue     =  module.get.percent();
            decrementValue =  decrementValue || module.get.randomValue();
            newValue       =  startValue - decrementValue;
            module.debug('Decrementing percentage by', decrementValue, startValue);
          }
          newValue = module.get.normalizedValue(newValue);
          module.set.progress(newValue);
        },

        has: {
          progressPoll: function() {
            return module.progressPoll;
          },
          total: function() {
            return (module.get.total() !== false);
          }
        },

        get: {
          text: function(templateText, index) {
            var
              index_  = index || 0,
              value   = module.get.value(index_),
              total   = module.total || 0,
              percent = (animating)
                ? module.get.displayPercent(index_)
                : module.get.percent(index_),
              left = (module.total > 0)
                ? (total - value)
                : (100 - percent)
            ;
            templateText = templateText || '';
            templateText = templateText
              .replace('{value}', value)
              .replace('{total}', total)
              .replace('{left}', left)
              .replace('{percent}', percent)
              .replace('{bar}', settings.text.bars[index_] || '')
            ;
            module.verbose('Adding variables to progress bar text', templateText);
            return templateText;
          },

          normalizedValue: function(value) {
            if(value < 0) {
              module.debug('Value cannot decrement below 0');
              return 0;
            }
            if(module.has.total()) {
              if(value > module.total) {
                module.debug('Value cannot increment above total', module.total);
                return module.total;
              }
            }
            else if(value > 100 ) {
              module.debug('Value cannot increment above 100 percent');
              return 100;
            }
            return value;
          },

          updateInterval: function() {
            if(settings.updateInterval == 'auto') {
              return settings.duration;
            }
            return settings.updateInterval;
          },

          randomValue: function() {
            module.debug('Generating random increment percentage');
            return Math.floor((Math.random() * settings.random.max) + settings.random.min);
          },

          numericValue: function(value) {
            return (typeof value === 'string')
              ? (value.replace(/[^\d.]/g, '') !== '')
                ? +(value.replace(/[^\d.]/g, ''))
                : false
              : value
            ;
          },

          transitionEnd: function() {
            var
              element     = document.createElement('element'),
              transitions = {
                'transition'       :'transitionend',
                'OTransition'      :'oTransitionEnd',
                'MozTransition'    :'transitionend',
                'WebkitTransition' :'webkitTransitionEnd'
              },
              transition
            ;
            for(transition in transitions){
              if( element.style[transition] !== undefined ){
                return transitions[transition];
              }
            }
          },

          // gets current displayed percentage (if animating values this is the intermediary value)
          displayPercent: function(index) {
            var
              $bar           = $($bars[index]),
              barWidth       = $bar.width(),
              totalWidth     = $module.width(),
              minDisplay     = parseInt($bar.css('min-width'), 10),
              displayPercent = (barWidth > minDisplay)
                ? (barWidth / totalWidth * 100)
                : module.percent
            ;
            return (settings.precision > 0)
              ? Math.round(displayPercent * (10 * settings.precision)) / (10 * settings.precision)
              : Math.round(displayPercent)
              ;
          },

          percent: function(index) {
            return module.percent && module.percent[index || 0] || 0;
          },
          value: function(index) {
            return module.nextValue || module.value && module.value[index || 0] || 0;
          },
          total: function() {
            return module.total || false;
          }
        },

        create: {
          progressPoll: function() {
            module.progressPoll = setTimeout(function() {
              module.update.toNextValue();
              module.remove.progressPoll();
            }, module.get.updateInterval());
          },
        },

        is: {
          complete: function() {
            return module.is.success() || module.is.warning() || module.is.error();
          },
          success: function() {
            return $module.hasClass(className.success);
          },
          warning: function() {
            return $module.hasClass(className.warning);
          },
          error: function() {
            return $module.hasClass(className.error);
          },
          active: function() {
            return $module.hasClass(className.active);
          },
          visible: function() {
            return $module.is(':visible');
          }
        },

        remove: {
          progressPoll: function() {
            module.verbose('Removing progress poll timer');
            if(module.progressPoll) {
              clearTimeout(module.progressPoll);
              delete module.progressPoll;
            }
          },
          nextValue: function() {
            module.verbose('Removing progress value stored for next update');
            delete module.nextValue;
          },
          state: function() {
            module.verbose('Removing stored state');
            delete module.total;
            delete module.percent;
            delete module.value;
          },
          active: function() {
            module.verbose('Removing active state');
            $module.removeClass(className.active);
          },
          success: function() {
            module.verbose('Removing success state');
            $module.removeClass(className.success);
          },
          warning: function() {
            module.verbose('Removing warning state');
            $module.removeClass(className.warning);
          },
          error: function() {
            module.verbose('Removing error state');
            $module.removeClass(className.error);
          }
        },

        set: {
          barWidth: function(values) {
            module.debug("set bar width with ", values);
            values = module.helper.forceArray(values);
            var firstNonZeroIndex = -1;
            var lastNonZeroIndex = -1;
            var valuesSum = module.helper.sum(values);
            var barCounts = $bars.length;
            var isMultiple = barCounts > 1;
            var percents = values.map(function(value, index) {
              var allZero = (index === barCounts - 1 && valuesSum === 0);
              var $bar = $($bars[index]);
              if (value === 0 && isMultiple && !allZero) {
                $bar.css('display', 'none');
              } else {
                if (isMultiple && allZero) {
                  $bar.css('background', 'transparent');
                }
                if (firstNonZeroIndex == -1) {
                  firstNonZeroIndex = index;
                }
                lastNonZeroIndex = index;
                $bar.css({
                  display: 'block',
                  width: value + '%'
                });
              }
              return parseFloat(value);
            });
            values.forEach(function(_, index) {
              var $bar = $($bars[index]);
              $bar.css({
                borderTopLeftRadius: index == firstNonZeroIndex ? '' : 0,
                borderBottomLeftRadius: index == firstNonZeroIndex ? '' : 0,
                borderTopRightRadius: index == lastNonZeroIndex ? '' : 0,
                borderBottomRightRadius: index == lastNonZeroIndex ? '' : 0
              });
            });
            $module
              .attr('data-percent', percents)
            ;
          },
          duration: function(duration) {
            duration = duration || settings.duration;
            duration = (typeof duration == 'number')
              ? duration + 'ms'
              : duration
            ;
            module.verbose('Setting progress bar transition duration', duration);
            $bars
              .css({
                'transition-duration':  duration
              })
            ;
          },
          percent: function(percents) {
            percents = module.helper.forceArray(percents).map(function(percent) {
              return (typeof percent == 'string')
                ? +(percent.replace('%', ''))
                : percent
                ;
            });
            var hasTotal = module.has.total();
            var totalPecent = module.helper.sum(percents);
            var isMultpleValues = percents.length > 1 && hasTotal;
            var sumTotal = module.helper.sum(module.helper.forceArray(module.value));
            if (isMultpleValues && sumTotal > module.total) {
              // Sum values instead of pecents to avoid precision issues when summing floats
              module.error(error.sumExceedsTotal, sumTotal, module.total);
            } else if (!isMultpleValues && totalPecent > 100) {
              // Sum before rouding since sum of rounded may have error though sum of actual is fine
              module.error(error.tooHigh, totalPecent);
            } else if (totalPecent < 0) {
              module.error(error.tooLow, totalPecent);
            } else {
              var autoPrecision = settings.precision > 0
                ? settings.precision
                : isMultpleValues
                  ? module.helper.derivePrecision(Math.min.apply(null, module.value), module.total)
                  : undefined;

              // round display percentage
              var roundedPercents = percents.map(function (percent) {
                return (autoPrecision > 0)
                  ? Math.round(percent * (10 * autoPrecision)) / (10 * autoPrecision)
                  : Math.round(percent)
                  ;
              });
              module.percent = roundedPercents;
              if (!hasTotal) {
                module.value = roundedPercents.map(function (percent) {
                  return (autoPrecision > 0)
                    ? Math.round((percent / 100) * module.total * (10 * autoPrecision)) / (10 * autoPrecision)
                    : Math.round((percent / 100) * module.total * 10) / 10
                    ;
                });
                if (settings.limitValues) {
                  module.value = module.value.map(function (value) {
                    return (value > 100)
                      ? 100
                      : (module.value < 0)
                        ? 0
                        : module.value;
                  });
                }
              }
              module.set.barWidth(percents);
              module.set.labelInterval();
              module.set.labels();
            }
            settings.onChange.call(element, percents, module.value, module.total);
          },
          labelInterval: function() {
            var
              animationCallback = function() {
                module.verbose('Bar finished animating, removing continuous label updates');
                clearInterval(module.interval);
                animating = false;
                module.set.labels();
              }
            ;
            clearInterval(module.interval);
            module.bind.transitionEnd(animationCallback);
            animating = true;
            module.interval = setInterval(function() {
              var
                isInDOM = $.contains(document.documentElement, element)
              ;
              if(!isInDOM) {
                clearInterval(module.interval);
                animating = false;
              }
              module.set.labels();
            }, settings.framerate);
          },
          labels: function() {
            module.verbose('Setting both bar progress and outer label text');
            module.set.barLabel();
            module.set.state();
          },
          label: function(text) {
            text = text || '';
            if(text) {
              text = module.get.text(text);
              module.verbose('Setting label to text', text);
              $label.text(text);
            }
          },
          state: function(percent) {
            percent = (percent !== undefined)
              ? percent
              : module.helper.sum(module.percent)
            ;
            if(percent === 100) {
              if(settings.autoSuccess && $bars.length === 1 && !(module.is.warning() || module.is.error() || module.is.success())) {
                module.set.success();
                module.debug('Automatically triggering success at 100%');
              }
              else {
                module.verbose('Reached 100% removing active state');
                module.remove.active();
                module.remove.progressPoll();
              }
            }
            else if(percent > 0) {
              module.verbose('Adjusting active progress bar label', percent);
              module.set.active();
            }
            else {
              module.remove.active();
              module.set.label(settings.text.active);
            }
          },
          barLabel: function(text) {
            $progresses.map(function(index, element){
              var $progress = $(element);
              if (text !== undefined) {
                $progress.text( module.get.text(text, index) );
              }
              else if (settings.label == 'ratio' && module.total) {
                module.verbose('Adding ratio to bar label');
                $progress.text( module.get.text(settings.text.ratio, index) );
              }
              else if (settings.label == 'percent') {
                module.verbose('Adding percentage to bar label');
                $progress.text( module.get.text(settings.text.percent, index) );
              }
            });
          },
          active: function(text) {
            text = text || settings.text.active;
            module.debug('Setting active state');
            if(settings.showActivity && !module.is.active() ) {
              $module.addClass(className.active);
            }
            module.remove.warning();
            module.remove.error();
            module.remove.success();
            text = settings.onLabelUpdate('active', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onActive.call(element, module.value, module.total);
            });
          },
          success : function(text, keepState) {
            text = text || settings.text.success || settings.text.active;
            module.debug('Setting success state');
            $module.addClass(className.success);
            module.remove.active();
            module.remove.warning();
            module.remove.error();
            module.complete(keepState);
            if(settings.text.success) {
              text = settings.onLabelUpdate('success', text, module.value, module.total);
              module.set.label(text);
            }
            else {
              text = settings.onLabelUpdate('active', text, module.value, module.total);
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onSuccess.call(element, module.total);
            });
          },
          warning : function(text, keepState) {
            text = text || settings.text.warning;
            module.debug('Setting warning state');
            $module.addClass(className.warning);
            module.remove.active();
            module.remove.success();
            module.remove.error();
            module.complete(keepState);
            text = settings.onLabelUpdate('warning', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onWarning.call(element, module.value, module.total);
            });
          },
          error : function(text, keepState) {
            text = text || settings.text.error;
            module.debug('Setting error state');
            $module.addClass(className.error);
            module.remove.active();
            module.remove.success();
            module.remove.warning();
            module.complete(keepState);
            text = settings.onLabelUpdate('error', text, module.value, module.total);
            if(text) {
              module.set.label(text);
            }
            module.bind.transitionEnd(function() {
              settings.onError.call(element, module.value, module.total);
            });
          },
          transitionEvent: function() {
            transitionEnd = module.get.transitionEnd();
          },
          total: function(totalValue) {
            module.total = totalValue;
          },
          value: function(value) {
            module.value = module.helper.forceArray(value);
          },
          progress: function(value) {
            if(!module.has.progressPoll()) {
              module.debug('First update in progress update interval, immediately updating', value);
              module.update.progress(value);
              module.create.progressPoll();
            }
            else {
              module.debug('Updated within interval, setting next update to use new value', value);
              module.set.nextValue(value);
            }
          },
          nextValue: function(value) {
            module.nextValue = value;
          }
        },

        update: {
          toNextValue: function() {
            var
              nextValue = module.nextValue
            ;
            if(nextValue) {
              module.debug('Update interval complete using last updated value', nextValue);
              module.update.progress(nextValue);
              module.remove.nextValue();
            }
          },
          progress: function(values) {
            var hasTotal = module.has.total();
            if (hasTotal) {
              module.set.value(values);
            }
            var percentCompletes = module.helper.forceArray(values).map(function(value) {
              var
                percentComplete
              ;
              value = module.get.numericValue(value);
              if (value === false) {
                module.error(error.nonNumeric, value);
              }
              value = module.get.normalizedValue(value);
              if (hasTotal) {
                percentComplete = (value / module.total) * 100;
                module.debug('Calculating percent complete from total', percentComplete);
              }
              else {
                percentComplete = value;
                module.debug('Setting value to exact percentage value', percentComplete);
              }
              return percentComplete;
            });
            module.set.percent( percentCompletes );
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.progress.settings = {

  name         : 'Progress',
  namespace    : 'progress',

  silent       : false,
  debug        : false,
  verbose      : false,
  performance  : true,

  random       : {
    min : 2,
    max : 5
  },

  duration       : 300,

  updateInterval : 'auto',

  autoSuccess    : true,
  showActivity   : true,
  limitValues    : true,

  label          : 'percent',
  precision      : 0,
  framerate      : (1000 / 30), /// 30 fps

  percent        : false,
  total          : false,
  value          : false,

  // delay in ms for fail safe animation callback
  failSafeDelay : 100,

  onLabelUpdate : function(state, text, value, total){
    return text;
  },
  onChange      : function(percent, value, total){},
  onSuccess     : function(total){},
  onActive      : function(value, total){},
  onError       : function(value, total){},
  onWarning     : function(value, total){},

  error    : {
    method          : 'The method you called is not defined.',
    nonNumeric      : 'Progress value is non numeric',
    tooHigh         : 'Value specified is above 100%',
    tooLow          : 'Value specified is below 0%',
    sumExceedsTotal : 'Sum of multple values exceed total',
  },

  regExp: {
    variable: /\{\$*[A-z0-9]+\}/g
  },

  metadata: {
    percent : 'percent',
    total   : 'total',
    value   : 'value'
  },

  selector : {
    bar      : '> .bar',
    label    : '> .label',
    progress : '.bar > .progress'
  },

  text : {
    active  : false,
    error   : false,
    success : false,
    warning : false,
    percent : '{percent}%',
    ratio   : '{value} of {total}',
    bars    : ['']
  },

  className : {
    active  : 'active',
    error   : 'error',
    success : 'success',
    warning : 'warning'
  }

};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Slider
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ( $, window, document, undefined ) {

"use strict";

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.slider = function(parameters) {

  var
    $allModules    = $(this),
    $window        = $(window),

    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    alphabet       = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z'],

    SINGLE_STEP     = 1,
    BIG_STEP        = 2,
    NO_STEP         = 0,
    SINGLE_BACKSTEP = -1,
    BIG_BACKSTEP    = -2,

    // Used to manage document bound events.
    // Use this so that we can distinguish between which document events are bound to which range.
    currentRange    = 0,

    returnedValue
  ;

  $allModules
    .each(function() {

      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.slider.settings, parameters)
          : $.extend({}, $.fn.slider.settings),

        className       = settings.className,
        metadata        = settings.metadata,
        namespace       = settings.namespace,
        error           = settings.error,
        keys            = settings.keys,
        interpretLabel  = settings.interpretLabel,

        isHover         = false,
        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $currThumb,
        $thumb,
        $secondThumb,
        $track,
        $trackFill,
        $labels,

        element         = this,
        instance        = $module.data(moduleNamespace),

        documentEventID,

        value,
        position,
        secondPos,
        offset,
        precision,
        isTouch,
        gapRatio = 1,

        initialPosition,
        initialLoad,
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing slider', settings);
          initialLoad = true;

          currentRange += 1;
          documentEventID = currentRange;

          isTouch = module.setup.testOutTouch();
          module.setup.layout();
          module.setup.labels();

          if(!module.is.disabled()) {
            module.bind.events();
          }

          module.read.metadata();
          module.read.settings();

          initialLoad = false;
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of slider', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous slider for', $module);
          clearInterval(instance.interval);
          module.unbind.events();
          module.unbind.slidingEvents();
          $module.removeData(moduleNamespace);
          instance = undefined;
        },

        setup: {
          layout: function() {
            if( $module.attr('tabindex') === undefined) {
              $module.attr('tabindex', 0);
            }
            if($module.find('.inner').length == 0) {
              $module.append("<div class='inner'>"
                             + "<div class='track'></div>"
                             + "<div class='track-fill'></div>"
                             + "<div class='thumb'></div>"
                             + "</div>");
            }
            precision = module.get.precision();
            $thumb = $module.find('.thumb:not(.second)');
            $currThumb = $thumb;
            if(module.is.range()) {
              if($module.find('.thumb.second').length == 0) {
                $module.find('.inner').append("<div class='thumb second'></div>");
              }
              $secondThumb = $module.find('.thumb.second');
            }
            $track = $module.find('.track');
            $trackFill = $module.find('.track-fill');
            offset = $thumb.width() / 2;
          },
          labels: function() {
            if(module.is.labeled()) {
              $labels = $module.find('.labels:not(.auto)');
              if($labels.length != 0) {
                module.setup.customLabel();
              } else {
                module.setup.autoLabel();
              }

              if (settings.showLabelTicks) {
                $module.addClass(className.ticked)
              }
            }
          },
          testOutTouch: function() {
            try {
             document.createEvent('TouchEvent');
             return true;
            } catch (e) {
             return false;
            }
          },
          customLabel: function() {
            var
              $children   = $labels.find('.label'),
              numChildren = $children.length,
              min         = module.get.min(),
              max         = module.get.max(),
              ratio
            ;
            $children.each(function(index) {
              var
                $child = $(this),
                attrValue = $child.attr('data-value')
              ;
              if(attrValue) {
                attrValue = attrValue > max ? max : attrValue < min ? min : attrValue;
                ratio = (attrValue - min) / (max - min);
              } else {
                ratio = (index + 1) / (numChildren + 1);
              }
              module.update.labelPosition(ratio, $(this));
            });
          },
          autoLabel: function() {
            if(module.get.step() != 0) {
              $labels = $module.find('.labels');
              if($labels.length != 0) {
                $labels.empty();
              }
              else {
                $labels = $module.append('<ul class="auto labels"></ul>').find('.labels');
              }
              for(var i = 0, len = module.get.numLabels(); i <= len; i++) {
                var
                  labelText = module.get.label(i),
                  $label = (labelText !== "") 
                    ? !(i % module.get.gapRatio())
                      ? $('<li class="label">' + labelText + '</li>') 
                      : $('<li class="halftick label"></li>')
                    : null,
                  ratio  = i / len
                ;
                if($label) {
                  module.update.labelPosition(ratio, $label);
                  $labels.append($label);
                }
              }
            }
          }
        },

        bind: {
          events: function() {
            module.bind.globalKeyboardEvents();
            module.bind.keyboardEvents();
            module.bind.mouseEvents();
            if(module.is.touch()) {
              module.bind.touchEvents();
            }
            if (settings.autoAdjustLabels) {
              module.bind.windowEvents();
            }
          },
          keyboardEvents: function() {
            module.verbose('Binding keyboard events');
            $module.on('keydown' + eventNamespace, module.event.keydown);
          },
          globalKeyboardEvents: function() {
            $(document).on('keydown' + eventNamespace + documentEventID, module.event.activateFocus);
          },
          mouseEvents: function() {
            module.verbose('Binding mouse events');
            $module.find('.track, .thumb, .inner').on('mousedown' + eventNamespace, function(event) {
              event.stopImmediatePropagation();
              event.preventDefault();
              module.event.down(event);
            });
            $module.on('mousedown' + eventNamespace, module.event.down);
            $module.on('mouseenter' + eventNamespace, function(event) {
              isHover = true;
            });
            $module.on('mouseleave' + eventNamespace, function(event) {
              isHover = false;
            });
          },
          touchEvents: function() {
            module.verbose('Binding touch events');
            $module.find('.track, .thumb, .inner').on('touchstart' + eventNamespace, function(event) {
              event.stopImmediatePropagation();
              event.preventDefault();
              module.event.down(event);
            });
            $module.on('touchstart' + eventNamespace, module.event.down);
          },
          slidingEvents: function() {
            // these don't need the identifier because we only ever want one of them to be registered with document
            module.verbose('Binding page wide events while handle is being draged');
            if(module.is.touch()) {
              $(document).on('touchmove' + eventNamespace, module.event.move);
              $(document).on('touchend' + eventNamespace, module.event.up);
            }
            else {
              $(document).on('mousemove' + eventNamespace, module.event.move);
              $(document).on('mouseup' + eventNamespace, module.event.up);
            }
          },
          windowEvents: function() {
            $window.on('resize' + eventNamespace, module.event.resize);
          }
        },

        unbind: {
          events: function() {
            $module.find('.track, .thumb, .inner').off('mousedown' + eventNamespace);
            $module.find('.track, .thumb, .inner').off('touchstart' + eventNamespace);
            $module.off('mousedown' + eventNamespace);
            $module.off('mouseenter' + eventNamespace);
            $module.off('mouseleave' + eventNamespace);
            $module.off('touchstart' + eventNamespace);
            $module.off('keydown' + eventNamespace);
            $module.off('focusout' + eventNamespace);
            $(document).off('keydown' + eventNamespace + documentEventID, module.event.activateFocus);
            $window.off('resize' + eventNamespace);
          },
          slidingEvents: function() {
            if(module.is.touch()) {
              $(document).off('touchmove' + eventNamespace);
              $(document).off('touchend' + eventNamespace);
            } else {
              $(document).off('mousemove' + eventNamespace);
              $(document).off('mouseup' + eventNamespace);
            }
          },
        },

        event: {
          down: function(event) {
            event.preventDefault();
            if(module.is.range()) {
              var
                eventPos = module.determine.eventPos(event),
                newPos = module.determine.pos(eventPos)
              ;
              // Special handling if range mode and both thumbs have the same value
              if(module.is.range() && settings.preventCrossover && module.thumbVal === module.secondThumbVal) {
                initialPosition = newPos;
                $currThumb = undefined;
              } else {
                $currThumb = module.determine.closestThumb(newPos);
              }
            }
            if(!module.is.disabled()) {
              module.bind.slidingEvents();
            }
          },
          move: function(event) {
            event.preventDefault();
            var value = module.determine.valueFromEvent(event);
            if($currThumb === undefined) {
              var
                eventPos = module.determine.eventPos(event),
                newPos = module.determine.pos(eventPos)
              ;
              $currThumb = initialPosition > newPos ? $thumb : $secondThumb;
            }
            if(module.get.step() == 0 || module.is.smooth()) {
              var
                thumbVal = module.thumbVal,
                secondThumbVal = module.secondThumbVal,
                thumbSmoothVal = module.determine.smoothValueFromEvent(event)
              ;
              if(!$currThumb.hasClass('second')) {
                if(settings.preventCrossover) {
                  value = Math.min(secondThumbVal, value);
                  thumbSmoothVal = Math.min(secondThumbVal, thumbSmoothVal);
                }
                thumbVal = value;
              } else {
                if(settings.preventCrossover) {
                  value = Math.max(thumbVal, value);
                  thumbSmoothVal = Math.max(thumbVal, thumbSmoothVal);
                }
                secondThumbVal = value;
              }
              value = Math.abs(thumbVal - (secondThumbVal || 0));
              module.update.position(thumbSmoothVal);
              settings.onMove.call(element, value, thumbVal, secondThumbVal);
            } else {
              module.update.value(value, function(value, thumbVal, secondThumbVal) {
                settings.onMove.call(element, value, thumbVal, secondThumbVal);
              });
            }
          },
          up: function(event) {
            event.preventDefault();
            var value = module.determine.valueFromEvent(event);
            module.set.value(value);
            module.unbind.slidingEvents();
          },
          keydown: function(event, first) {
            if(module.is.range() && settings.preventCrossover && module.thumbVal === module.secondThumbVal) {
              $currThumb = undefined;
            }
            if(module.is.focused()) {
              $(document).trigger(event);
            }
            if(first || module.is.focused()) {
              var step = module.determine.keyMovement(event);
              if(step != NO_STEP) {
                event.preventDefault();
                switch(step) {
                  case SINGLE_STEP:
                    module.takeStep();
                    break;
                  case BIG_STEP:
                    module.takeStep(module.get.multiplier());
                    break;
                  case SINGLE_BACKSTEP:
                    module.backStep();
                    break;
                  case BIG_BACKSTEP:
                    module.backStep(module.get.multiplier());
                    break;
                }
              }
            }
          },
          activateFocus: function(event) {
            if(!module.is.focused() && module.is.hover() && module.determine.keyMovement(event) != NO_STEP) {
              event.preventDefault();
              module.event.keydown(event, true);
              $module.focus();
            }
          },
          resize: function(_event) {
            // To avoid a useless performance cost, we only call the label refresh when its necessary
            if (gapRatio != module.get.gapRatio()) {
              module.setup.labels();
              gapRatio = module.get.gapRatio();
            }
          }
        },

        resync: function() {
          module.verbose('Resyncing thumb position based on value');
          if(module.is.range()) {
            module.update.position(module.secondThumbVal, $secondThumb);
          }
          module.update.position(module.thumbVal, $thumb);
          module.setup.labels();
        },
        takeStep: function(multiplier) {
          var
            multiplier = multiplier != undefined ? multiplier : 1,
            step = module.get.step(),
            currValue = module.get.currentThumbValue()
          ;
          module.verbose('Taking a step');
          if(step > 0) {
            module.set.value(currValue + step * multiplier);
          } else if (step == 0){
            var
              precision = module.get.precision(),
              newValue = currValue + (multiplier/precision)
            ;
            module.set.value(Math.round(newValue * precision) / precision);
          }
        },

        backStep: function(multiplier) {
          var
            multiplier = multiplier != undefined ? multiplier : 1,
            step = module.get.step(),
            currValue = module.get.currentThumbValue()
          ;
          module.verbose('Going back a step');
          if(step > 0) {
            module.set.value(currValue - step * multiplier);
          } else if (step == 0) {
            var
              precision = module.get.precision(),
              newValue = currValue - (multiplier/precision)
            ;
            module.set.value(Math.round(newValue * precision) / precision);
          }
        },

        is: {
          range: function() {
            return $module.hasClass(settings.className.range);
          },
          hover: function() {
            return isHover;
          },
          focused: function() {
            return $module.is(':focus');
          },
          disabled: function() {
            return $module.hasClass(settings.className.disabled);
          },
          labeled: function() {
            return $module.hasClass(settings.className.labeled);
          },
          reversed: function() {
            return $module.hasClass(settings.className.reversed);
          },
          vertical: function() {
            return $module.hasClass(settings.className.vertical);
          },
          smooth: function() {
            return settings.smooth || $module.hasClass(settings.className.smooth);
          },
          touch: function() {
            return isTouch;
          }
        },

        get: {
          trackOffset: function() {
            if (module.is.vertical()) {
              return $track.offset().top;
            } else {
              return $track.offset().left;
            }
          },
          trackLength: function() {
            if (module.is.vertical()) {
              return $track.height();
            } else {
              return $track.width();
            }
          },
          trackLeft: function() {
            if (module.is.vertical()) {
              return $track.position().top;
            } else {
              return $track.position().left;
            }
          },
          trackStartPos: function() {
            return module.is.reversed() ? module.get.trackLeft() + module.get.trackLength() : module.get.trackLeft();
          },
          trackEndPos: function() {
            return module.is.reversed() ? module.get.trackLeft() : module.get.trackLeft() + module.get.trackLength();
          },
          trackStartMargin: function () {
            var margin;
            if (module.is.vertical()) {
              margin = module.is.reversed() ? $module.css('padding-bottom') : $module.css('padding-top');
            } else {
              margin = module.is.reversed() ? $module.css('padding-right') : $module.css('padding-left');
            }
            return margin || '0px';
          },
          trackEndMargin: function () {
            var margin;
            if (module.is.vertical()) {
              margin = module.is.reversed() ? $module.css('padding-top') : $module.css('padding-bottom');
            } else {
              margin = module.is.reversed() ? $module.css('padding-left') : $module.css('padding-right');
            }
            return margin || '0px';
          },
          precision: function() {
            var
              decimalPlaces,
              step = module.get.step()
            ;
            if(step != 0) {
              var split = String(step).split('.');
              if(split.length == 2) {
                decimalPlaces = split[1].length;
              } else {
                decimalPlaces = 0;
              }
            } else {
              decimalPlaces = settings.decimalPlaces;
            }
            var precision = Math.pow(10, decimalPlaces);
            module.debug('Precision determined', precision);
            return precision;
          },
          min: function() {
            return settings.min;
          },
          max: function() {
            var step = module.get.step(),
                min = module.get.min(),
                quotient = step === 0 ? 0 : Math.floor((settings.max - min) / step),
                remainder = step === 0 ? 0 : (settings.max - min) % step;
            return remainder === 0 ? settings.max : min + quotient * step;
          },
          step: function() {
            return settings.step;
          },
          numLabels: function() {
            var value = Math.round((module.get.max() - module.get.min()) / module.get.step());
            module.debug('Determined that there should be ' + value + ' labels');
            return value;
          },
          labelType: function() {
            return settings.labelType;
          },
          label: function(value) {
            if(interpretLabel) {
              return interpretLabel(value);
            }

            switch (settings.labelType) {
              case settings.labelTypes.number:
                return Math.round(((value * module.get.step()) + module.get.min()) * precision ) / precision;
              case settings.labelTypes.letter:
                return alphabet[(value) % 26];
              default:
                return value;
            }
          },
          value: function() {
            return value;
          },
          currentThumbValue: function() {
            return $currThumb !== undefined && $currThumb.hasClass('second') ? module.secondThumbVal : module.thumbVal;
          },
          thumbValue: function(which) {
            switch(which) {
              case 'second':
                if(module.is.range()) {
                  return module.secondThumbVal;
                }
                else {
                  module.error(error.notrange);
                  break;
                }
              case 'first':
              default:
                return module.thumbVal;
            }
          },
          multiplier: function() {
            return settings.pageMultiplier;
          },
          thumbPosition: function(which) {
            switch(which) {
              case 'second':
                if(module.is.range()) {
                  return secondPos;
                }
                else {
                  module.error(error.notrange);
                  break;
                }
              case 'first':
              default:
                return position;
            }
          },
          gapRatio: function() {
            var gapRatio = 1;
            
            if( settings.autoAdjustLabels ) {
              var 
                numLabels = module.get.numLabels(),
                trackLength = module.get.trackLength(),
                gapCounter = 1
              ;

              // While the distance between two labels is too short,
              // we divide the number of labels at each iteration
              // and apply only if the modulo of the operation is an odd number.
              if(trackLength>0){
                while ((trackLength / numLabels) * gapCounter < settings.labelDistance) {
                  if( !(numLabels % gapCounter) ) {
                    gapRatio = gapCounter;
                  }
                  gapCounter += 1;
                }
              }
            }
            return gapRatio;
          }
        },

        determine: {
          pos: function(pagePos) {
            return module.is.reversed()
              ?
              module.get.trackStartPos() - pagePos + module.get.trackOffset()
              :
              pagePos - module.get.trackOffset() - module.get.trackStartPos()
            ;
          },
          closestThumb: function(eventPos) {
            var
              thumbPos = parseFloat(module.determine.thumbPos($thumb)),
              thumbDelta = Math.abs(eventPos - thumbPos),
              secondThumbPos = parseFloat(module.determine.thumbPos($secondThumb)),
              secondThumbDelta = Math.abs(eventPos - secondThumbPos)
            ;
            if(thumbDelta === secondThumbDelta && module.get.thumbValue() === module.get.min()) {
              return $secondThumb;
            }
            return thumbDelta <= secondThumbDelta ? $thumb : $secondThumb;
          },
          closestThumbPos: function(eventPos) {
            var
              thumbPos = parseFloat(module.determine.thumbPos($thumb)),
              thumbDelta = Math.abs(eventPos - thumbPos),
              secondThumbPos = parseFloat(module.determine.thumbPos($secondThumb)),
              secondThumbDelta = Math.abs(eventPos - secondThumbPos)
            ;
            return thumbDelta <= secondThumbDelta ? thumbPos : secondThumbPos;
          },
          thumbPos: function($element) {
            var pos =
              module.is.vertical()
              ?
              module.is.reversed() ? $element.css('bottom') : $element.css('top')
              :
              module.is.reversed() ? $element.css('right') : $element.css('left')
            ;
            return pos;
          },
          positionFromValue: function(value) {
            var
              min = module.get.min(),
              max = module.get.max(),
              value = value > max ? max : value < min ? min : value,
              trackLength = module.get.trackLength(),
              ratio = (value - min) / (max - min),
              position = Math.round(ratio * trackLength)
            ;
            module.verbose('Determined position: ' + position + ' from value: ' + value);
            return position;
          },
          positionFromRatio: function(ratio) {
            var
              trackLength = module.get.trackLength(),
              step = module.get.step(),
              position = Math.round(ratio * trackLength),
              adjustedPos = (step == 0) ? position : Math.round(position / step) * step
            ;
            return adjustedPos;
          },
          valueFromEvent: function(event) {
            var
              eventPos = module.determine.eventPos(event),
              newPos = module.determine.pos(eventPos),
              value
            ;
            if(eventPos < module.get.trackOffset()) {
              value = module.is.reversed() ? module.get.max() : module.get.min();
            } else if(eventPos > module.get.trackOffset() + module.get.trackLength()) {
              value = module.is.reversed() ? module.get.min() : module.get.max();
            } else {
              value = module.determine.value(newPos);
            }
            return value;
          },
          smoothValueFromEvent: function(event) {
            var
              min = module.get.min(),
              max = module.get.max(),
              trackLength = module.get.trackLength(),
              eventPos = module.determine.eventPos(event),
              newPos = eventPos - module.get.trackOffset(),
              ratio,
              value
            ;
            newPos = newPos < 0 ? 0 : newPos > trackLength ? trackLength : newPos;
            ratio = newPos / trackLength;
            if (module.is.reversed()) {
              ratio = 1 - ratio;
            }
            value = ratio * (max - min) + min;
            return value;
          },
          eventPos: function(event) {
            if(module.is.touch()) {
              var
                touchEvent = event.changedTouches ? event : event.originalEvent,
                touches = touchEvent.changedTouches[0] ? touchEvent.changedTouches : touchEvent.touches,
                touchY = touches[0].pageY,
                touchX = touches[0].pageX
              ;
              return module.is.vertical() ? touchY : touchX;
            }
            var
              clickY = event.pageY || event.originalEvent.pageY,
              clickX = event.pageX || event.originalEvent.pageX
            ;
            return module.is.vertical() ? clickY : clickX;
          },
          value: function(position) {
            var
              startPos = module.is.reversed() ? module.get.trackEndPos() : module.get.trackStartPos(),
              endPos = module.is.reversed() ? module.get.trackStartPos() : module.get.trackEndPos(),
              ratio = (position - startPos) / (endPos - startPos),
              range = module.get.max() - module.get.min(),
              step = module.get.step(),
              value = (ratio * range),
              difference = (step == 0) ? value : Math.round(value / step) * step
            ;
            module.verbose('Determined value based upon position: ' + position + ' as: ' + value);
            if(value != difference) {
              module.verbose('Rounding value to closest step: ' + difference);
            }
            // Use precision to avoid ugly Javascript floating point rounding issues
            // (like 35 * .01 = 0.35000000000000003)
            difference = Math.round(difference * precision) / precision;
            module.verbose('Cutting off additional decimal places');
            return difference + module.get.min();
          },
          keyMovement: function(event) {
            var
              key = event.which,
              downArrow =
                module.is.vertical()
                ?
                module.is.reversed() ? keys.downArrow : keys.upArrow
                :
                keys.downArrow
              ,
              upArrow =
                module.is.vertical()
                ?
                module.is.reversed() ? keys.upArrow : keys.downArrow
                :
                keys.upArrow
              ,
              leftArrow =
                !module.is.vertical()
                ?
                module.is.reversed() ? keys.rightArrow : keys.leftArrow
                :
                keys.leftArrow
              ,
              rightArrow =
                !module.is.vertical()
                ?
                module.is.reversed() ? keys.leftArrow : keys.rightArrow
                :
                keys.rightArrow
            ;
            if(key == downArrow || key == leftArrow) {
              return SINGLE_BACKSTEP;
            } else if(key == upArrow || key == rightArrow) {
              return SINGLE_STEP;
            } else if (key == keys.pageDown) {
              return BIG_BACKSTEP;
            } else if (key == keys.pageUp) {
              return BIG_STEP;
            } else {
              return NO_STEP;
            }
          }
        },

        handleNewValuePosition: function(val) {
          var
            min = module.get.min(),
            max = module.get.max(),
            newPos
          ;
          if (val <= min) {
            val = min;
          } else if (val >= max) {
            val = max;
          }
          newPos = module.determine.positionFromValue(val);
          return newPos;
        },

        set: {
          value: function(newValue) {
            module.update.value(newValue, function(value, thumbVal, secondThumbVal) {
              if (!initialLoad || settings.fireOnInit){
                settings.onChange.call(element, value, thumbVal, secondThumbVal);
                settings.onMove.call(element, value, thumbVal, secondThumbVal);
              }
            });
          },
          rangeValue: function(first, second) {
            if(module.is.range()) {
              var
                min = module.get.min(),
                max = module.get.max()
              ;
              if (first <= min) {
                first = min;
              } else if(first >= max){
                first = max;
              }
              if (second <= min) {
                second = min;
              } else if(second >= max){
                second = max;
              }
              module.thumbVal = first;
              module.secondThumbVal = second;
              value = Math.abs(module.thumbVal - module.secondThumbVal);
              module.update.position(module.thumbVal, $thumb);
              module.update.position(module.secondThumbVal, $secondThumb);
              if (!initialLoad || settings.fireOnInit) {
                settings.onChange.call(element, value, module.thumbVal, module.secondThumbVal);
                settings.onMove.call(element, value, module.thumbVal, module.secondThumbVal);
              }
            } else {
              module.error(error.notrange);
            }
          },
          position: function(position, which) {
            var thumbVal = module.determine.value(position);
            switch (which) {
              case 'second':
                module.secondThumbVal = thumbVal;
                module.update.position(thumbVal, $secondThumb);
                break;
              default:
                module.thumbVal = thumbVal;
                module.update.position(thumbVal, $thumb);
            }
            value = Math.abs(module.thumbVal - (module.secondThumbVal || 0));
            module.set.value(value);
          }
        },

        update: {
          value: function(newValue, callback) {
            var
              min = module.get.min(),
              max = module.get.max()
            ;
            if (newValue <= min) {
              newValue = min;
            } else if(newValue >= max){
              newValue = max;
            }
            if(!module.is.range()) {
              value = newValue;
              module.thumbVal = value;
            } else {
              if($currThumb === undefined) {
                $currThumb = newValue <= module.get.currentThumbValue() ? $thumb : $secondThumb;
              }
              if(!$currThumb.hasClass('second')) {
                if(settings.preventCrossover) {
                  newValue = Math.min(module.secondThumbVal, newValue);
                }
                module.thumbVal = newValue;
              } else {
                if(settings.preventCrossover) {
                  newValue = Math.max(module.thumbVal, newValue);
                }
                module.secondThumbVal = newValue;
              }
              value = Math.abs(module.thumbVal - module.secondThumbVal);
            }
            module.update.position(newValue);
            module.debug('Setting slider value to ' + value);
            if(typeof callback === 'function') {
              callback(value, module.thumbVal, module.secondThumbVal);
            }
          },
          position: function(newValue, $element) {
            var
              newPos = module.handleNewValuePosition(newValue),
              $targetThumb = $element != undefined ? $element : $currThumb,
              thumbVal = module.thumbVal || module.get.min(),
              secondThumbVal = module.secondThumbVal || module.get.min()
            ;
            if(module.is.range()) {
              if(!$targetThumb.hasClass('second')) {
                position = newPos;
                thumbVal = newValue;
              } else {
                secondPos = newPos;
                secondThumbVal = newValue;
              }
            } else {
              position = newPos;
              thumbVal = newValue;
            }
            var
              trackPosValue,
              thumbPosValue,
              min = module.get.min(),
              max = module.get.max(),
              thumbPosPercent = 100 * (newValue - min) / (max - min),
              trackStartPosPercent = 100 * (Math.min(thumbVal, secondThumbVal) - min) / (max - min),
              trackEndPosPercent = 100 * (1 - (Math.max(thumbVal, secondThumbVal) - min) / (max - min))
            ;
            if (module.is.vertical()) {
              if (module.is.reversed()) {
                thumbPosValue = {bottom: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', top: 'auto'};
                trackPosValue = {bottom: trackStartPosPercent + '%', top: trackEndPosPercent + '%'};
              }
              else {
                thumbPosValue = {top: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', bottom: 'auto'};
                trackPosValue = {top: trackStartPosPercent + '%', bottom: trackEndPosPercent + '%'};
              }
            } else {
              if (module.is.reversed()) {
                thumbPosValue = {right: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', left: 'auto'};
                trackPosValue = {right: trackStartPosPercent + '%', left: trackEndPosPercent + '%'};
              }
              else {
                thumbPosValue = {left: 'calc(' + thumbPosPercent + '% - ' + offset + 'px)', right: 'auto'};
                trackPosValue = {left: trackStartPosPercent + '%', right: trackEndPosPercent + '%'};
              }
            }
            $targetThumb.css(thumbPosValue);
            $trackFill.css(trackPosValue);
            module.debug('Setting slider position to ' + newPos);
          },
          labelPosition: function (ratio, $label) {
            var
              startMargin = module.get.trackStartMargin(),
              endMargin   = module.get.trackEndMargin(),
              posDir =
                module.is.vertical()
                ?
                module.is.reversed() ? 'bottom' : 'top'
                :
                  module.is.reversed() ? 'right' : 'left',
              startMarginMod = module.is.reversed() && !module.is.vertical() ? ' - ' : ' + '
            ;
            var position = '(100% - ' + startMargin + ' - ' + endMargin + ') * ' + ratio;
            $label.css(posDir, 'calc(' + position + startMarginMod + startMargin + ')');
          }
        },

        goto: {
          max: function() {
            module.set.value(module.get.max());
          },
          min: function() {
            module.set.value(module.get.min());
          },
        },

        read: {
          metadata: function() {
            var
              data = {
                thumbVal        : $module.data(metadata.thumbVal),
                secondThumbVal  : $module.data(metadata.secondThumbVal)
              }
            ;
            if(data.thumbVal) {
              if(module.is.range() && data.secondThumbVal) {
                module.debug('Current value set from metadata', data.thumbVal, data.secondThumbVal);
                module.set.rangeValue(data.thumbVal, data.secondThumbVal);
              } else {
                module.debug('Current value set from metadata', data.thumbVal);
                module.set.value(data.thumbVal);
              }
            }
          },
          settings: function() {
            if(settings.start !== false) {
              if(module.is.range()) {
                module.debug('Start position set from settings', settings.start, settings.end);
                module.set.rangeValue(settings.start, settings.end);
              } else {
                module.debug('Start position set from settings', settings.start);
                module.set.value(settings.start);
              }
            }
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
            response = found.apply(context, passedArguments);
          }
          else if(found !== undefined) {
            response = found;
          }
          if($.isArray(returnedValue)) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;

};

$.fn.slider.settings = {

  silent       : false,
  debug        : false,
  verbose      : false,
  performance  : true,

  name         : 'Slider',
  namespace    : 'slider',

  error    : {
    method    : 'The method you called is not defined.',
    notrange : 'This slider is not a range slider'
  },

  metadata: {
    thumbVal        : 'thumbVal',
    secondThumbVal  : 'secondThumbVal'
  },

  min              : 0,
  max              : 20,
  step             : 1,
  start            : 0,
  end              : 20,
  labelType        : 'number',
  showLabelTicks   : false,
  smooth           : false,
  autoAdjustLabels : true,
  labelDistance    : 100,
  preventCrossover : true,
  fireOnInit       : false,

  //the decimal place to round to if step is undefined
  decimalPlaces  : 2,

  // page up/down multiplier. How many more times the steps to take on page up/down press
  pageMultiplier : 2,

  selector: {

  },

  className     : {
    reversed : 'reversed',
    disabled : 'disabled',
    labeled  : 'labeled',
    ticked   : 'ticked',
    vertical : 'vertical',
    range    : 'range',
    smooth   : 'smooth'
  },

  keys : {
    pageUp     : 33,
    pageDown   : 34,
    leftArrow  : 37,
    upArrow    : 38,
    rightArrow : 39,
    downArrow  : 40
  },

  labelTypes    : {
    number  : 'number',
    letter  : 'letter'
  },

  onChange : function(value, thumbVal, secondThumbVal){},
  onMove   : function(value, thumbVal, secondThumbVal){},
};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Rating
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

$.fn.rating = function(parameters) {
  var
    $allModules     = $(this),
    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),
    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.rating.settings, parameters)
          : $.extend({}, $.fn.rating.settings),

        namespace       = settings.namespace,
        className       = settings.className,
        metadata        = settings.metadata,
        selector        = settings.selector,
        cssVars         = settings.cssVars,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        element         = this,
        instance        = $(this).data(moduleNamespace),

        $module         = $(this),
        $icon           = $module.find(selector.icon),

        initialLoad,
        module
      ;

      module = {

        initialize: function() {
          module.verbose('Initializing rating module', settings);

          if($icon.length === 0) {
            module.setup.layout();
          }

          if(settings.interactive && !module.is.disabled()) {
            module.enable();
          }
          else {
            module.disable();
          }
          module.set.initialLoad();
          module.set.rating( module.get.initialRating() );
          module.remove.initialLoad();
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Instantiating module', settings);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous instance', instance);
          module.remove.events();
          $module
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          $icon   = $module.find(selector.icon);
        },

        setup: {
          layout: function() {
            var
              maxRating = module.get.maxRating(),
              icon      = module.get.icon(),
              html      = $.fn.rating.settings.templates.icon(maxRating, icon)
            ;
            module.debug('Generating icon html dynamically');
            $module
              .html(html)
            ;
            module.refresh();
          }
        },

        event: {
          mouseenter: function() {
            var
              $activeIcon = $(this)
            ;
            $activeIcon
              .nextAll()
                .removeClass(className.selected)
            ;
            $module
              .addClass(className.selected)
            ;
            $activeIcon
              .addClass(className.selected)
                .prevAll()
                .addClass(className.selected)
            ;
          },
          mouseleave: function() {
            $module
              .removeClass(className.selected)
            ;
            $icon
              .removeClass(className.selected)
            ;
          },
          click: function() {
            var
              $activeIcon   = $(this),
              currentRating = module.get.rating(),
              rating        = $icon.index($activeIcon) + 1,
              canClear      = (settings.clearable == 'auto')
               ? ($icon.length === 1)
               : settings.clearable
            ;
            if(canClear && currentRating == rating) {
              module.clearRating();
            }
            else {
              module.set.rating( rating );
            }
          }
        },

        clearRating: function() {
          module.debug('Clearing current rating');
          module.set.rating(0);
        },

        bind: {
          events: function() {
            module.verbose('Binding events');
            $module
              .on('mouseenter' + eventNamespace, selector.icon, module.event.mouseenter)
              .on('mouseleave' + eventNamespace, selector.icon, module.event.mouseleave)
              .on('click'      + eventNamespace, selector.icon, module.event.click)
            ;
          }
        },

        remove: {
          events: function() {
            module.verbose('Removing events');
            $module
              .off(eventNamespace)
            ;
          },
          initialLoad: function() {
            initialLoad = false;
          }
        },

        enable: function() {
          module.debug('Setting rating to interactive mode');
          module.bind.events();
          $module
            .removeClass(className.disabled)
          ;
        },

        disable: function() {
          module.debug('Setting rating to read-only mode');
          module.remove.events();
          $module
            .addClass(className.disabled)
          ;
        },

        is: {
          initialLoad: function() {
            return initialLoad;
          },
          disabled: function() {
            return $module.hasClass(className.disabled);
          }
        },

        get: {
          icon: function(){
            var icon = $module.data(metadata.icon);
            if (icon) {
              $module.removeData(metadata.icon);
            }
            return icon || settings.icon;
          },
          initialRating: function() {
            if($module.data(metadata.rating) !== undefined) {
              $module.removeData(metadata.rating);
              return $module.data(metadata.rating);
            }
            return settings.initialRating;
          },
          maxRating: function() {
            if($module.data(metadata.maxRating) !== undefined) {
              $module.removeData(metadata.maxRating);
              return $module.data(metadata.maxRating);
            }
            return settings.maxRating;
          },
          rating: function() {
            var
              currentRating = $icon.filter('.' + className.active).length
            ;
            module.verbose('Current rating retrieved', currentRating);
            return currentRating;
          }
        },

        set: {
          rating: function(rating) {
            var
              ratingIndex = Math.floor(
                (rating - 1 >= 0)
                  ? (rating - 1)
                  : 0
              ),
              $activeIcon = $icon.eq(ratingIndex),
              $partialActiveIcon = rating <= 1
                ? $activeIcon
                : $activeIcon.next()
              ,
              filledPercentage = (rating % 1) * 100
            ;
            $module
              .removeClass(className.selected)
            ;
            $icon
              .removeClass(className.selected)
              .removeClass(className.active)
              .removeClass(className.partiallyActive)
            ;
            if(rating > 0) {
              module.verbose('Setting current rating to', rating);
              $activeIcon
                .prevAll()
                .addBack()
                .addClass(className.active)
              ;
              if($activeIcon.next() && rating % 1 !== 0) {
                $partialActiveIcon
                  .addClass(className.partiallyActive)
                  .addClass(className.active)
                ;
                $partialActiveIcon
                  .css(cssVars.filledCustomPropName, filledPercentage + '%')
                ;
                if($partialActiveIcon.css('backgroundColor') === 'transparent') {
                  $partialActiveIcon
                    .removeClass(className.partiallyActive)
                    .removeClass(className.active)
                  ;
                }
              }
            }
            if(!module.is.initialLoad()) {
              settings.onRate.call(element, rating);
            }
          },
          initialLoad: function() {
            initialLoad = true;
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
          if ( $.isFunction( found ) ) {
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
      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.rating.settings = {

  name          : 'Rating',
  namespace     : 'rating',

  icon          : 'star',

  silent        : false,
  debug         : false,
  verbose       : false,
  performance   : true,

  initialRating : 0,
  interactive   : true,
  maxRating     : 4,
  clearable     : 'auto',

  fireOnInit    : false,

  onRate        : function(rating){},

  error         : {
    method    : 'The method you called is not defined',
    noMaximum : 'No maximum rating specified. Cannot generate HTML automatically'
  },


  metadata: {
    rating    : 'rating',
    maxRating : 'maxRating',
    icon      : 'icon'
  },

  className : {
    active   : 'active',
    disabled : 'disabled',
    selected : 'selected',
    loading  : 'loading',
    partiallyActive : 'partial'
  },

  cssVars : {
    filledCustomPropName : '--full'
  },

  selector  : {
    icon : '.icon'
  },

  templates: {
    icon: function(maxRating, iconClass) {
      var
        icon = 1,
        html = ''
      ;
      while(icon <= maxRating) {
        html += '<i class="'+iconClass+' icon"></i>';
        icon++;
      }
      return html;
    }
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Search
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

$.fn.search = function(parameters) {
  var
    $allModules     = $(this),
    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),
    returnedValue
  ;
  $(this)
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.search.settings, parameters)
          : $.extend({}, $.fn.search.settings),

        className        = settings.className,
        metadata         = settings.metadata,
        regExp           = settings.regExp,
        fields           = settings.fields,
        selector         = settings.selector,
        error            = settings.error,
        namespace        = settings.namespace,

        eventNamespace   = '.' + namespace,
        moduleNamespace  = namespace + '-module',

        $module          = $(this),
        $prompt          = $module.find(selector.prompt),
        $searchButton    = $module.find(selector.searchButton),
        $results         = $module.find(selector.results),
        $result          = $module.find(selector.result),
        $category        = $module.find(selector.category),

        element          = this,
        instance         = $module.data(moduleNamespace),

        disabledBubbled  = false,
        resultsDismissed = false,

        module
      ;

      module = {

        initialize: function() {
          module.verbose('Initializing module');
          module.get.settings();
          module.determine.searchFields();
          module.bind.events();
          module.set.type();
          module.create.results();
          module.instantiate();
        },
        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },
        destroy: function() {
          module.verbose('Destroying instance');
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          module.debug('Refreshing selector cache');
          $prompt         = $module.find(selector.prompt);
          $searchButton   = $module.find(selector.searchButton);
          $category       = $module.find(selector.category);
          $results        = $module.find(selector.results);
          $result         = $module.find(selector.result);
        },

        refreshResults: function() {
          $results = $module.find(selector.results);
          $result  = $module.find(selector.result);
        },

        bind: {
          events: function() {
            module.verbose('Binding events to search');
            if(settings.automatic) {
              $module
                .on(module.get.inputEvent() + eventNamespace, selector.prompt, module.event.input)
              ;
              $prompt
                .attr('autocomplete', 'off')
              ;
            }
            $module
              // prompt
              .on('focus'     + eventNamespace, selector.prompt, module.event.focus)
              .on('blur'      + eventNamespace, selector.prompt, module.event.blur)
              .on('keydown'   + eventNamespace, selector.prompt, module.handleKeyboard)
              // search button
              .on('click'     + eventNamespace, selector.searchButton, module.query)
              // results
              .on('mousedown' + eventNamespace, selector.results, module.event.result.mousedown)
              .on('mouseup'   + eventNamespace, selector.results, module.event.result.mouseup)
              .on('click'     + eventNamespace, selector.result,  module.event.result.click)
            ;
          }
        },

        determine: {
          searchFields: function() {
            // this makes sure $.extend does not add specified search fields to default fields
            // this is the only setting which should not extend defaults
            if(parameters && parameters.searchFields !== undefined) {
              settings.searchFields = parameters.searchFields;
            }
          }
        },

        event: {
          input: function() {
            if(settings.searchDelay) {
              clearTimeout(module.timer);
              module.timer = setTimeout(function() {
                if(module.is.focused()) {
                  module.query();
                }
              }, settings.searchDelay);
            }
            else {
              module.query();
            }
          },
          focus: function() {
            module.set.focus();
            if(settings.searchOnFocus && module.has.minimumCharacters() ) {
              module.query(function() {
                if(module.can.show() ) {
                  module.showResults();
                }
              });
            }
          },
          blur: function(event) {
            var
              pageLostFocus = (document.activeElement === this),
              callback      = function() {
                module.cancel.query();
                module.remove.focus();
                module.timer = setTimeout(module.hideResults, settings.hideDelay);
              }
            ;
            if(pageLostFocus) {
              return;
            }
            resultsDismissed = false;
            if(module.resultsClicked) {
              module.debug('Determining if user action caused search to close');
              $module
                .one('click.close' + eventNamespace, selector.results, function(event) {
                  if(module.is.inMessage(event) || disabledBubbled) {
                    $prompt.focus();
                    return;
                  }
                  disabledBubbled = false;
                  if( !module.is.animating() && !module.is.hidden()) {
                    callback();
                  }
                })
              ;
            }
            else {
              module.debug('Input blurred without user action, closing results');
              callback();
            }
          },
          result: {
            mousedown: function() {
              module.resultsClicked = true;
            },
            mouseup: function() {
              module.resultsClicked = false;
            },
            click: function(event) {
              module.debug('Search result selected');
              var
                $result = $(this),
                $title  = $result.find(selector.title).eq(0),
                $link   = $result.is('a[href]')
                  ? $result
                  : $result.find('a[href]').eq(0),
                href    = $link.attr('href')   || false,
                target  = $link.attr('target') || false,
                // title is used for result lookup
                value   = ($title.length > 0)
                  ? $title.text()
                  : false,
                results = module.get.results(),
                result  = $result.data(metadata.result) || module.get.result(value, results)
              ;
              if(value) {
                module.set.value(value);
              }
              if( $.isFunction(settings.onSelect) ) {
                if(settings.onSelect.call(element, result, results) === false) {
                  module.debug('Custom onSelect callback cancelled default select action');
                  disabledBubbled = true;
                  return;
                }
              }
              module.hideResults();
              if(href) {
                module.verbose('Opening search link found in result', $link);
                if(target == '_blank' || event.ctrlKey) {
                  window.open(href);
                }
                else {
                  window.location.href = (href);
                }
              }
            }
          }
        },
        handleKeyboard: function(event) {
          var
            // force selector refresh
            $result         = $module.find(selector.result),
            $category       = $module.find(selector.category),
            $activeResult   = $result.filter('.' + className.active),
            currentIndex    = $result.index( $activeResult ),
            resultSize      = $result.length,
            hasActiveResult = $activeResult.length > 0,

            keyCode         = event.which,
            keys            = {
              backspace : 8,
              enter     : 13,
              escape    : 27,
              upArrow   : 38,
              downArrow : 40
            },
            newIndex
          ;
          // search shortcuts
          if(keyCode == keys.escape) {
            module.verbose('Escape key pressed, blurring search field');
            module.hideResults();
            resultsDismissed = true;
          }
          if( module.is.visible() ) {
            if(keyCode == keys.enter) {
              module.verbose('Enter key pressed, selecting active result');
              if( $result.filter('.' + className.active).length > 0 ) {
                module.event.result.click.call($result.filter('.' + className.active), event);
                event.preventDefault();
                return false;
              }
            }
            else if(keyCode == keys.upArrow && hasActiveResult) {
              module.verbose('Up key pressed, changing active result');
              newIndex = (currentIndex - 1 < 0)
                ? currentIndex
                : currentIndex - 1
              ;
              $category
                .removeClass(className.active)
              ;
              $result
                .removeClass(className.active)
                .eq(newIndex)
                  .addClass(className.active)
                  .closest($category)
                    .addClass(className.active)
              ;
              event.preventDefault();
            }
            else if(keyCode == keys.downArrow) {
              module.verbose('Down key pressed, changing active result');
              newIndex = (currentIndex + 1 >= resultSize)
                ? currentIndex
                : currentIndex + 1
              ;
              $category
                .removeClass(className.active)
              ;
              $result
                .removeClass(className.active)
                .eq(newIndex)
                  .addClass(className.active)
                  .closest($category)
                    .addClass(className.active)
              ;
              event.preventDefault();
            }
          }
          else {
            // query shortcuts
            if(keyCode == keys.enter) {
              module.verbose('Enter key pressed, executing query');
              module.query();
              module.set.buttonPressed();
              $prompt.one('keyup', module.remove.buttonFocus);
            }
          }
        },

        setup: {
          api: function(searchTerm, callback) {
            var
              apiSettings = {
                debug             : settings.debug,
                on                : false,
                cache             : settings.cache,
                action            : 'search',
                urlData           : {
                  query : searchTerm
                },
                onSuccess         : function(response) {
                  module.parse.response.call(element, response, searchTerm);
                  callback();
                },
                onFailure         : function() {
                  module.displayMessage(error.serverError);
                  callback();
                },
                onAbort : function(response) {
                },
                onError           : module.error
              }
            ;
            $.extend(true, apiSettings, settings.apiSettings);
            module.verbose('Setting up API request', apiSettings);
            $module.api(apiSettings);
          }
        },

        can: {
          useAPI: function() {
            return $.fn.api !== undefined;
          },
          show: function() {
            return module.is.focused() && !module.is.visible() && !module.is.empty();
          },
          transition: function() {
            return settings.transition && $.fn.transition !== undefined && $module.transition('is supported');
          }
        },

        is: {
          animating: function() {
            return $results.hasClass(className.animating);
          },
          hidden: function() {
            return $results.hasClass(className.hidden);
          },
          inMessage: function(event) {
            if(!event.target) {
              return;
            }
            var
              $target = $(event.target),
              isInDOM = $.contains(document.documentElement, event.target)
            ;
            return (isInDOM && $target.closest(selector.message).length > 0);
          },
          empty: function() {
            return ($results.html() === '');
          },
          visible: function() {
            return ($results.filter(':visible').length > 0);
          },
          focused: function() {
            return ($prompt.filter(':focus').length > 0);
          }
        },

        get: {
          settings: function() {
            if($.isPlainObject(parameters) && parameters.searchFullText) {
              settings.fullTextSearch = parameters.searchFullText;
              module.error(settings.error.oldSearchSyntax, element);
            }
            if (settings.ignoreDiacritics && !String.prototype.normalize) {
              settings.ignoreDiacritics = false;
              module.error(error.noNormalize, element);
            }
          },
          inputEvent: function() {
            var
              prompt = $prompt[0],
              inputEvent   = (prompt !== undefined && prompt.oninput !== undefined)
                ? 'input'
                : (prompt !== undefined && prompt.onpropertychange !== undefined)
                  ? 'propertychange'
                  : 'keyup'
            ;
            return inputEvent;
          },
          value: function() {
            return $prompt.val();
          },
          results: function() {
            var
              results = $module.data(metadata.results)
            ;
            return results;
          },
          result: function(value, results) {
            var
              result       = false
            ;
            value = (value !== undefined)
              ? value
              : module.get.value()
            ;
            results = (results !== undefined)
              ? results
              : module.get.results()
            ;
            if(settings.type === 'category') {
              module.debug('Finding result that matches', value);
              $.each(results, function(index, category) {
                if(Array.isArray(category.results)) {
                  result = module.search.object(value, category.results)[0];
                  // don't continue searching if a result is found
                  if(result) {
                    return false;
                  }
                }
              });
            }
            else {
              module.debug('Finding result in results object', value);
              result = module.search.object(value, results)[0];
            }
            return result || false;
          },
        },

        select: {
          firstResult: function() {
            module.verbose('Selecting first result');
            $result.first().addClass(className.active);
          }
        },

        set: {
          focus: function() {
            $module.addClass(className.focus);
          },
          loading: function() {
            $module.addClass(className.loading);
          },
          value: function(value) {
            module.verbose('Setting search input value', value);
            $prompt
              .val(value)
            ;
          },
          type: function(type) {
            type = type || settings.type;
            if(settings.type == 'category') {
              $module.addClass(settings.type);
            }
          },
          buttonPressed: function() {
            $searchButton.addClass(className.pressed);
          }
        },

        remove: {
          loading: function() {
            $module.removeClass(className.loading);
          },
          focus: function() {
            $module.removeClass(className.focus);
          },
          buttonPressed: function() {
            $searchButton.removeClass(className.pressed);
          },
          diacritics: function(text) {
            return settings.ignoreDiacritics ?  text.normalize('NFD').replace(/[\u0300-\u036f]/g, '') : text;
          }
        },

        query: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          var
            searchTerm = module.get.value(),
            cache = module.read.cache(searchTerm)
          ;
          callback = callback || function() {};
          if( module.has.minimumCharacters() )  {
            if(cache) {
              module.debug('Reading result from cache', searchTerm);
              module.save.results(cache.results);
              module.addResults(cache.html);
              module.inject.id(cache.results);
              callback();
            }
            else {
              module.debug('Querying for', searchTerm);
              if($.isPlainObject(settings.source) || Array.isArray(settings.source)) {
                module.search.local(searchTerm);
                callback();
              }
              else if( module.can.useAPI() ) {
                module.search.remote(searchTerm, callback);
              }
              else {
                module.error(error.source);
                callback();
              }
            }
            settings.onSearchQuery.call(element, searchTerm);
          }
          else {
            module.hideResults();
          }
        },

        search: {
          local: function(searchTerm) {
            var
              results = module.search.object(searchTerm, settings.source),
              searchHTML
            ;
            module.set.loading();
            module.save.results(results);
            module.debug('Returned full local search results', results);
            if(settings.maxResults > 0) {
              module.debug('Using specified max results', results);
              results = results.slice(0, settings.maxResults);
            }
            if(settings.type == 'category') {
              results = module.create.categoryResults(results);
            }
            searchHTML = module.generateResults({
              results: results
            });
            module.remove.loading();
            module.addResults(searchHTML);
            module.inject.id(results);
            module.write.cache(searchTerm, {
              html    : searchHTML,
              results : results
            });
          },
          remote: function(searchTerm, callback) {
            callback = $.isFunction(callback)
              ? callback
              : function(){}
            ;
            if($module.api('is loading')) {
              $module.api('abort');
            }
            module.setup.api(searchTerm, callback);
            $module
              .api('query')
            ;
          },
          object: function(searchTerm, source, searchFields) {
            searchTerm = module.remove.diacritics(String(searchTerm));
            var
              results      = [],
              exactResults = [],
              fuzzyResults = [],
              searchExp    = searchTerm.replace(regExp.escape, '\\$&'),
              matchRegExp  = new RegExp(regExp.beginsWith + searchExp, 'i'),

              // avoid duplicates when pushing results
              addResult = function(array, result) {
                var
                  notResult      = ($.inArray(result, results) == -1),
                  notFuzzyResult = ($.inArray(result, fuzzyResults) == -1),
                  notExactResults = ($.inArray(result, exactResults) == -1)
                ;
                if(notResult && notFuzzyResult && notExactResults) {
                  array.push(result);
                }
              }
            ;
            source = source || settings.source;
            searchFields = (searchFields !== undefined)
              ? searchFields
              : settings.searchFields
            ;

            // search fields should be array to loop correctly
            if(!Array.isArray(searchFields)) {
              searchFields = [searchFields];
            }

            // exit conditions if no source
            if(source === undefined || source === false) {
              module.error(error.source);
              return [];
            }
            // iterate through search fields looking for matches
            $.each(searchFields, function(index, field) {
              $.each(source, function(label, content) {
                var
                  fieldExists = (typeof content[field] == 'string') || (typeof content[field] == 'number')
                ;
                if(fieldExists) {
                  var text;
                  if (typeof content[field] === 'string'){  
                      text = module.remove.diacritics(content[field]);
                  } else {
                      text = content[field].toString(); 
                  }
                  if( text.search(matchRegExp) !== -1) {
                    // content starts with value (first in results)
                    addResult(results, content);
                  }
                  else if(settings.fullTextSearch === 'exact' && module.exactSearch(searchTerm, text) ) {
                    // content fuzzy matches (last in results)
                    addResult(exactResults, content);
                  }
                  else if(settings.fullTextSearch == true && module.fuzzySearch(searchTerm, text) ) {
                    // content fuzzy matches (last in results)
                    addResult(fuzzyResults, content);
                  }
                }
              });
            });
            $.merge(exactResults, fuzzyResults);
            $.merge(results, exactResults);
            return results;
          }
        },
        exactSearch: function (query, term) {
          query = query.toLowerCase();
          term  = term.toLowerCase();
          return term.indexOf(query) > -1;
        },
        fuzzySearch: function(query, term) {
          var
            termLength  = term.length,
            queryLength = query.length
          ;
          if(typeof query !== 'string') {
            return false;
          }
          query = query.toLowerCase();
          term  = term.toLowerCase();
          if(queryLength > termLength) {
            return false;
          }
          if(queryLength === termLength) {
            return (query === term);
          }
          search: for (var characterIndex = 0, nextCharacterIndex = 0; characterIndex < queryLength; characterIndex++) {
            var
              queryCharacter = query.charCodeAt(characterIndex)
            ;
            while(nextCharacterIndex < termLength) {
              if(term.charCodeAt(nextCharacterIndex++) === queryCharacter) {
                continue search;
              }
            }
            return false;
          }
          return true;
        },

        parse: {
          response: function(response, searchTerm) {
            if(Array.isArray(response)){
                var o={};
                o[fields.results]=response;
                response = o;
            }
            var
              searchHTML = module.generateResults(response)
            ;
            module.verbose('Parsing server response', response);
            if(response !== undefined) {
              if(searchTerm !== undefined && response[fields.results] !== undefined) {
                module.addResults(searchHTML);
                module.inject.id(response[fields.results]);
                module.write.cache(searchTerm, {
                  html    : searchHTML,
                  results : response[fields.results]
                });
                module.save.results(response[fields.results]);
              }
            }
          }
        },

        cancel: {
          query: function() {
            if( module.can.useAPI() ) {
              $module.api('abort');
            }
          }
        },

        has: {
          minimumCharacters: function() {
            var
              searchTerm    = module.get.value(),
              numCharacters = searchTerm.length
            ;
            return (numCharacters >= settings.minCharacters);
          },
          results: function() {
            if($results.length === 0) {
              return false;
            }
            var
              html = $results.html()
            ;
            return html != '';
          }
        },

        clear: {
          cache: function(value) {
            var
              cache = $module.data(metadata.cache)
            ;
            if(!value) {
              module.debug('Clearing cache', value);
              $module.removeData(metadata.cache);
            }
            else if(value && cache && cache[value]) {
              module.debug('Removing value from cache', value);
              delete cache[value];
              $module.data(metadata.cache, cache);
            }
          }
        },

        read: {
          cache: function(name) {
            var
              cache = $module.data(metadata.cache)
            ;
            if(settings.cache) {
              module.verbose('Checking cache for generated html for query', name);
              return (typeof cache == 'object') && (cache[name] !== undefined)
                ? cache[name]
                : false
              ;
            }
            return false;
          }
        },

        create: {
          categoryResults: function(results) {
            var
              categoryResults = {}
            ;
            $.each(results, function(index, result) {
              if(!result.category) {
                return;
              }
              if(categoryResults[result.category] === undefined) {
                module.verbose('Creating new category of results', result.category);
                categoryResults[result.category] = {
                  name    : result.category,
                  results : [result]
                };
              }
              else {
                categoryResults[result.category].results.push(result);
              }
            });
            return categoryResults;
          },
          id: function(resultIndex, categoryIndex) {
            var
              resultID      = (resultIndex + 1), // not zero indexed
              letterID,
              id
            ;
            if(categoryIndex !== undefined) {
              // start char code for "A"
              letterID = String.fromCharCode(97 + categoryIndex);
              id          = letterID + resultID;
              module.verbose('Creating category result id', id);
            }
            else {
              id = resultID;
              module.verbose('Creating result id', id);
            }
            return id;
          },
          results: function() {
            if($results.length === 0) {
              $results = $('<div />')
                .addClass(className.results)
                .appendTo($module)
              ;
            }
          }
        },

        inject: {
          result: function(result, resultIndex, categoryIndex) {
            module.verbose('Injecting result into results');
            var
              $selectedResult = (categoryIndex !== undefined)
                ? $results
                    .children().eq(categoryIndex)
                      .children(selector.results)
                        .first()
                        .children(selector.result)
                          .eq(resultIndex)
                : $results
                    .children(selector.result).eq(resultIndex)
            ;
            module.verbose('Injecting results metadata', $selectedResult);
            $selectedResult
              .data(metadata.result, result)
            ;
          },
          id: function(results) {
            module.debug('Injecting unique ids into results');
            var
              // since results may be object, we must use counters
              categoryIndex = 0,
              resultIndex   = 0
            ;
            if(settings.type === 'category') {
              // iterate through each category result
              $.each(results, function(index, category) {
                if(category.results.length > 0){
                  resultIndex = 0;
                  $.each(category.results, function(index, result) {
                    if(result.id === undefined) {
                      result.id = module.create.id(resultIndex, categoryIndex);
                    }
                    module.inject.result(result, resultIndex, categoryIndex);
                    resultIndex++;
                  });
                  categoryIndex++;
                }
              });
            }
            else {
              // top level
              $.each(results, function(index, result) {
                if(result.id === undefined) {
                  result.id = module.create.id(resultIndex);
                }
                module.inject.result(result, resultIndex);
                resultIndex++;
              });
            }
            return results;
          }
        },

        save: {
          results: function(results) {
            module.verbose('Saving current search results to metadata', results);
            $module.data(metadata.results, results);
          }
        },

        write: {
          cache: function(name, value) {
            var
              cache = ($module.data(metadata.cache) !== undefined)
                ? $module.data(metadata.cache)
                : {}
            ;
            if(settings.cache) {
              module.verbose('Writing generated html to cache', name, value);
              cache[name] = value;
              $module
                .data(metadata.cache, cache)
              ;
            }
          }
        },

        addResults: function(html) {
          if( $.isFunction(settings.onResultsAdd) ) {
            if( settings.onResultsAdd.call($results, html) === false ) {
              module.debug('onResultsAdd callback cancelled default action');
              return false;
            }
          }
          if(html) {
            $results
              .html(html)
            ;
            module.refreshResults();
            if(settings.selectFirstResult) {
              module.select.firstResult();
            }
            module.showResults();
          }
          else {
            module.hideResults(function() {
              $results.empty();
            });
          }
        },

        showResults: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if(resultsDismissed) {
            return;
          }
          if(!module.is.visible() && module.has.results()) {
            if( module.can.transition() ) {
              module.debug('Showing results with css animations');
              $results
                .transition({
                  animation  : settings.transition + ' in',
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  duration   : settings.duration,
                  onComplete : function() {
                    callback();
                  },
                  queue      : true
                })
              ;
            }
            else {
              module.debug('Showing results with javascript');
              $results
                .stop()
                .fadeIn(settings.duration, settings.easing)
              ;
            }
            settings.onResultsOpen.call($results);
          }
        },
        hideResults: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if( module.is.visible() ) {
            if( module.can.transition() ) {
              module.debug('Hiding results with css animations');
              $results
                .transition({
                  animation  : settings.transition + ' out',
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  duration   : settings.duration,
                  onComplete : function() {
                    callback();
                  },
                  queue      : true
                })
              ;
            }
            else {
              module.debug('Hiding results with javascript');
              $results
                .stop()
                .fadeOut(settings.duration, settings.easing)
              ;
            }
            settings.onResultsClose.call($results);
          }
        },

        generateResults: function(response) {
          module.debug('Generating html from response', response);
          var
            template       = settings.templates[settings.type],
            isProperObject = ($.isPlainObject(response[fields.results]) && !$.isEmptyObject(response[fields.results])),
            isProperArray  = (Array.isArray(response[fields.results]) && response[fields.results].length > 0),
            html           = ''
          ;
          if(isProperObject || isProperArray ) {
            if(settings.maxResults > 0) {
              if(isProperObject) {
                if(settings.type == 'standard') {
                  module.error(error.maxResults);
                }
              }
              else {
                response[fields.results] = response[fields.results].slice(0, settings.maxResults);
              }
            }
            if($.isFunction(template)) {
              html = template(response, fields, settings.preserveHTML);
            }
            else {
              module.error(error.noTemplate, false);
            }
          }
          else if(settings.showNoResults) {
            html = module.displayMessage(error.noResults, 'empty', error.noResultsHeader);
          }
          settings.onResults.call(element, response);
          return html;
        },

        displayMessage: function(text, type, header) {
          type = type || 'standard';
          module.debug('Displaying message', text, type, header);
          module.addResults( settings.templates.message(text, type, header) );
          return settings.templates.message(text, type, header);
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
      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }

    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.search.settings = {

  name              : 'Search',
  namespace         : 'search',

  silent            : false,
  debug             : false,
  verbose           : false,
  performance       : true,

  // template to use (specified in settings.templates)
  type              : 'standard',

  // minimum characters required to search
  minCharacters     : 1,

  // whether to select first result after searching automatically
  selectFirstResult : false,

  // API config
  apiSettings       : false,

  // object to search
  source            : false,

  // Whether search should query current term on focus
  searchOnFocus     : true,

  // fields to search
  searchFields   : [
    'id',
    'title',
    'description'
  ],

  // field to display in standard results template
  displayField   : '',

  // search anywhere in value (set to 'exact' to require exact matches
  fullTextSearch : 'exact',

  // match results also if they contain diacritics of the same base character (for example searching for "a" will also match "á" or "â" or "à", etc...)
  ignoreDiacritics : false,

  // whether to add events to prompt automatically
  automatic      : true,

  // delay before hiding menu after blur
  hideDelay      : 0,

  // delay before searching
  searchDelay    : 200,

  // maximum results returned from search
  maxResults     : 7,

  // whether to store lookups in local cache
  cache          : true,

  // whether no results errors should be shown
  showNoResults  : true,

  // preserve possible html of resultset values
  preserveHTML   : true,

  // transition settings
  transition     : 'scale',
  duration       : 200,
  easing         : 'easeOutExpo',

  // callbacks
  onSelect       : false,
  onResultsAdd   : false,

  onSearchQuery  : function(query){},
  onResults      : function(response){},

  onResultsOpen  : function(){},
  onResultsClose : function(){},

  className: {
    animating : 'animating',
    active    : 'active',
    empty     : 'empty',
    focus     : 'focus',
    hidden    : 'hidden',
    loading   : 'loading',
    results   : 'results',
    pressed   : 'down'
  },

  error : {
    source          : 'Cannot search. No source used, and Semantic API module was not included',
    noResultsHeader : 'No Results',
    noResults       : 'Your search returned no results',
    logging         : 'Error in debug logging, exiting.',
    noEndpoint      : 'No search endpoint was specified',
    noTemplate      : 'A valid template name was not specified.',
    oldSearchSyntax : 'searchFullText setting has been renamed fullTextSearch for consistency, please adjust your settings.',
    serverError     : 'There was an issue querying the server.',
    maxResults      : 'Results must be an array to use maxResults setting',
    method          : 'The method you called is not defined.',
    noNormalize     : '"ignoreDiacritics" setting will be ignored. Browser does not support String().normalize(). You may consider including <https://cdn.jsdelivr.net/npm/unorm@1.4.1/lib/unorm.min.js> as a polyfill.'
  },

  metadata: {
    cache   : 'cache',
    results : 'results',
    result  : 'result'
  },

  regExp: {
    escape     : /[\-\[\]\/\{\}\(\)\*\+\?\.\\\^\$\|]/g,
    beginsWith : '(?:\s|^)'
  },

  // maps api response attributes to internal representation
  fields: {
    categories      : 'results',     // array of categories (category view)
    categoryName    : 'name',        // name of category (category view)
    categoryResults : 'results',     // array of results (category view)
    description     : 'description', // result description
    image           : 'image',       // result image
    price           : 'price',       // result price
    results         : 'results',     // array of results (standard)
    title           : 'title',       // result title
    url             : 'url',         // result url
    action          : 'action',      // "view more" object name
    actionText      : 'text',        // "view more" text
    actionURL       : 'url'          // "view more" url
  },

  selector : {
    prompt       : '.prompt',
    searchButton : '.search.button',
    results      : '.results',
    message      : '.results > .message',
    category     : '.category',
    result       : '.result',
    title        : '.title, .name'
  },

  templates: {
    escape: function(string, preserveHTML) {
      if (preserveHTML){
        return string;
      }
      var
        badChars     = /[&<>"'`]/g,
        shouldEscape = /[&<>"'`]/,
        escape       = {
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#x27;",
          "`": "&#x60;"
        },
        escapedChar  = function(chr) {
          return escape[chr];
        }
      ;
      if(shouldEscape.test(string)) {
        return string.replace(badChars, escapedChar);
      }
      return string;
    },
    message: function(message, type, header) {
      var
        html = ''
      ;
      if(message !== undefined && type !== undefined) {
        html +=  ''
          + '<div class="message ' + type + '">'
        ;
        if(header) {
          html += ''
          + '<div class="header">' + header + '</div>'
          ;
        }
        html += ' <div class="description">' + message + '</div>';
        html += '</div>';
      }
      return html;
    },
    category: function(response, fields, preserveHTML) {
      var
        html = '',
        escape = $.fn.search.settings.templates.escape
      ;
      if(response[fields.categoryResults] !== undefined) {

        // each category
        $.each(response[fields.categoryResults], function(index, category) {
          if(category[fields.results] !== undefined && category.results.length > 0) {

            html  += '<div class="category">';

            if(category[fields.categoryName] !== undefined) {
              html += '<div class="name">' + escape(category[fields.categoryName], preserveHTML) + '</div>';
            }

            // each item inside category
            html += '<div class="results">';
            $.each(category.results, function(index, result) {
              if(result[fields.url]) {
                html  += '<a class="result" href="' + result[fields.url].replace(/"/g,"") + '">';
              }
              else {
                html  += '<a class="result">';
              }
              if(result[fields.image] !== undefined) {
                html += ''
                  + '<div class="image">'
                  + ' <img src="' + result[fields.image].replace(/"/g,"") + '">'
                  + '</div>'
                ;
              }
              html += '<div class="content">';
              if(result[fields.price] !== undefined) {
                html += '<div class="price">' + escape(result[fields.price], preserveHTML) + '</div>';
              }
              if(result[fields.title] !== undefined) {
                html += '<div class="title">' + escape(result[fields.title], preserveHTML) + '</div>';
              }
              if(result[fields.description] !== undefined) {
                html += '<div class="description">' + escape(result[fields.description], preserveHTML) + '</div>';
              }
              html  += ''
                + '</div>'
              ;
              html += '</a>';
            });
            html += '</div>';
            html  += ''
              + '</div>'
            ;
          }
        });
        if(response[fields.action]) {
          if(fields.actionURL === false) {
            html += ''
            + '<div class="action">'
            +   escape(response[fields.action][fields.actionText], preserveHTML)
            + '</div>';
          } else {
            html += ''
            + '<a href="' + response[fields.action][fields.actionURL].replace(/"/g,"") + '" class="action">'
            +   escape(response[fields.action][fields.actionText], preserveHTML)
            + '</a>';
          }
        }
        return html;
      }
      return false;
    },
    standard: function(response, fields, preserveHTML) {
      var
        html = '',
        escape = $.fn.search.settings.templates.escape
      ;
      if(response[fields.results] !== undefined) {

        // each result
        $.each(response[fields.results], function(index, result) {
          if(result[fields.url]) {
            html  += '<a class="result" href="' + result[fields.url].replace(/"/g,"") + '">';
          }
          else {
            html  += '<a class="result">';
          }
          if(result[fields.image] !== undefined) {
            html += ''
              + '<div class="image">'
              + ' <img src="' + result[fields.image].replace(/"/g,"") + '">'
              + '</div>'
            ;
          }
          html += '<div class="content">';
          if(result[fields.price] !== undefined) {
            html += '<div class="price">' + escape(result[fields.price], preserveHTML) + '</div>';
          }
          if(result[fields.title] !== undefined) {
            html += '<div class="title">' + escape(result[fields.title], preserveHTML) + '</div>';
          }
          if(result[fields.description] !== undefined) {
            html += '<div class="description">' + escape(result[fields.description], preserveHTML) + '</div>';
          }
          html  += ''
            + '</div>'
          ;
          html += '</a>';
        });
        if(response[fields.action]) {
          if(fields.actionURL === false) {
            html += ''
            + '<div class="action">'
            +   escape(response[fields.action][fields.actionText], preserveHTML)
            + '</div>';
          } else {
            html += ''
            + '<a href="' + response[fields.action][fields.actionURL].replace(/"/g,"") + '" class="action">'
            +   escape(response[fields.action][fields.actionText], preserveHTML)
            + '</a>';
          }
        }
        return html;
      }
      return false;
    }
  }
};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Shape
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

$.fn.shape = function(parameters) {
  var
    $allModules     = $(this),

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    requestAnimationFrame = window.requestAnimationFrame
      || window.mozRequestAnimationFrame
      || window.webkitRequestAnimationFrame
      || window.msRequestAnimationFrame
      || function(callback) { setTimeout(callback, 0); },

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        moduleSelector = $allModules.selector || '',
        settings       = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.shape.settings, parameters)
          : $.extend({}, $.fn.shape.settings),

        // internal aliases
        namespace     = settings.namespace,
        selector      = settings.selector,
        error         = settings.error,
        className     = settings.className,

        // define namespaces for modules
        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        // selector cache
        $module       = $(this),
        $sides        = $module.find('>' + selector.sides),
        $side         = $sides.find('>' + selector.side),

        // private variables
        nextIndex = false,
        $activeSide,
        $nextSide,

        // standard module
        element       = this,
        instance      = $module.data(moduleNamespace),
        module
      ;

      module = {

        initialize: function() {
          module.verbose('Initializing module for', element);
          module.set.defaultSide();
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous module for', element);
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
        },

        refresh: function() {
          module.verbose('Refreshing selector cache for', element);
          $module = $(element);
          $sides  = $(this).find(selector.sides);
          $side   = $(this).find(selector.side);
        },

        repaint: function() {
          module.verbose('Forcing repaint event');
          var
            shape          = $sides[0] || document.createElement('div'),
            fakeAssignment = shape.offsetWidth
          ;
        },

        animate: function(propertyObject, callback) {
          module.verbose('Animating box with properties', propertyObject);
          callback = callback || function(event) {
            module.verbose('Executing animation callback');
            if(event !== undefined) {
              event.stopPropagation();
            }
            module.reset();
            module.set.active();
          };
          settings.beforeChange.call($nextSide[0]);
          if(module.get.transitionEvent()) {
            module.verbose('Starting CSS animation');
            $module
              .addClass(className.animating)
            ;
            $sides
              .css(propertyObject)
              .one(module.get.transitionEvent(), callback)
            ;
            module.set.duration(settings.duration);
            requestAnimationFrame(function() {
              $module
                .addClass(className.animating)
              ;
              $activeSide
                .addClass(className.hidden)
              ;
            });
          }
          else {
            callback();
          }
        },

        queue: function(method) {
          module.debug('Queueing animation of', method);
          $sides
            .one(module.get.transitionEvent(), function() {
              module.debug('Executing queued animation');
              setTimeout(function(){
                $module.shape(method);
              }, 0);
            })
          ;
        },

        reset: function() {
          module.verbose('Animating states reset');
          $module
            .removeClass(className.animating)
            .attr('style', '')
            .removeAttr('style')
          ;
          // removeAttr style does not consistently work in safari
          $sides
            .attr('style', '')
            .removeAttr('style')
          ;
          $side
            .attr('style', '')
            .removeAttr('style')
            .removeClass(className.hidden)
          ;
          $nextSide
            .removeClass(className.animating)
            .attr('style', '')
            .removeAttr('style')
          ;
        },

        is: {
          complete: function() {
            return ($side.filter('.' + className.active)[0] == $nextSide[0]);
          },
          animating: function() {
            return $module.hasClass(className.animating);
          },
          hidden: function() {
            return $module.closest(':hidden').length > 0;
          }
        },

        set: {

          defaultSide: function() {
            $activeSide = $side.filter('.' + settings.className.active);
            $nextSide   = ( $activeSide.next(selector.side).length > 0 )
              ? $activeSide.next(selector.side)
              : $side.first()
            ;
            nextIndex = false;
            module.verbose('Active side set to', $activeSide);
            module.verbose('Next side set to', $nextSide);
          },

          duration: function(duration) {
            duration = duration || settings.duration;
            duration = (typeof duration == 'number')
              ? duration + 'ms'
              : duration
            ;
            module.verbose('Setting animation duration', duration);
            if(settings.duration || settings.duration === 0) {
              $sides.add($side)
                .css({
                  '-webkit-transition-duration': duration,
                  '-moz-transition-duration': duration,
                  '-ms-transition-duration': duration,
                  '-o-transition-duration': duration,
                  'transition-duration': duration
                })
              ;
            }
          },

          currentStageSize: function() {
            var
              $activeSide = $side.filter('.' + settings.className.active),
              width       = $activeSide.outerWidth(true),
              height      = $activeSide.outerHeight(true)
            ;
            $module
              .css({
                width: width,
                height: height
              })
            ;
          },

          stageSize: function() {
            var
              $clone      = $module.clone().addClass(className.loading),
              $side       = $clone.find('>' + selector.sides + '>' + selector.side),
              $activeSide = $side.filter('.' + settings.className.active),
              $nextSide   = (nextIndex)
                ? $side.eq(nextIndex)
                : ( $activeSide.next(selector.side).length > 0 )
                  ? $activeSide.next(selector.side)
                  : $side.first(),
              newWidth    = (settings.width === 'next')
                ? $nextSide.outerWidth(true)
                : (settings.width === 'initial')
                  ? $module.width()
                  : settings.width,
              newHeight    = (settings.height === 'next')
                ? $nextSide.outerHeight(true)
                : (settings.height === 'initial')
                  ? $module.height()
                  : settings.height
            ;
            $activeSide.removeClass(className.active);
            $nextSide.addClass(className.active);
            $clone.insertAfter($module);
            $clone.remove();
            if(settings.width !== 'auto') {
              $module.css('width', newWidth + settings.jitter);
              module.verbose('Specifying width during animation', newWidth);
            }
            if(settings.height !== 'auto') {
              $module.css('height', newHeight + settings.jitter);
              module.verbose('Specifying height during animation', newHeight);
            }
          },

          nextSide: function(selector) {
            nextIndex = selector;
            $nextSide = $side.filter(selector);
            nextIndex = $side.index($nextSide);
            if($nextSide.length === 0) {
              module.set.defaultSide();
              module.error(error.side);
            }
            module.verbose('Next side manually set to', $nextSide);
          },

          active: function() {
            module.verbose('Setting new side to active', $nextSide);
            $side
              .removeClass(className.active)
            ;
            $nextSide
              .addClass(className.active)
            ;
            settings.onChange.call($nextSide[0]);
            module.set.defaultSide();
          }
        },

        flip: {
          to: function(type,stage){
            if(module.is.hidden()) {
              module.debug('Module not visible', $nextSide);
              return;
            }
            if(module.is.complete() && !module.is.animating() && !settings.allowRepeats) {
              module.debug('Side already visible', $nextSide);
              return;
            }
            var
              transform = module.get.transform[type]()
            ;
            if( !module.is.animating()) {
              module.debug('Flipping '+type, $nextSide);
              module.set.stageSize();
              module.stage[stage]();
              module.animate(transform);
            }
            else {
              module.queue('flip '+type);
            }
          },

          up: function() {
            module.flip.to('up','above');
          },

          down: function() {
            module.flip.to('down','below');
          },

          left: function() {
            module.flip.to('left','left');
          },

          right: function() {
            module.flip.to('right','right');
          },

          over: function() {
            module.flip.to('over','behind');
          },

          back: function() {
            module.flip.to('back','behind');
          }

        },

        get: {

          transform: {
            up: function() {
              var
                    translateZ = $activeSide.outerHeight(true) / 2,
                translateY = $nextSide.outerHeight(true) - translateZ
              ;
              return {
                transform: 'translateY(' + translateY + 'px) translateZ(-'+ translateZ + 'px) rotateX(-90deg)'
              };
            },

            down: function() {
              var
                translate = {
                  z: $activeSide.outerHeight(true) / 2
                }
              ;
              return {
                transform: 'translateY(-' + translate.z + 'px) translateZ(-'+ translate.z + 'px) rotateX(90deg)'
              };
            },

            left: function() {
              var
                  translateZ = $activeSide.outerWidth(true) / 2,
                  translateX = $nextSide.outerWidth(true) - translateZ
              ;
              return {
                transform: 'translateX(' + translateX + 'px) translateZ(-' + translateZ + 'px) rotateY(90deg)'
              };
            },

            right: function() {
              var
                translate = {
                  z : $activeSide.outerWidth(true) / 2
                }
              ;
              return {
                transform: 'translateX(-' + translate.z + 'px) translateZ(-' + translate.z + 'px) rotateY(-90deg)'
              };
            },

            over: function() {
              var
                translate = {
                  x : -(($activeSide.outerWidth(true) - $nextSide.outerWidth(true)) / 2)
                }
              ;
              return {
                transform: 'translateX(' + translate.x + 'px) rotateY(180deg)'
              };
            },

            back: function() {
              var
                translate = {
                  x : -(($activeSide.outerWidth(true) - $nextSide.outerWidth(true)) / 2)
                }
              ;
              return {
                transform: 'translateX(' + translate.x + 'px) rotateY(-180deg)'
              };
            }
          },

          transitionEvent: function() {
            var
              element     = document.createElement('element'),
              transitions = {
                'transition'       :'transitionend',
                'OTransition'      :'oTransitionEnd',
                'MozTransition'    :'transitionend',
                'WebkitTransition' :'webkitTransitionEnd'
              },
              transition
            ;
            for(transition in transitions){
              if( element.style[transition] !== undefined ){
                return transitions[transition];
              }
            }
          },

          nextSide: function() {
            return ( $activeSide.next(selector.side).length > 0 )
              ? $activeSide.next(selector.side)
              : $side.first()
            ;
          }

        },

        stage: {

          above: function() {
            var
              box = {
                origin : (($activeSide.outerHeight(true) - $nextSide.outerHeight(true)) / 2),
                depth  : {
                  active : ($nextSide.outerHeight(true) / 2),
                  next   : ($activeSide.outerHeight(true) / 2)
                }
              }
            ;
            module.verbose('Setting the initial animation position as above', $nextSide, box);
            $activeSide
              .css({
                'transform' : 'rotateX(0deg)'
              })
            ;
            $nextSide
              .addClass(className.animating)
              .css({
                'top'       : box.origin + 'px',
                'transform' : 'rotateX(90deg) translateZ(' + box.depth.next + 'px) translateY(-' + box.depth.active + 'px)'
              })
            ;
          },

          below: function() {
            var
              box = {
                origin : (($activeSide.outerHeight(true) - $nextSide.outerHeight(true)) / 2),
                depth  : {
                  active : ($nextSide.outerHeight(true) / 2),
                  next   : ($activeSide.outerHeight(true) / 2)
                }
              }
            ;
            module.verbose('Setting the initial animation position as below', $nextSide, box);
            $activeSide
              .css({
                'transform' : 'rotateX(0deg)'
              })
            ;
            $nextSide
              .addClass(className.animating)
              .css({
                'top'       : box.origin + 'px',
                'transform' : 'rotateX(-90deg) translateZ(' + box.depth.next + 'px) translateY(' + box.depth.active + 'px)'
              })
            ;
          },

          left: function() {
            var
              height = {
                active : $activeSide.outerWidth(true),
                next   : $nextSide.outerWidth(true)
              },
              box = {
                origin : ( ( height.active - height.next ) / 2),
                depth  : {
                  active : (height.next / 2),
                  next   : (height.active / 2)
                }
              }
            ;
            module.verbose('Setting the initial animation position as left', $nextSide, box);
            $activeSide
              .css({
                'transform' : 'rotateY(0deg)'
              })
            ;
            $nextSide
              .addClass(className.animating)
              .css({
                'left'      : box.origin + 'px',
                'transform' : 'rotateY(-90deg) translateZ(' + box.depth.next + 'px) translateX(-' + box.depth.active + 'px)'
              })
            ;
          },

          right: function() {
            var
              height = {
                active : $activeSide.outerWidth(true),
                next   : $nextSide.outerWidth(true)
              },
              box = {
                origin : ( ( height.active - height.next ) / 2),
                depth  : {
                  active : (height.next / 2),
                  next   : (height.active / 2)
                }
              }
            ;
            module.verbose('Setting the initial animation position as right', $nextSide, box);
            $activeSide
              .css({
                'transform' : 'rotateY(0deg)'
              })
            ;
            $nextSide
              .addClass(className.animating)
              .css({
                'left'      : box.origin + 'px',
                'transform' : 'rotateY(90deg) translateZ(' + box.depth.next + 'px) translateX(' + box.depth.active + 'px)'
              })
            ;
          },

          behind: function() {
            var
              height = {
                active : $activeSide.outerWidth(true),
                next   : $nextSide.outerWidth(true)
              },
              box = {
                origin : ( ( height.active - height.next ) / 2),
                depth  : {
                  active : (height.next / 2),
                  next   : (height.active / 2)
                }
              }
            ;
            module.verbose('Setting the initial animation position as behind', $nextSide, box);
            $activeSide
              .css({
                'transform' : 'rotateY(0deg)'
              })
            ;
            $nextSide
              .addClass(className.animating)
              .css({
                'left'      : box.origin + 'px',
                'transform' : 'rotateY(-180deg)'
              })
            ;
          }
        },
        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        var $inputs = $module.find('input');
        if( $inputs.length > 0) {
          $inputs.blur();
          setTimeout(function(){
            module.invoke(query);
          }, 150);
        } else {
          module.invoke(query);
        }
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.shape.settings = {

  // module info
  name : 'Shape',

  // hide all debug content
  silent     : false,

  // debug content outputted to console
  debug      : false,

  // verbose debug output
  verbose    : false,

  // fudge factor in pixels when swapping from 2d to 3d (can be useful to correct rounding errors)
  jitter     : 0,

  // performance data output
  performance: true,

  // event namespace
  namespace  : 'shape',

  // width during animation, can be set to 'auto', initial', 'next' or pixel amount
  width: 'initial',

  // height during animation, can be set to 'auto', 'initial', 'next' or pixel amount
  height: 'initial',

  // callback occurs on side change
  beforeChange : function() {},
  onChange     : function() {},

  // allow animation to same side
  allowRepeats: false,

  // animation duration
  duration   : false,

  // possible errors
  error: {
    side   : 'You tried to switch to a side that does not exist.',
    method : 'The method you called is not defined'
  },

  // classnames used
  className   : {
    animating : 'animating',
    hidden    : 'hidden',
    loading   : 'loading',
    active    : 'active'
  },

  // selectors used
  selector    : {
    sides : '.sides',
    side  : '.side'
  }

};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Sidebar
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

$.fn.sidebar = function(parameters) {
  var
    $allModules     = $(this),
    $window         = $(window),
    $document       = $(document),
    $html           = $('html'),
    $head           = $('head'),

    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    requestAnimationFrame = window.requestAnimationFrame
      || window.mozRequestAnimationFrame
      || window.webkitRequestAnimationFrame
      || window.msRequestAnimationFrame
      || function(callback) { setTimeout(callback, 0); },

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.sidebar.settings, parameters)
          : $.extend({}, $.fn.sidebar.settings),

        selector        = settings.selector,
        className       = settings.className,
        namespace       = settings.namespace,
        regExp          = settings.regExp,
        error           = settings.error,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $module         = $(this),
        $context        = $(settings.context),

        $sidebars       = $module.children(selector.sidebar),
        $fixed          = $context.children(selector.fixed),
        $pusher         = $context.children(selector.pusher),
        $style,

        element         = this,
        instance        = $module.data(moduleNamespace),

        elementNamespace,
        id,
        currentScroll,
        transitionEvent,

        module
      ;

      module      = {

        initialize: function() {
          module.debug('Initializing sidebar', parameters);

          module.create.id();

          transitionEvent = module.get.transitionEvent();

          // avoids locking rendering if initialized in onReady
          if(settings.delaySetup) {
            requestAnimationFrame(module.setup.layout);
          }
          else {
            module.setup.layout();
          }

          requestAnimationFrame(function() {
            module.setup.cache();
          });

          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        create: {
          id: function() {
            id = (Math.random().toString(16) + '000000000').substr(2,8);
            elementNamespace = '.' + id;
            module.verbose('Creating unique id for element', id);
          }
        },

        destroy: function() {
          module.verbose('Destroying previous module for', $module);
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
          if(module.is.ios()) {
            module.remove.ios();
          }
          // bound by uuid
          $context.off(elementNamespace);
          $window.off(elementNamespace);
          $document.off(elementNamespace);
        },

        event: {
          clickaway: function(event) {
            if(settings.closable){
              var
                clickedInPusher = ($pusher.find(event.target).length > 0 || $pusher.is(event.target)),
                clickedContext  = ($context.is(event.target))
              ;
              if(clickedInPusher) {
                module.verbose('User clicked on dimmed page');
                module.hide();
              }
              if(clickedContext) {
                module.verbose('User clicked on dimmable context (scaled out page)');
                module.hide();
              }
            }
          },
          touch: function(event) {
            //event.stopPropagation();
          },
          containScroll: function(event) {
            if(element.scrollTop <= 0)  {
              element.scrollTop = 1;
            }
            if((element.scrollTop + element.offsetHeight) >= element.scrollHeight) {
              element.scrollTop = element.scrollHeight - element.offsetHeight - 1;
            }
          },
          scroll: function(event) {
            if( $(event.target).closest(selector.sidebar).length === 0 ) {
              event.preventDefault();
            }
          }
        },

        bind: {
          clickaway: function() {
            module.verbose('Adding clickaway events to context', $context);
            $context
              .on('click'    + elementNamespace, module.event.clickaway)
              .on('touchend' + elementNamespace, module.event.clickaway)
            ;
          },
          scrollLock: function() {
            if(settings.scrollLock) {
              module.debug('Disabling page scroll');
              $window
                .on('DOMMouseScroll' + elementNamespace, module.event.scroll)
              ;
            }
            module.verbose('Adding events to contain sidebar scroll');
            $document
              .on('touchmove' + elementNamespace, module.event.touch)
            ;
            $module
              .on('scroll' + eventNamespace, module.event.containScroll)
            ;
          }
        },
        unbind: {
          clickaway: function() {
            module.verbose('Removing clickaway events from context', $context);
            $context.off(elementNamespace);
          },
          scrollLock: function() {
            module.verbose('Removing scroll lock from page');
            $document.off(elementNamespace);
            $window.off(elementNamespace);
            $module.off('scroll' + eventNamespace);
          }
        },

        add: {
          inlineCSS: function() {
            var
              width     = module.cache.width  || $module.outerWidth(),
              height    = module.cache.height || $module.outerHeight(),
              isRTL     = module.is.rtl(),
              direction = module.get.direction(),
              distance  = {
                left   : width,
                right  : -width,
                top    : height,
                bottom : -height
              },
              style
            ;

            if(isRTL){
              module.verbose('RTL detected, flipping widths');
              distance.left = -width;
              distance.right = width;
            }

            style  = '<style>';

            if(direction === 'left' || direction === 'right') {
              module.debug('Adding CSS rules for animation distance', width);
              style  += ''
                + ' .ui.visible.' + direction + '.sidebar ~ .fixed,'
                + ' .ui.visible.' + direction + '.sidebar ~ .pusher {'
                + '   -webkit-transform: translate3d('+ distance[direction] + 'px, 0, 0);'
                + '           transform: translate3d('+ distance[direction] + 'px, 0, 0);'
                + ' }'
              ;
            }
            else if(direction === 'top' || direction == 'bottom') {
              style  += ''
                + ' .ui.visible.' + direction + '.sidebar ~ .fixed,'
                + ' .ui.visible.' + direction + '.sidebar ~ .pusher {'
                + '   -webkit-transform: translate3d(0, ' + distance[direction] + 'px, 0);'
                + '           transform: translate3d(0, ' + distance[direction] + 'px, 0);'
                + ' }'
              ;
            }

            /* IE is only browser not to create context with transforms */
            /* https://www.w3.org/Bugs/Public/show_bug.cgi?id=16328 */
            if( module.is.ie() ) {
              if(direction === 'left' || direction === 'right') {
                module.debug('Adding CSS rules for animation distance', width);
                style  += ''
                  + ' body.pushable > .ui.visible.' + direction + '.sidebar ~ .pusher:after {'
                  + '   -webkit-transform: translate3d('+ distance[direction] + 'px, 0, 0);'
                  + '           transform: translate3d('+ distance[direction] + 'px, 0, 0);'
                  + ' }'
                ;
              }
              else if(direction === 'top' || direction == 'bottom') {
                style  += ''
                  + ' body.pushable > .ui.visible.' + direction + '.sidebar ~ .pusher:after {'
                  + '   -webkit-transform: translate3d(0, ' + distance[direction] + 'px, 0);'
                  + '           transform: translate3d(0, ' + distance[direction] + 'px, 0);'
                  + ' }'
                ;
              }
              /* opposite sides visible forces content overlay */
              style += ''
                + ' body.pushable > .ui.visible.left.sidebar ~ .ui.visible.right.sidebar ~ .pusher:after,'
                + ' body.pushable > .ui.visible.right.sidebar ~ .ui.visible.left.sidebar ~ .pusher:after {'
                + '   -webkit-transform: translate3d(0, 0, 0);'
                + '           transform: translate3d(0, 0, 0);'
                + ' }'
              ;
            }
            style += '</style>';
            $style = $(style)
              .appendTo($head)
            ;
            module.debug('Adding sizing css to head', $style);
          }
        },

        refresh: function() {
          module.verbose('Refreshing selector cache');
          $context  = $(settings.context);
          $sidebars = $context.children(selector.sidebar);
          $pusher   = $context.children(selector.pusher);
          $fixed    = $context.children(selector.fixed);
          module.clear.cache();
        },

        refreshSidebars: function() {
          module.verbose('Refreshing other sidebars');
          $sidebars = $context.children(selector.sidebar);
        },

        repaint: function() {
          module.verbose('Forcing repaint event');
          element.style.display = 'none';
          var ignored = element.offsetHeight;
          element.scrollTop = element.scrollTop;
          element.style.display = '';
        },

        setup: {
          cache: function() {
            module.cache = {
              width  : $module.outerWidth(),
              height : $module.outerHeight()
            };
          },
          layout: function() {
            if( $context.children(selector.pusher).length === 0 ) {
              module.debug('Adding wrapper element for sidebar');
              module.error(error.pusher);
              $pusher = $('<div class="pusher" />');
              $context
                .children()
                  .not(selector.omitted)
                  .not($sidebars)
                  .wrapAll($pusher)
              ;
              module.refresh();
            }
            if($module.nextAll(selector.pusher).length === 0 || $module.nextAll(selector.pusher)[0] !== $pusher[0]) {
              module.debug('Moved sidebar to correct parent element');
              module.error(error.movedSidebar, element);
              $module.detach().prependTo($context);
              module.refresh();
            }
            module.clear.cache();
            module.set.pushable();
            module.set.direction();
          }
        },

        attachEvents: function(selector, event) {
          var
            $toggle = $(selector)
          ;
          event = $.isFunction(module[event])
            ? module[event]
            : module.toggle
          ;
          if($toggle.length > 0) {
            module.debug('Attaching sidebar events to element', selector, event);
            $toggle
              .on('click' + eventNamespace, event)
            ;
          }
          else {
            module.error(error.notFound, selector);
          }
        },

        show: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if(module.is.hidden()) {
            module.refreshSidebars();
            if(settings.overlay)  {
              module.error(error.overlay);
              settings.transition = 'overlay';
            }
            module.refresh();
            if(module.othersActive()) {
              module.debug('Other sidebars currently visible');
              if(settings.exclusive) {
                // if not overlay queue animation after hide
                if(settings.transition != 'overlay') {
                  module.hideOthers(module.show);
                  return;
                }
                else {
                  module.hideOthers();
                }
              }
              else {
                settings.transition = 'overlay';
              }
            }
            module.pushPage(function() {
              callback.call(element);
              settings.onShow.call(element);
            });
            settings.onChange.call(element);
            settings.onVisible.call(element);
          }
          else {
            module.debug('Sidebar is already visible');
          }
        },

        hide: function(callback) {
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if(module.is.visible() || module.is.animating()) {
            module.debug('Hiding sidebar', callback);
            module.refreshSidebars();
            module.pullPage(function() {
              callback.call(element);
              settings.onHidden.call(element);
            });
            settings.onChange.call(element);
            settings.onHide.call(element);
          }
        },

        othersAnimating: function() {
          return ($sidebars.not($module).filter('.' + className.animating).length > 0);
        },
        othersVisible: function() {
          return ($sidebars.not($module).filter('.' + className.visible).length > 0);
        },
        othersActive: function() {
          return(module.othersVisible() || module.othersAnimating());
        },

        hideOthers: function(callback) {
          var
            $otherSidebars = $sidebars.not($module).filter('.' + className.visible),
            sidebarCount   = $otherSidebars.length,
            callbackCount  = 0
          ;
          callback = callback || function(){};
          $otherSidebars
            .sidebar('hide', function() {
              callbackCount++;
              if(callbackCount == sidebarCount) {
                callback();
              }
            })
          ;
        },

        toggle: function() {
          module.verbose('Determining toggled direction');
          if(module.is.hidden()) {
            module.show();
          }
          else {
            module.hide();
          }
        },

        pushPage: function(callback) {
          var
            transition = module.get.transition(),
            $transition = (transition === 'overlay' || module.othersActive())
              ? $module
              : $pusher,
            animate,
            dim,
            transitionEnd
          ;
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          if(settings.transition == 'scale down') {
            module.scrollToTop();
          }
          module.set.transition(transition);
          module.repaint();
          animate = function() {
            module.bind.clickaway();
            module.add.inlineCSS();
            module.set.animating();
            module.set.visible();
          };
          dim = function() {
            module.set.dimmed();
          };
          transitionEnd = function(event) {
            if( event.target == $transition[0] ) {
              $transition.off(transitionEvent + elementNamespace, transitionEnd);
              module.remove.animating();
              module.bind.scrollLock();
              callback.call(element);
            }
          };
          $transition.off(transitionEvent + elementNamespace);
          $transition.on(transitionEvent + elementNamespace, transitionEnd);
          requestAnimationFrame(animate);
          if(settings.dimPage && !module.othersVisible()) {
            requestAnimationFrame(dim);
          }
        },

        pullPage: function(callback) {
          var
            transition = module.get.transition(),
            $transition = (transition == 'overlay' || module.othersActive())
              ? $module
              : $pusher,
            animate,
            transitionEnd
          ;
          callback = $.isFunction(callback)
            ? callback
            : function(){}
          ;
          module.verbose('Removing context push state', module.get.direction());

          module.unbind.clickaway();
          module.unbind.scrollLock();

          animate = function() {
            module.set.transition(transition);
            module.set.animating();
            module.remove.visible();
            if(settings.dimPage && !module.othersVisible()) {
              $pusher.removeClass(className.dimmed);
            }
          };
          transitionEnd = function(event) {
            if( event.target == $transition[0] ) {
              $transition.off(transitionEvent + elementNamespace, transitionEnd);
              module.remove.animating();
              module.remove.transition();
              module.remove.inlineCSS();
              if(transition == 'scale down' || (settings.returnScroll && module.is.mobile()) ) {
                module.scrollBack();
              }
              callback.call(element);
            }
          };
          $transition.off(transitionEvent + elementNamespace);
          $transition.on(transitionEvent + elementNamespace, transitionEnd);
          requestAnimationFrame(animate);
        },

        scrollToTop: function() {
          module.verbose('Scrolling to top of page to avoid animation issues');
          currentScroll = $(window).scrollTop();
          $module.scrollTop(0);
          window.scrollTo(0, 0);
        },

        scrollBack: function() {
          module.verbose('Scrolling back to original page position');
          window.scrollTo(0, currentScroll);
        },

        clear: {
          cache: function() {
            module.verbose('Clearing cached dimensions');
            module.cache = {};
          }
        },

        set: {

          // ios only (scroll on html not document). This prevent auto-resize canvas/scroll in ios
          // (This is no longer necessary in latest iOS)
          ios: function() {
            $html.addClass(className.ios);
          },

          // container
          pushed: function() {
            $context.addClass(className.pushed);
          },
          pushable: function() {
            $context.addClass(className.pushable);
          },

          // pusher
          dimmed: function() {
            $pusher.addClass(className.dimmed);
          },

          // sidebar
          active: function() {
            $module.addClass(className.active);
          },
          animating: function() {
            $module.addClass(className.animating);
          },
          transition: function(transition) {
            transition = transition || module.get.transition();
            $module.addClass(transition);
          },
          direction: function(direction) {
            direction = direction || module.get.direction();
            $module.addClass(className[direction]);
          },
          visible: function() {
            $module.addClass(className.visible);
          },
          overlay: function() {
            $module.addClass(className.overlay);
          }
        },
        remove: {

          inlineCSS: function() {
            module.debug('Removing inline css styles', $style);
            if($style && $style.length > 0) {
              $style.remove();
            }
          },

          // ios scroll on html not document
          ios: function() {
            $html.removeClass(className.ios);
          },

          // context
          pushed: function() {
            $context.removeClass(className.pushed);
          },
          pushable: function() {
            $context.removeClass(className.pushable);
          },

          // sidebar
          active: function() {
            $module.removeClass(className.active);
          },
          animating: function() {
            $module.removeClass(className.animating);
          },
          transition: function(transition) {
            transition = transition || module.get.transition();
            $module.removeClass(transition);
          },
          direction: function(direction) {
            direction = direction || module.get.direction();
            $module.removeClass(className[direction]);
          },
          visible: function() {
            $module.removeClass(className.visible);
          },
          overlay: function() {
            $module.removeClass(className.overlay);
          }
        },

        get: {
          direction: function() {
            if($module.hasClass(className.top)) {
              return className.top;
            }
            else if($module.hasClass(className.right)) {
              return className.right;
            }
            else if($module.hasClass(className.bottom)) {
              return className.bottom;
            }
            return className.left;
          },
          transition: function() {
            var
              direction = module.get.direction(),
              transition
            ;
            transition = ( module.is.mobile() )
              ? (settings.mobileTransition == 'auto')
                ? settings.defaultTransition.mobile[direction]
                : settings.mobileTransition
              : (settings.transition == 'auto')
                ? settings.defaultTransition.computer[direction]
                : settings.transition
            ;
            module.verbose('Determined transition', transition);
            return transition;
          },
          transitionEvent: function() {
            var
              element     = document.createElement('element'),
              transitions = {
                'transition'       :'transitionend',
                'OTransition'      :'oTransitionEnd',
                'MozTransition'    :'transitionend',
                'WebkitTransition' :'webkitTransitionEnd'
              },
              transition
            ;
            for(transition in transitions){
              if( element.style[transition] !== undefined ){
                return transitions[transition];
              }
            }
          }
        },

        is: {

          ie: function() {
            var
              isIE11 = (!(window.ActiveXObject) && 'ActiveXObject' in window),
              isIE   = ('ActiveXObject' in window)
            ;
            return (isIE11 || isIE);
          },

          ios: function() {
            var
              userAgent      = navigator.userAgent,
              isIOS          = userAgent.match(regExp.ios),
              isMobileChrome = userAgent.match(regExp.mobileChrome)
            ;
            if(isIOS && !isMobileChrome) {
              module.verbose('Browser was found to be iOS', userAgent);
              return true;
            }
            else {
              return false;
            }
          },
          mobile: function() {
            var
              userAgent    = navigator.userAgent,
              isMobile     = userAgent.match(regExp.mobile)
            ;
            if(isMobile) {
              module.verbose('Browser was found to be mobile', userAgent);
              return true;
            }
            else {
              module.verbose('Browser is not mobile, using regular transition', userAgent);
              return false;
            }
          },
          hidden: function() {
            return !module.is.visible();
          },
          visible: function() {
            return $module.hasClass(className.visible);
          },
          // alias
          open: function() {
            return module.is.visible();
          },
          closed: function() {
            return module.is.hidden();
          },
          vertical: function() {
            return $module.hasClass(className.top);
          },
          animating: function() {
            return $context.hasClass(className.animating);
          },
          rtl: function () {
            if(module.cache.rtl === undefined) {
              module.cache.rtl = $module.attr('dir') === 'rtl' || $module.css('direction') === 'rtl';
            }
            return module.cache.rtl;
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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
      }
    ;

    if(methodInvoked) {
      if(instance === undefined) {
        module.initialize();
      }
      module.invoke(query);
    }
    else {
      if(instance !== undefined) {
        module.invoke('destroy');
      }
      module.initialize();
    }
  });

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.sidebar.settings = {

  name              : 'Sidebar',
  namespace         : 'sidebar',

  silent            : false,
  debug             : false,
  verbose           : false,
  performance       : true,

  transition        : 'auto',
  mobileTransition  : 'auto',

  defaultTransition : {
    computer: {
      left   : 'uncover',
      right  : 'uncover',
      top    : 'overlay',
      bottom : 'overlay'
    },
    mobile: {
      left   : 'uncover',
      right  : 'uncover',
      top    : 'overlay',
      bottom : 'overlay'
    }
  },

  context           : 'body',
  exclusive         : false,
  closable          : true,
  dimPage           : true,
  scrollLock        : false,
  returnScroll      : false,
  delaySetup        : false,

  duration          : 500,

  onChange          : function(){},
  onShow            : function(){},
  onHide            : function(){},

  onHidden          : function(){},
  onVisible         : function(){},

  className         : {
    active    : 'active',
    animating : 'animating',
    dimmed    : 'dimmed',
    ios       : 'ios',
    pushable  : 'pushable',
    pushed    : 'pushed',
    right     : 'right',
    top       : 'top',
    left      : 'left',
    bottom    : 'bottom',
    visible   : 'visible'
  },

  selector: {
    fixed   : '.fixed',
    omitted : 'script, link, style, .ui.modal, .ui.dimmer, .ui.nag, .ui.fixed',
    pusher  : '.pusher',
    sidebar : '.ui.sidebar'
  },

  regExp: {
    ios          : /(iPad|iPhone|iPod)/g,
    mobileChrome : /(CriOS)/g,
    mobile       : /Mobile|iP(hone|od|ad)|Android|BlackBerry|IEMobile|Kindle|NetFront|Silk-Accelerated|(hpw|web)OS|Fennec|Minimo|Opera M(obi|ini)|Blazer|Dolfin|Dolphin|Skyfire|Zune/g
  },

  error   : {
    method       : 'The method you called is not defined.',
    pusher       : 'Had to add pusher element. For optimal performance make sure body content is inside a pusher element',
    movedSidebar : 'Had to move sidebar. For optimal performance make sure sidebar and pusher are direct children of your body tag',
    overlay      : 'The overlay setting is no longer supported, use animation: overlay',
    notFound     : 'There were no elements that matched the specified selector'
  }

};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Sticky
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

$.fn.sticky = function(parameters) {
  var
    $allModules    = $(this),
    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings              = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.sticky.settings, parameters)
          : $.extend({}, $.fn.sticky.settings),

        className             = settings.className,
        namespace             = settings.namespace,
        error                 = settings.error,

        eventNamespace        = '.' + namespace,
        moduleNamespace       = 'module-' + namespace,

        $module               = $(this),
        $window               = $(window),
        $scroll               = $(settings.scrollContext),
        $container,
        $context,

        instance              = $module.data(moduleNamespace),

        requestAnimationFrame = window.requestAnimationFrame
          || window.mozRequestAnimationFrame
          || window.webkitRequestAnimationFrame
          || window.msRequestAnimationFrame
          || function(callback) { setTimeout(callback, 0); },

        element         = this,

        documentObserver,
        observer,
        module
      ;

      module      = {

        initialize: function() {

          module.determineContainer();
          module.determineContext();
          module.verbose('Initializing sticky', settings, $container);

          module.save.positions();
          module.checkErrors();
          module.bind.events();

          if(settings.observeChanges) {
            module.observeChanges();
          }
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous instance');
          module.reset();
          if(documentObserver) {
            documentObserver.disconnect();
          }
          if(observer) {
            observer.disconnect();
          }
          $window
            .off('load' + eventNamespace, module.event.load)
            .off('resize' + eventNamespace, module.event.resize)
          ;
          $scroll
            .off('scrollchange' + eventNamespace, module.event.scrollchange)
          ;
          $module.removeData(moduleNamespace);
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            documentObserver = new MutationObserver(module.event.documentChanged);
            observer         = new MutationObserver(module.event.changed);
            documentObserver.observe(document, {
              childList : true,
              subtree   : true
            });
            observer.observe(element, {
              childList : true,
              subtree   : true
            });
            observer.observe($context[0], {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', observer);
          }
        },

        determineContainer: function() {
          if(settings.container) {
            $container = $(settings.container);
          }
          else {
            $container = $module.offsetParent();
          }
        },

        determineContext: function() {
          if(settings.context) {
            $context = $(settings.context);
          }
          else {
            $context = $container;
          }
          if($context.length === 0) {
            module.error(error.invalidContext, settings.context, $module);
            return;
          }
        },

        checkErrors: function() {
          if( module.is.hidden() ) {
            module.error(error.visible, $module);
          }
          if(module.cache.element.height > module.cache.context.height) {
            module.reset();
            module.error(error.elementSize, $module);
            return;
          }
        },

        bind: {
          events: function() {
            $window
              .on('load' + eventNamespace, module.event.load)
              .on('resize' + eventNamespace, module.event.resize)
            ;
            // pub/sub pattern
            $scroll
              .off('scroll' + eventNamespace)
              .on('scroll' + eventNamespace, module.event.scroll)
              .on('scrollchange' + eventNamespace, module.event.scrollchange)
            ;
          }
        },

        event: {
          changed: function(mutations) {
            clearTimeout(module.timer);
            module.timer = setTimeout(function() {
              module.verbose('DOM tree modified, updating sticky menu', mutations);
              module.refresh();
            }, 100);
          },
          documentChanged: function(mutations) {
            [].forEach.call(mutations, function(mutation) {
              if(mutation.removedNodes) {
                [].forEach.call(mutation.removedNodes, function(node) {
                  if(node == element || $(node).find(element).length > 0) {
                    module.debug('Element removed from DOM, tearing down events');
                    module.destroy();
                  }
                });
              }
            });
          },
          load: function() {
            module.verbose('Page contents finished loading');
            requestAnimationFrame(module.refresh);
          },
          resize: function() {
            module.verbose('Window resized');
            requestAnimationFrame(module.refresh);
          },
          scroll: function() {
            requestAnimationFrame(function() {
              $scroll.triggerHandler('scrollchange' + eventNamespace, $scroll.scrollTop() );
            });
          },
          scrollchange: function(event, scrollPosition) {
            module.stick(scrollPosition);
            settings.onScroll.call(element);
          }
        },

        refresh: function(hardRefresh) {
          module.reset();
          if(!settings.context) {
            module.determineContext();
          }
          if(hardRefresh) {
            module.determineContainer();
          }
          module.save.positions();
          module.stick();
          settings.onReposition.call(element);
        },

        supports: {
          sticky: function() {
            var
              $element = $('<div/>')
            ;
            $element.addClass(className.supported);
            return($element.css('position').match('sticky'));
          }
        },

        save: {
          lastScroll: function(scroll) {
            module.lastScroll = scroll;
          },
          elementScroll: function(scroll) {
            module.elementScroll = scroll;
          },
          positions: function() {
            var
              scrollContext = {
                height : $scroll.height()
              },
              element = {
                margin: {
                  top    : parseInt($module.css('margin-top'), 10),
                  bottom : parseInt($module.css('margin-bottom'), 10),
                },
                offset : $module.offset(),
                width  : $module.outerWidth(),
                height : $module.outerHeight()
              },
              context = {
                offset : $context.offset(),
                height : $context.outerHeight()
              }
            ;
            if( !module.is.standardScroll() ) {
              module.debug('Non-standard scroll. Removing scroll offset from element offset');

              scrollContext.top  = $scroll.scrollTop();
              scrollContext.left = $scroll.scrollLeft();

              element.offset.top  += scrollContext.top;
              context.offset.top  += scrollContext.top;
              element.offset.left += scrollContext.left;
              context.offset.left += scrollContext.left;
            }
            module.cache = {
              fits          : ( (element.height + settings.offset) <= scrollContext.height),
              sameHeight    : (element.height == context.height),
              scrollContext : {
                height : scrollContext.height
              },
              element: {
                margin : element.margin,
                top    : element.offset.top - element.margin.top,
                left   : element.offset.left,
                width  : element.width,
                height : element.height,
                bottom : element.offset.top + element.height
              },
              context: {
                top           : context.offset.top,
                height        : context.height,
                bottom        : context.offset.top + context.height
              }
            };
            module.set.containerSize();

            module.stick();
            module.debug('Caching element positions', module.cache);
          }
        },

        get: {
          direction: function(scroll) {
            var
              direction = 'down'
            ;
            scroll = scroll || $scroll.scrollTop();
            if(module.lastScroll !== undefined) {
              if(module.lastScroll < scroll) {
                direction = 'down';
              }
              else if(module.lastScroll > scroll) {
                direction = 'up';
              }
            }
            return direction;
          },
          scrollChange: function(scroll) {
            scroll = scroll || $scroll.scrollTop();
            return (module.lastScroll)
              ? (scroll - module.lastScroll)
              : 0
            ;
          },
          currentElementScroll: function() {
            if(module.elementScroll) {
              return module.elementScroll;
            }
            return ( module.is.top() )
              ? Math.abs(parseInt($module.css('top'), 10))    || 0
              : Math.abs(parseInt($module.css('bottom'), 10)) || 0
            ;
          },

          elementScroll: function(scroll) {
            scroll = scroll || $scroll.scrollTop();
            var
              element        = module.cache.element,
              scrollContext  = module.cache.scrollContext,
              delta          = module.get.scrollChange(scroll),
              maxScroll      = (element.height - scrollContext.height + settings.offset),
              elementScroll  = module.get.currentElementScroll(),
              possibleScroll = (elementScroll + delta)
            ;
            if(module.cache.fits || possibleScroll < 0) {
              elementScroll = 0;
            }
            else if(possibleScroll > maxScroll ) {
              elementScroll = maxScroll;
            }
            else {
              elementScroll = possibleScroll;
            }
            return elementScroll;
          }
        },

        remove: {
          lastScroll: function() {
            delete module.lastScroll;
          },
          elementScroll: function(scroll) {
            delete module.elementScroll;
          },
          minimumSize: function() {
            $container
              .css('min-height', '')
            ;
          },
          offset: function() {
            $module.css('margin-top', '');
          }
        },

        set: {
          offset: function() {
            module.verbose('Setting offset on element', settings.offset);
            $module
              .css('margin-top', settings.offset)
            ;
          },
          containerSize: function() {
            var
              tagName = $container.get(0).tagName
            ;
            if(tagName === 'HTML' || tagName == 'body') {
              // this can trigger for too many reasons
              //module.error(error.container, tagName, $module);
              module.determineContainer();
            }
            else {
              if( Math.abs($container.outerHeight() - module.cache.context.height) > settings.jitter) {
                module.debug('Context has padding, specifying exact height for container', module.cache.context.height);
                $container.css({
                  height: module.cache.context.height
                });
              }
            }
          },
          minimumSize: function() {
            var
              element   = module.cache.element
            ;
            $container
              .css('min-height', element.height)
            ;
          },
          scroll: function(scroll) {
            module.debug('Setting scroll on element', scroll);
            if(module.elementScroll == scroll) {
              return;
            }
            if( module.is.top() ) {
              $module
                .css('bottom', '')
                .css('top', -scroll)
              ;
            }
            if( module.is.bottom() ) {
              $module
                .css('top', '')
                .css('bottom', scroll)
              ;
            }
          },
          size: function() {
            if(module.cache.element.height !== 0 && module.cache.element.width !== 0) {
              element.style.setProperty('width',  module.cache.element.width  + 'px', 'important');
              element.style.setProperty('height', module.cache.element.height + 'px', 'important');
            }
          }
        },

        is: {
          standardScroll: function() {
            return ($scroll[0] == window);
          },
          top: function() {
            return $module.hasClass(className.top);
          },
          bottom: function() {
            return $module.hasClass(className.bottom);
          },
          initialPosition: function() {
            return (!module.is.fixed() && !module.is.bound());
          },
          hidden: function() {
            return (!$module.is(':visible'));
          },
          bound: function() {
            return $module.hasClass(className.bound);
          },
          fixed: function() {
            return $module.hasClass(className.fixed);
          }
        },

        stick: function(scroll) {
          var
            cachedPosition = scroll || $scroll.scrollTop(),
            cache          = module.cache,
            fits           = cache.fits,
            sameHeight     = cache.sameHeight,
            element        = cache.element,
            scrollContext  = cache.scrollContext,
            context        = cache.context,
            offset         = (module.is.bottom() && settings.pushing)
              ? settings.bottomOffset
              : settings.offset,
            scroll         = {
              top    : cachedPosition + offset,
              bottom : cachedPosition + offset + scrollContext.height
            },
            elementScroll  = (fits)
              ? 0
              : module.get.elementScroll(scroll.top),

            // shorthand
            doesntFit      = !fits,
            elementVisible = (element.height !== 0)
          ;
          if(elementVisible && !sameHeight) {

            if( module.is.initialPosition() ) {
              if(scroll.top >= context.bottom) {
                module.debug('Initial element position is bottom of container');
                module.bindBottom();
              }
              else if(scroll.top > element.top) {
                if( (element.height + scroll.top - elementScroll) >= context.bottom ) {
                  module.debug('Initial element position is bottom of container');
                  module.bindBottom();
                }
                else {
                  module.debug('Initial element position is fixed');
                  module.fixTop();
                }
              }

            }
            else if( module.is.fixed() ) {

              // currently fixed top
              if( module.is.top() ) {
                if( scroll.top <= element.top ) {
                  module.debug('Fixed element reached top of container');
                  module.setInitialPosition();
                }
                else if( (element.height + scroll.top - elementScroll) >= context.bottom ) {
                  module.debug('Fixed element reached bottom of container');
                  module.bindBottom();
                }
                // scroll element if larger than screen
                else if(doesntFit) {
                  module.set.scroll(elementScroll);
                  module.save.lastScroll(scroll.top);
                  module.save.elementScroll(elementScroll);
                }
              }

              // currently fixed bottom
              else if(module.is.bottom() ) {

                // top edge
                if( (scroll.bottom - element.height) <= element.top) {
                  module.debug('Bottom fixed rail has reached top of container');
                  module.setInitialPosition();
                }
                // bottom edge
                else if(scroll.bottom >= context.bottom) {
                  module.debug('Bottom fixed rail has reached bottom of container');
                  module.bindBottom();
                }
                // scroll element if larger than screen
                else if(doesntFit) {
                  module.set.scroll(elementScroll);
                  module.save.lastScroll(scroll.top);
                  module.save.elementScroll(elementScroll);
                }

              }
            }
            else if( module.is.bottom() ) {
              if( scroll.top <= element.top ) {
                module.debug('Jumped from bottom fixed to top fixed, most likely used home/end button');
                module.setInitialPosition();
              }
              else {
                if(settings.pushing) {
                  if(module.is.bound() && scroll.bottom <= context.bottom ) {
                    module.debug('Fixing bottom attached element to bottom of browser.');
                    module.fixBottom();
                  }
                }
                else {
                  if(module.is.bound() && (scroll.top <= context.bottom - element.height) ) {
                    module.debug('Fixing bottom attached element to top of browser.');
                    module.fixTop();
                  }
                }
              }
            }
          }
        },

        bindTop: function() {
          module.debug('Binding element to top of parent container');
          module.remove.offset();
          $module
            .css({
              left         : '',
              top          : '',
              marginBottom : ''
            })
            .removeClass(className.fixed)
            .removeClass(className.bottom)
            .addClass(className.bound)
            .addClass(className.top)
          ;
          settings.onTop.call(element);
          settings.onUnstick.call(element);
        },
        bindBottom: function() {
          module.debug('Binding element to bottom of parent container');
          module.remove.offset();
          $module
            .css({
              left         : '',
              top          : ''
            })
            .removeClass(className.fixed)
            .removeClass(className.top)
            .addClass(className.bound)
            .addClass(className.bottom)
          ;
          settings.onBottom.call(element);
          settings.onUnstick.call(element);
        },

        setInitialPosition: function() {
          module.debug('Returning to initial position');
          module.unfix();
          module.unbind();
        },


        fixTop: function() {
          module.debug('Fixing element to top of page');
          if(settings.setSize) {
            module.set.size();
          }
          module.set.minimumSize();
          module.set.offset();
          $module
            .css({
              left         : module.cache.element.left,
              bottom       : '',
              marginBottom : ''
            })
            .removeClass(className.bound)
            .removeClass(className.bottom)
            .addClass(className.fixed)
            .addClass(className.top)
          ;
          settings.onStick.call(element);
        },

        fixBottom: function() {
          module.debug('Sticking element to bottom of page');
          if(settings.setSize) {
            module.set.size();
          }
          module.set.minimumSize();
          module.set.offset();
          $module
            .css({
              left         : module.cache.element.left,
              bottom       : '',
              marginBottom : ''
            })
            .removeClass(className.bound)
            .removeClass(className.top)
            .addClass(className.fixed)
            .addClass(className.bottom)
          ;
          settings.onStick.call(element);
        },

        unbind: function() {
          if( module.is.bound() ) {
            module.debug('Removing container bound position on element');
            module.remove.offset();
            $module
              .removeClass(className.bound)
              .removeClass(className.top)
              .removeClass(className.bottom)
            ;
          }
        },

        unfix: function() {
          if( module.is.fixed() ) {
            module.debug('Removing fixed position on element');
            module.remove.minimumSize();
            module.remove.offset();
            $module
              .removeClass(className.fixed)
              .removeClass(className.top)
              .removeClass(className.bottom)
            ;
            settings.onUnstick.call(element);
          }
        },

        reset: function() {
          module.debug('Resetting elements position');
          module.unbind();
          module.unfix();
          module.resetCSS();
          module.remove.offset();
          module.remove.lastScroll();
        },

        resetCSS: function() {
          $module
            .css({
              width  : '',
              height : ''
            })
          ;
          $container
            .css({
              height: ''
            })
          ;
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
            module.performance.timer = setTimeout(module.performance.display, 0);
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
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.sticky.settings = {

  name           : 'Sticky',
  namespace      : 'sticky',

  silent         : false,
  debug          : false,
  verbose        : true,
  performance    : true,

  // whether to stick in the opposite direction on scroll up
  pushing        : false,

  context        : false,
  container      : false,

  // Context to watch scroll events
  scrollContext  : window,

  // Offset to adjust scroll
  offset         : 0,

  // Offset to adjust scroll when attached to bottom of screen
  bottomOffset   : 0,

  // will only set container height if difference between context and container is larger than this number
  jitter         : 5,

  // set width of sticky element when it is fixed to page (used to make sure 100% width is maintained if no fixed size set)
  setSize        : true,

  // Whether to automatically observe changes with Mutation Observers
  observeChanges : false,

  // Called when position is recalculated
  onReposition   : function(){},

  // Called on each scroll
  onScroll       : function(){},

  // Called when element is stuck to viewport
  onStick        : function(){},

  // Called when element is unstuck from viewport
  onUnstick      : function(){},

  // Called when element reaches top of context
  onTop          : function(){},

  // Called when element reaches bottom of context
  onBottom       : function(){},

  error         : {
    container      : 'Sticky element must be inside a relative container',
    visible        : 'Element is hidden, you must call refresh after element becomes visible. Use silent setting to surpress this warning in production.',
    method         : 'The method you called is not defined.',
    invalidContext : 'Context specified does not exist',
    elementSize    : 'Sticky element is larger than its container, cannot create sticky.'
  },

  className : {
    bound     : 'bound',
    fixed     : 'fixed',
    supported : 'native',
    top       : 'top',
    bottom    : 'bottom'
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Tab
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

'use strict';

$.isWindow = $.isWindow || function(obj) {
  return obj != null && obj === obj.window;
};
$.isFunction = $.isFunction || function(obj) {
  return typeof obj === "function" && typeof obj.nodeType !== "number";
};

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.tab = function(parameters) {

  var
    // use window context if none specified
    $allModules     = $.isFunction(this)
        ? $(window)
        : $(this),

    moduleSelector  = $allModules.selector || '',
    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    initializedHistory = false,
    returnedValue
  ;

  $allModules
    .each(function() {
      var

        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.tab.settings, parameters)
          : $.extend({}, $.fn.tab.settings),

        className       = settings.className,
        metadata        = settings.metadata,
        selector        = settings.selector,
        error           = settings.error,
        regExp          = settings.regExp,

        eventNamespace  = '.' + settings.namespace,
        moduleNamespace = 'module-' + settings.namespace,

        $module         = $(this),
        $context,
        $tabs,

        cache           = {},
        firstLoad       = true,
        recursionDepth  = 0,
        element         = this,
        instance        = $module.data(moduleNamespace),

        activeTabPath,
        parameterArray,
        module,

        historyEvent

      ;

      module = {

        initialize: function() {
          module.debug('Initializing tab menu item', $module);
          module.fix.callbacks();
          module.determineTabs();

          module.debug('Determining tabs', settings.context, $tabs);
          // set up automatic routing
          if(settings.auto) {
            module.set.auto();
          }
          module.bind.events();

          if(settings.history && !initializedHistory) {
            module.initializeHistory();
            initializedHistory = true;
          }

          if(instance === undefined && module.determine.activeTab() == null) {
            module.debug('No active tab detected, setting first tab active', module.get.initialPath());
            module.changeTab(module.get.initialPath());
          };

          module.instantiate();
        },

        instantiate: function () {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, module)
          ;
        },

        destroy: function() {
          module.debug('Destroying tabs', $module);
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
        },

        bind: {
          events: function() {
            // if using $.tab don't add events
            if( !$.isWindow( element ) ) {
              module.debug('Attaching tab activation events to element', $module);
              $module
                .on('click' + eventNamespace, module.event.click)
              ;
            }
          }
        },

        determineTabs: function() {
          var
            $reference
          ;

          // determine tab context
          if(settings.context === 'parent') {
            if($module.closest(selector.ui).length > 0) {
              $reference = $module.closest(selector.ui);
              module.verbose('Using closest UI element as parent', $reference);
            }
            else {
              $reference = $module;
            }
            $context = $reference.parent();
            module.verbose('Determined parent element for creating context', $context);
          }
          else if(settings.context) {
            $context = $(settings.context);
            module.verbose('Using selector for tab context', settings.context, $context);
          }
          else {
            $context = $('body');
          }
          // find tabs
          if(settings.childrenOnly) {
            $tabs = $context.children(selector.tabs);
            module.debug('Searching tab context children for tabs', $context, $tabs);
          }
          else {
            $tabs = $context.find(selector.tabs);
            module.debug('Searching tab context for tabs', $context, $tabs);
          }
        },

        fix: {
          callbacks: function() {
            if( $.isPlainObject(parameters) && (parameters.onTabLoad || parameters.onTabInit) ) {
              if(parameters.onTabLoad) {
                parameters.onLoad = parameters.onTabLoad;
                delete parameters.onTabLoad;
                module.error(error.legacyLoad, parameters.onLoad);
              }
              if(parameters.onTabInit) {
                parameters.onFirstLoad = parameters.onTabInit;
                delete parameters.onTabInit;
                module.error(error.legacyInit, parameters.onFirstLoad);
              }
              settings = $.extend(true, {}, $.fn.tab.settings, parameters);
            }
          }
        },

        initializeHistory: function() {
          module.debug('Initializing page state');
          if( $.address === undefined ) {
            module.error(error.state);
            return false;
          }
          else {
            if(settings.historyType == 'state') {
              module.debug('Using HTML5 to manage state');
              if(settings.path !== false) {
                $.address
                  .history(true)
                  .state(settings.path)
                ;
              }
              else {
                module.error(error.path);
                return false;
              }
            }
            $.address
              .bind('change', module.event.history.change)
            ;
          }
        },

        event: {
          click: function(event) {
            var
              tabPath = $(this).data(metadata.tab)
            ;
            if(tabPath !== undefined) {
              if(settings.history) {
                module.verbose('Updating page state', event);
                $.address.value(tabPath);
              }
              else {
                module.verbose('Changing tab', event);
                module.changeTab(tabPath);
              }
              event.preventDefault();
            }
            else {
              module.debug('No tab specified');
            }
          },
          history: {
            change: function(event) {
              var
                tabPath   = event.pathNames.join('/') || module.get.initialPath(),
                pageTitle = settings.templates.determineTitle(tabPath) || false
              ;
              module.performance.display();
              module.debug('History change event', tabPath, event);
              historyEvent = event;
              if(tabPath !== undefined) {
                module.changeTab(tabPath);
              }
              if(pageTitle) {
                $.address.title(pageTitle);
              }
            }
          }
        },

        refresh: function() {
          if(activeTabPath) {
            module.debug('Refreshing tab', activeTabPath);
            module.changeTab(activeTabPath);
          }
        },

        cache: {

          read: function(cacheKey) {
            return (cacheKey !== undefined)
              ? cache[cacheKey]
              : false
            ;
          },
          add: function(cacheKey, content) {
            cacheKey = cacheKey || activeTabPath;
            module.debug('Adding cached content for', cacheKey);
            cache[cacheKey] = content;
          },
          remove: function(cacheKey) {
            cacheKey = cacheKey || activeTabPath;
            module.debug('Removing cached content for', cacheKey);
            delete cache[cacheKey];
          }
        },

        escape: {
          string: function(text) {
            text =  String(text);
            return text.replace(regExp.escape, '\\$&');
          }
        },

        set: {
          auto: function() {
            var
              url = (typeof settings.path == 'string')
                ? settings.path.replace(/\/$/, '') + '/{$tab}'
                : '/{$tab}'
            ;
            module.verbose('Setting up automatic tab retrieval from server', url);
            if($.isPlainObject(settings.apiSettings)) {
              settings.apiSettings.url = url;
            }
            else {
              settings.apiSettings = {
                url: url
              };
            }
          },
          loading: function(tabPath) {
            var
              $tab      = module.get.tabElement(tabPath),
              isLoading = $tab.hasClass(className.loading)
            ;
            if(!isLoading) {
              module.verbose('Setting loading state for', $tab);
              $tab
                .addClass(className.loading)
                .siblings($tabs)
                  .removeClass(className.active + ' ' + className.loading)
              ;
              if($tab.length > 0) {
                settings.onRequest.call($tab[0], tabPath);
              }
            }
          },
          state: function(state) {
            $.address.value(state);
          }
        },

        changeTab: function(tabPath) {
          var
            pushStateAvailable = (window.history && window.history.pushState),
            shouldIgnoreLoad   = (pushStateAvailable && settings.ignoreFirstLoad && firstLoad),
            remoteContent      = (settings.auto || $.isPlainObject(settings.apiSettings) ),
            // only add default path if not remote content
            pathArray = (remoteContent && !shouldIgnoreLoad)
              ? module.utilities.pathToArray(tabPath)
              : module.get.defaultPathArray(tabPath)
          ;
          tabPath = module.utilities.arrayToPath(pathArray);
          $.each(pathArray, function(index, tab) {
            var
              currentPathArray   = pathArray.slice(0, index + 1),
              currentPath        = module.utilities.arrayToPath(currentPathArray),

              isTab              = module.is.tab(currentPath),
              isLastIndex        = (index + 1 == pathArray.length),

              $tab               = module.get.tabElement(currentPath),
              $anchor,
              nextPathArray,
              nextPath,
              isLastTab
            ;
            module.verbose('Looking for tab', tab);
            if(isTab) {
              module.verbose('Tab was found', tab);
              // scope up
              activeTabPath  = currentPath;
              parameterArray = module.utilities.filterArray(pathArray, currentPathArray);

              if(isLastIndex) {
                isLastTab = true;
              }
              else {
                nextPathArray = pathArray.slice(0, index + 2);
                nextPath      = module.utilities.arrayToPath(nextPathArray);
                isLastTab     = ( !module.is.tab(nextPath) );
                if(isLastTab) {
                  module.verbose('Tab parameters found', nextPathArray);
                }
              }
              if(isLastTab && remoteContent) {
                if(!shouldIgnoreLoad) {
                  module.activate.navigation(currentPath);
                  module.fetch.content(currentPath, tabPath);
                }
                else {
                  module.debug('Ignoring remote content on first tab load', currentPath);
                  firstLoad = false;
                  module.cache.add(tabPath, $tab.html());
                  module.activate.all(currentPath);
                  settings.onFirstLoad.call($tab[0], currentPath, parameterArray, historyEvent);
                  settings.onLoad.call($tab[0], currentPath, parameterArray, historyEvent);
                }
                return false;
              }
              else {
                module.debug('Opened local tab', currentPath);
                module.activate.all(currentPath);
                if( !module.cache.read(currentPath) ) {
                  module.cache.add(currentPath, true);
                  module.debug('First time tab loaded calling tab init');
                  settings.onFirstLoad.call($tab[0], currentPath, parameterArray, historyEvent);
                }
                settings.onLoad.call($tab[0], currentPath, parameterArray, historyEvent);
              }

            }
            else if(tabPath.search('/') == -1 && tabPath !== '') {
              // look for in page anchor
              tabPath = module.escape.string(tabPath);
              $anchor     = $('#' + tabPath + ', a[name="' + tabPath + '"]');
              currentPath = $anchor.closest('[data-tab]').data(metadata.tab);
              $tab        = module.get.tabElement(currentPath);
              // if anchor exists use parent tab
              if($anchor && $anchor.length > 0 && currentPath) {
                module.debug('Anchor link used, opening parent tab', $tab, $anchor);
                if( !$tab.hasClass(className.active) ) {
                  setTimeout(function() {
                    module.scrollTo($anchor);
                  }, 0);
                }
                module.activate.all(currentPath);
                if( !module.cache.read(currentPath) ) {
                  module.cache.add(currentPath, true);
                  module.debug('First time tab loaded calling tab init');
                  settings.onFirstLoad.call($tab[0], currentPath, parameterArray, historyEvent);
                }
                settings.onLoad.call($tab[0], currentPath, parameterArray, historyEvent);
                return false;
              }
            }
            else {
              module.error(error.missingTab, $module, $context, currentPath);
              return false;
            }
          });
        },

        scrollTo: function($element) {
          var
            scrollOffset = ($element && $element.length > 0)
              ? $element.offset().top
              : false
          ;
          if(scrollOffset !== false) {
            module.debug('Forcing scroll to an in-page link in a hidden tab', scrollOffset, $element);
            $(document).scrollTop(scrollOffset);
          }
        },

        update: {
          content: function(tabPath, html, evaluateScripts) {
            var
              $tab = module.get.tabElement(tabPath),
              tab  = $tab[0]
            ;
            evaluateScripts = (evaluateScripts !== undefined)
              ? evaluateScripts
              : settings.evaluateScripts
            ;
            if(typeof settings.cacheType == 'string' && settings.cacheType.toLowerCase() == 'dom' && typeof html !== 'string') {
              $tab
                .empty()
                .append($(html).clone(true))
              ;
            }
            else {
              if(evaluateScripts) {
                module.debug('Updating HTML and evaluating inline scripts', tabPath, html);
                $tab.html(html);
              }
              else {
                module.debug('Updating HTML', tabPath, html);
                tab.innerHTML = html;
              }
            }
          }
        },

        fetch: {

          content: function(tabPath, fullTabPath) {
            var
              $tab        = module.get.tabElement(tabPath),
              apiSettings = {
                dataType         : 'html',
                encodeParameters : false,
                on               : 'now',
                cache            : settings.alwaysRefresh,
                headers          : {
                  'X-Remote': true
                },
                onSuccess : function(response) {
                  if(settings.cacheType == 'response') {
                    module.cache.add(fullTabPath, response);
                  }
                  module.update.content(tabPath, response);
                  if(tabPath == activeTabPath) {
                    module.debug('Content loaded', tabPath);
                    module.activate.tab(tabPath);
                  }
                  else {
                    module.debug('Content loaded in background', tabPath);
                  }
                  settings.onFirstLoad.call($tab[0], tabPath, parameterArray, historyEvent);
                  settings.onLoad.call($tab[0], tabPath, parameterArray, historyEvent);

                  if(settings.loadOnce) {
                    module.cache.add(fullTabPath, true);
                  }
                  else if(typeof settings.cacheType == 'string' && settings.cacheType.toLowerCase() == 'dom' && $tab.children().length > 0) {
                    setTimeout(function() {
                      var
                        $clone = $tab.children().clone(true)
                      ;
                      $clone = $clone.not('script');
                      module.cache.add(fullTabPath, $clone);
                    }, 0);
                  }
                  else {
                    module.cache.add(fullTabPath, $tab.html());
                  }
                },
                urlData: {
                  tab: fullTabPath
                }
              },
              request         = $tab.api('get request') || false,
              existingRequest = ( request && request.state() === 'pending' ),
              requestSettings,
              cachedContent
            ;

            fullTabPath   = fullTabPath || tabPath;
            cachedContent = module.cache.read(fullTabPath);


            if(settings.cache && cachedContent) {
              module.activate.tab(tabPath);
              module.debug('Adding cached content', fullTabPath);
              if(!settings.loadOnce) {
                if(settings.evaluateScripts == 'once') {
                  module.update.content(tabPath, cachedContent, false);
                }
                else {
                  module.update.content(tabPath, cachedContent);
                }
              }
              settings.onLoad.call($tab[0], tabPath, parameterArray, historyEvent);
            }
            else if(existingRequest) {
              module.set.loading(tabPath);
              module.debug('Content is already loading', fullTabPath);
            }
            else if($.api !== undefined) {
              requestSettings = $.extend(true, {}, settings.apiSettings, apiSettings);
              module.debug('Retrieving remote content', fullTabPath, requestSettings);
              module.set.loading(tabPath);
              $tab.api(requestSettings);
            }
            else {
              module.error(error.api);
            }
          }
        },

        activate: {
          all: function(tabPath) {
            module.activate.tab(tabPath);
            module.activate.navigation(tabPath);
          },
          tab: function(tabPath) {
            var
              $tab          = module.get.tabElement(tabPath),
              $deactiveTabs = (settings.deactivate == 'siblings')
                ? $tab.siblings($tabs)
                : $tabs.not($tab),
              isActive      = $tab.hasClass(className.active)
            ;
            module.verbose('Showing tab content for', $tab);
            if(!isActive) {
              $tab
                .addClass(className.active)
              ;
              $deactiveTabs
                .removeClass(className.active + ' ' + className.loading)
              ;
              if($tab.length > 0) {
                settings.onVisible.call($tab[0], tabPath);
              }
            }
          },
          navigation: function(tabPath) {
            var
              $navigation         = module.get.navElement(tabPath),
              $deactiveNavigation = (settings.deactivate == 'siblings')
                ? $navigation.siblings($allModules)
                : $allModules.not($navigation),
              isActive    = $navigation.hasClass(className.active)
            ;
            module.verbose('Activating tab navigation for', $navigation, tabPath);
            if(!isActive) {
              $navigation
                .addClass(className.active)
              ;
              $deactiveNavigation
                .removeClass(className.active + ' ' + className.loading)
              ;
            }
          }
        },

        deactivate: {
          all: function() {
            module.deactivate.navigation();
            module.deactivate.tabs();
          },
          navigation: function() {
            $allModules
              .removeClass(className.active)
            ;
          },
          tabs: function() {
            $tabs
              .removeClass(className.active + ' ' + className.loading)
            ;
          }
        },

        is: {
          tab: function(tabName) {
            return (tabName !== undefined)
              ? ( module.get.tabElement(tabName).length > 0 )
              : false
            ;
          }
        },

        get: {
          initialPath: function() {
            return $allModules.eq(0).data(metadata.tab) || $tabs.eq(0).data(metadata.tab);
          },
          path: function() {
            return $.address.value();
          },
          // adds default tabs to tab path
          defaultPathArray: function(tabPath) {
            return module.utilities.pathToArray( module.get.defaultPath(tabPath) );
          },
          defaultPath: function(tabPath) {
            var
              $defaultNav = $allModules.filter('[data-' + metadata.tab + '^="' + module.escape.string(tabPath) + '/"]').eq(0),
              defaultTab  = $defaultNav.data(metadata.tab) || false
            ;
            if( defaultTab ) {
              module.debug('Found default tab', defaultTab);
              if(recursionDepth < settings.maxDepth) {
                recursionDepth++;
                return module.get.defaultPath(defaultTab);
              }
              module.error(error.recursion);
            }
            else {
              module.debug('No default tabs found for', tabPath, $tabs);
            }
            recursionDepth = 0;
            return tabPath;
          },
          navElement: function(tabPath) {
            tabPath = tabPath || activeTabPath;
            return $allModules.filter('[data-' + metadata.tab + '="' + module.escape.string(tabPath) + '"]');
          },
          tabElement: function(tabPath) {
            var
              $fullPathTab,
              $simplePathTab,
              tabPathArray,
              lastTab
            ;
            tabPath        = tabPath || activeTabPath;
            tabPathArray   = module.utilities.pathToArray(tabPath);
            lastTab        = module.utilities.last(tabPathArray);
            $fullPathTab   = $tabs.filter('[data-' + metadata.tab + '="' + module.escape.string(tabPath) + '"]');
            $simplePathTab = $tabs.filter('[data-' + metadata.tab + '="' + module.escape.string(lastTab) + '"]');
            return ($fullPathTab.length > 0)
              ? $fullPathTab
              : $simplePathTab
            ;
          },
          tab: function() {
            return activeTabPath;
          }
        },

        determine: {
          activeTab: function() {
            var activeTab = null;

            $tabs.each(function(_index, tab) {
              var $tab = $(tab);

              if( $tab.hasClass(className.active) ) {
                var
                  tabPath = $(this).data(metadata.tab),
                  $anchor = $allModules.filter('[data-' + metadata.tab + '="' + module.escape.string(tabPath) + '"]')
                ;

                if( $anchor.hasClass(className.active) ) {
                  activeTab = tabPath;
                }
              }
            });

            return activeTab;
          }
        },

        utilities: {
          filterArray: function(keepArray, removeArray) {
            return $.grep(keepArray, function(keepValue) {
              return ( $.inArray(keepValue, removeArray) == -1);
            });
          },
          last: function(array) {
            return Array.isArray(array)
              ? array[ array.length - 1]
              : false
            ;
          },
          pathToArray: function(pathName) {
            if(pathName === undefined) {
              pathName = activeTabPath;
            }
            return typeof pathName == 'string'
              ? pathName.split('/')
              : [pathName]
            ;
          },
          arrayToPath: function(pathArray) {
            return Array.isArray(pathArray)
              ? pathArray.join('/')
              : false
            ;
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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
      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;
  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;

};

// shortcut for tabbed content with no defined navigation
$.tab = function() {
  $(window).tab.apply(this, arguments);
};

$.fn.tab.settings = {

  name            : 'Tab',
  namespace       : 'tab',

  silent          : false,
  debug           : false,
  verbose         : false,
  performance     : true,

  auto            : false,      // uses pjax style endpoints fetching content from same url with remote-content headers
  history         : false,      // use browser history
  historyType     : 'hash',     // #/ or html5 state
  path            : false,      // base path of url

  context         : false,      // specify a context that tabs must appear inside
  childrenOnly    : false,      // use only tabs that are children of context
  maxDepth        : 25,         // max depth a tab can be nested

  deactivate      : 'siblings', // whether tabs should deactivate sibling menu elements or all elements initialized together

  alwaysRefresh   : false,      // load tab content new every tab click
  cache           : true,       // cache the content requests to pull locally
  loadOnce        : false,      // Whether tab data should only be loaded once when using remote content
  cacheType       : 'response', // Whether to cache exact response, or to html cache contents after scripts execute
  ignoreFirstLoad : false,      // don't load remote content on first load

  apiSettings     : false,      // settings for api call
  evaluateScripts : 'once',     // whether inline scripts should be parsed (true/false/once). Once will not re-evaluate on cached content

  onFirstLoad : function(tabPath, parameterArray, historyEvent) {}, // called first time loaded
  onLoad      : function(tabPath, parameterArray, historyEvent) {}, // called on every load
  onVisible   : function(tabPath, parameterArray, historyEvent) {}, // called every time tab visible
  onRequest   : function(tabPath, parameterArray, historyEvent) {}, // called ever time a tab beings loading remote content

  templates : {
    determineTitle: function(tabArray) {} // returns page title for path
  },

  error: {
    api        : 'You attempted to load content without API module',
    method     : 'The method you called is not defined',
    missingTab : 'Activated tab cannot be found. Tabs are case-sensitive.',
    noContent  : 'The tab you specified is missing a content url.',
    path       : 'History enabled, but no path was specified',
    recursion  : 'Max recursive depth reached',
    legacyInit : 'onTabInit has been renamed to onFirstLoad in 2.0, please adjust your code.',
    legacyLoad : 'onTabLoad has been renamed to onLoad in 2.0. Please adjust your code',
    state      : 'History requires Asual\'s Address library <https://github.com/asual/jquery-address>'
  },

  regExp : {
    escape   : /[-[\]{}()*+?.,\\^$|#\s:=@]/g
  },

  metadata : {
    tab    : 'tab',
    loaded : 'loaded',
    promise: 'promise'
  },

  className   : {
    loading : 'loading',
    active  : 'active'
  },

  selector    : {
    tabs : '.ui.tab',
    ui   : '.ui'
  }

};

})( jQuery, window, document );

/*!
 * # Fomantic-UI - Toast
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

$.fn.toast = function(parameters) {
  var
    $allModules    = $(this),
    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.toast.settings, parameters)
          : $.extend({}, $.fn.toast.settings),

        className        = settings.className,
        selector         = settings.selector,
        error            = settings.error,
        namespace        = settings.namespace,
        fields           = settings.fields,

        eventNamespace   = '.' + namespace,
        moduleNamespace  = namespace + '-module',

        $module          = $(this),
        $toastBox,
        $toast,
        $actions,
        $progress,
        $progressBar,
        $animationObject,
        $close,
        $context         = (settings.context)
          ? $(settings.context)
          : $('body'),

        isToastComponent = $module.hasClass('toast') || $module.hasClass('message') || $module.hasClass('card'),

        element          = this,
        instance         = isToastComponent ? $module.data(moduleNamespace) : undefined,

        module
      ;
      module = {

        initialize: function() {
          module.verbose('Initializing element');
          if (!module.has.container()) {
            module.create.container();
          }
          if(isToastComponent || settings.message !== '' || settings.title !== '' || module.get.iconClass() !== '' || settings.showImage || module.has.configActions()) {
            if(typeof settings.showProgress !== 'string' || [className.top,className.bottom].indexOf(settings.showProgress) === -1 ) {
              settings.showProgress = false;
            }
            module.create.toast();
            if(settings.closeOnClick && (settings.closeIcon || $($toast).find(selector.input).length > 0 || module.has.configActions())){
              settings.closeOnClick = false;
            }
            if(!settings.closeOnClick) {
              $toastBox.addClass(className.unclickable);
            }
            module.bind.events();
          }
          module.instantiate();
          if($toastBox) {
            module.show();
          }
        },

        instantiate: function() {
          module.verbose('Storing instance of toast');
          instance = module;
          $module
              .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          if($toastBox) {
            module.debug('Removing toast', $toastBox);
            module.unbind.events();
            $toastBox.remove();
            $toastBox = undefined;
            $toast = undefined;
            $animationObject = undefined;
            settings.onRemove.call($toastBox, element);
            $progress = undefined;
            $progressBar = undefined;
            $close = undefined;
          }
          $module
            .removeData(moduleNamespace)
          ;
        },

        show: function(callback) {
          callback = callback || function(){};
          module.debug('Showing toast');
          if(settings.onShow.call($toastBox, element) === false) {
            module.debug('onShow callback returned false, cancelling toast animation');
            return;
          }
          module.animate.show(callback);
        },

        close: function(callback) {
          callback = callback || function(){};
          module.remove.visible();
          module.unbind.events();
          module.animate.close(callback);

        },

        create: {
          container: function() {
            module.verbose('Creating container');
            $context.append($('<div/>',{class: settings.position + ' ' + className.container}));
          },
          toast: function() {
            $toastBox = $('<div/>', {class: className.box});
            if (!isToastComponent) {
              module.verbose('Creating toast');
              $toast = $('<div/>');
              var $content = $('<div/>', {class: className.content});
              var iconClass = module.get.iconClass();
              if (iconClass !== '') {
                $toast.append($('<i/>', {class: iconClass + ' ' + className.icon}));
              }

              if (settings.showImage) {
                $toast.append($('<img>', {
                  class: className.image + ' ' + settings.classImage,
                  src: settings.showImage
                }));
              }
              if (settings.title !== '') {
                $content.append($('<div/>', {
                  class: className.title,
                  text: settings.title
                }));
              }

              $content.append($('<div/>', {html: module.helpers.escape(settings.message, settings.preserveHTML)}));

              $toast
                .addClass(settings.class + ' ' + className.toast)
                .append($content)
              ;
              $toast.css('opacity', settings.opacity);
              if (settings.closeIcon) {
                $close = $('<i/>', {class: className.close + ' ' + (typeof settings.closeIcon === 'string' ? settings.closeIcon : '')});
                if($close.hasClass(className.left)) {
                  $toast.prepend($close);
                } else {
                  $toast.append($close);
                }
              }
            } else {
              $toast = settings.cloneModule ? $module.clone().removeAttr('id') : $module;
              $close = $toast.find('> i'+module.helpers.toClass(className.close));
              settings.closeIcon = ($close.length > 0);
            }
            if ($toast.hasClass(className.compact)) {
              settings.compact = true;
            }
            if ($toast.hasClass('card')) {
              settings.compact = false;
            }
            $actions = $toast.find('.actions');
            if (module.has.configActions()) {
              if ($actions.length === 0) {
                $actions = $('<div/>', {class: className.actions + ' ' + (settings.classActions || '')}).appendTo($toast);
              }
              if($toast.hasClass('card') && !$actions.hasClass(className.attached)) {
                $actions.addClass(className.extraContent);
                if($actions.hasClass(className.vertical)) {
                  $actions.removeClass(className.vertical);
                  module.error(error.verticalCard);
                }
              }
              settings.actions.forEach(function (el) {
                var icon = el[fields.icon] ? '<i class="' + module.helpers.deQuote(el[fields.icon]) + ' icon"></i>' : '',
                  text = module.helpers.escape(el[fields.text] || '', settings.preserveHTML),
                  cls = module.helpers.deQuote(el[fields.class] || ''),
                  click = el[fields.click] && $.isFunction(el[fields.click]) ? el[fields.click] : function () {};
                $actions.append($('<button/>', {
                  html: icon + text,
                  class: className.button + ' ' + cls,
                  click: function () {
                    if (click.call(element, $module) === false) {
                      return;
                    }
                    module.close();
                  }
                }));
              });
            }
            if ($actions && $actions.hasClass(className.vertical)) {
                $toast.addClass(className.vertical);
            }
            if($actions.length > 0 && !$actions.hasClass(className.attached)) {
              if ($actions && (!$actions.hasClass(className.basic) || $actions.hasClass(className.left))) {
                $toast.addClass(className.actions);
              }
            }
            if(settings.displayTime === 'auto'){
              settings.displayTime = Math.max(settings.minDisplayTime, $toast.text().split(" ").length / settings.wordsPerMinute * 60000);
            }
            $toastBox.append($toast);

            if($actions.length > 0 && $actions.hasClass(className.attached)) {
              $actions.addClass(className.buttons);
              $actions.detach();
              $toast.addClass(className.attached);
              if (!$actions.hasClass(className.vertical)) {
                if ($actions.hasClass(className.top)) {
                  $toastBox.prepend($actions);
                  $toast.addClass(className.bottom);
                } else {
                  $toastBox.append($actions);
                  $toast.addClass(className.top);
                }
              } else {
                $toast.wrap(
                  $('<div/>',{
                    class:className.vertical + ' ' +
                          className.attached + ' ' +
                          (settings.compact ? className.compact : '')
                  })
                );
                if($actions.hasClass(className.left)) {
                  $toast.addClass(className.left).parent().addClass(className.left).prepend($actions);
                } else {
                  $toast.parent().append($actions);
                }
              }
            }
            if($module !== $toast) {
              $module = $toast;
              element = $toast[0];
            }
            if(settings.displayTime > 0) {
              var progressingClass = className.progressing+' '+(settings.pauseOnHover ? className.pausable:'');
              if (!!settings.showProgress) {
                $progress = $('<div/>', {
                  class: className.progress + ' ' + (settings.classProgress || settings.class),
                  'data-percent': ''
                });
                if(!settings.classProgress) {
                  if ($toast.hasClass('toast') && !$toast.hasClass(className.inverted)) {
                    $progress.addClass(className.inverted);
                  } else {
                    $progress.removeClass(className.inverted);
                  }
                }
                $progressBar = $('<div/>', {class: 'bar '+(settings.progressUp ? 'up ' : 'down ')+progressingClass});
                $progress
                    .addClass(settings.showProgress)
                    .append($progressBar);
                if ($progress.hasClass(className.top)) {
                  $toastBox.prepend($progress);
                } else {
                  $toastBox.append($progress);
                }
                $progressBar.css('animation-duration', settings.displayTime / 1000 + 's');
              }
              $animationObject = $('<span/>',{class:'wait '+progressingClass});
              $animationObject.css('animation-duration', settings.displayTime / 1000 + 's');
              $animationObject.appendTo($toast);
            }
            if (settings.compact) {
              $toastBox.addClass(className.compact);
              $toast.addClass(className.compact);
              if($progress) {
                $progress.addClass(className.compact);
              }
            }
            if (settings.newestOnTop) {
              $toastBox.prependTo(module.get.container());
            }
            else {
              $toastBox.appendTo(module.get.container());
            }
          }
        },

        bind: {
          events: function() {
            module.debug('Binding events to toast');
            if(settings.closeOnClick || settings.closeIcon) {
              (settings.closeIcon ? $close : $toast)
                  .on('click' + eventNamespace, module.event.click)
              ;
            }
            if($animationObject) {
              $animationObject.on('animationend' + eventNamespace, module.close);
            }
            $toastBox
              .on('click' + eventNamespace, selector.approve, module.event.approve)
              .on('click' + eventNamespace, selector.deny, module.event.deny)
            ;
          }
        },

        unbind: {
          events: function() {
            module.debug('Unbinding events to toast');
            if(settings.closeOnClick || settings.closeIcon) {
              (settings.closeIcon ? $close : $toast)
                  .off('click' + eventNamespace)
              ;
            }
            if($animationObject) {
              $animationObject.off('animationend' + eventNamespace);
            }
            $toastBox
              .off('click' + eventNamespace)
            ;
          }
        },

        animate: {
          show: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            if(settings.transition && module.can.useElement('transition') && $module.transition('is supported')) {
              module.set.visible();
              $toastBox
                .transition({
                  animation  : settings.transition.showMethod + ' in',
                  queue      : false,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  duration   : settings.transition.showDuration,
                  onComplete : function() {
                    callback.call($toastBox, element);
                    settings.onVisible.call($toastBox, element);
                  }
                })
              ;
            }
          },
          close: function(callback) {
            callback = $.isFunction(callback) ? callback : function(){};
            module.debug('Closing toast');
            if(settings.onHide.call($toastBox, element) === false) {
              module.debug('onHide callback returned false, cancelling toast animation');
              return;
            }
            if(settings.transition && $.fn.transition !== undefined && $module.transition('is supported')) {
              $toastBox
                .transition({
                  animation  : settings.transition.hideMethod + ' out',
                  queue      : false,
                  duration   : settings.transition.hideDuration,
                  debug      : settings.debug,
                  verbose    : settings.verbose,
                  interval   : 50,

                  onBeforeHide: function(callback){
                      callback = $.isFunction(callback)?callback : function(){};
                      if(settings.transition.closeEasing !== ''){
                          $toastBox.css('opacity',0);
                          $toastBox.wrap('<div/>').parent().slideUp(500,settings.transition.closeEasing,function(){
                            if($toastBox){
                              $toastBox.parent().remove();
                              callback.call($toastBox);
                            }
                          });
                      } else {
                        callback.call($toastBox);
                      }
                  },
                  onComplete : function() {
                    callback.call($toastBox, element);
                    settings.onHidden.call($toastBox, element);
                    module.destroy();
                  }
                })
              ;
            }
            else {
              module.error(error.noTransition);
            }
          },
          pause: function() {
            $animationObject.css('animationPlayState','paused');
            if($progressBar) {
              $progressBar.css('animationPlayState', 'paused');
            }
          },
          continue: function() {
            $animationObject.css('animationPlayState','running');
            if($progressBar) {
              $progressBar.css('animationPlayState', 'running');
            }
          }
        },

        has: {
          container: function() {
            module.verbose('Determining if there is already a container');
            return ($context.find(module.helpers.toClass(settings.position) + selector.container).length > 0);
          },
          toast: function(){
            return !!module.get.toast();
          },
          toasts: function(){
            return module.get.toasts().length > 0;
          },
          configActions: function () {
            return Array.isArray(settings.actions) && settings.actions.length > 0;
          }
        },

        get: {
          container: function() {
            return ($context.find(module.helpers.toClass(settings.position) + selector.container)[0]);
          },
          toastBox: function() {
            return $toastBox || null;
          },
          toast: function() {
            return $toast || null;
          },
          toasts: function() {
            return $(module.get.container()).find(selector.box);
          },
          iconClass: function() {
            return typeof settings.showIcon === 'string' ? settings.showIcon : settings.showIcon && settings.icons[settings.class] ? settings.icons[settings.class] : '';
          },
          remainingTime: function() {
            return $animationObject ? $animationObject.css('opacity') * settings.displayTime : 0;
          }
        },

        set: {
          visible: function() {
            $toast.addClass(className.visible);
          }
        },

        remove: {
          visible: function() {
            $toast.removeClass(className.visible);
          }
        },

        event: {
          click: function(event) {
            if($(event.target).closest('a').length === 0) {
              settings.onClick.call($toastBox, element);
              module.close();
            }
          },
          approve: function() {
            if(settings.onApprove.call(element, $module) === false) {
              module.verbose('Approve callback returned false cancelling close');
              return;
            }
            module.close();
          },
          deny: function() {
            if(settings.onDeny.call(element, $module) === false) {
              module.verbose('Deny callback returned false cancelling close');
              return;
            }
            module.close();
          }
        },

        helpers: {
          toClass: function(selector) {
            var
              classes = selector.split(' '),
              result = ''
            ;

            classes.forEach(function (element) {
              result += '.' + element;
            });

            return result;
          },
          deQuote: function(string) {
            return String(string).replace(/"/g,"");
          },
          escape: function(string, preserveHTML) {
            if (preserveHTML){
              return string;
            }
            var
              badChars     = /[&<>"'`]/g,
              shouldEscape = /[&<>"'`]/,
              escape       = {
                "&": "&amp;",
                "<": "&lt;",
                ">": "&gt;",
                '"': "&quot;",
                "'": "&#x27;",
                "`": "&#x60;"
              },
              escapedChar  = function(chr) {
                return escape[chr];
              }
            ;
            if(shouldEscape.test(string)) {
              return string.replace(badChars, escapedChar);
            }
            return string;
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

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
        returnedValue = $module;
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.toast.settings = {

  name           : 'Toast',
  namespace      : 'toast',

  silent         : false,
  debug          : false,
  verbose        : false,
  performance    : true,

  context        : 'body',

  position       : 'top right',
  class          : 'neutral',
  classProgress  : false,
  classActions   : false,
  classImage     : 'mini',

  title          : '',
  message        : '',
  displayTime    : 3000, // set to zero to require manually dismissal, otherwise hides on its own
  minDisplayTime : 1000, // minimum displaytime in case displayTime is set to 'auto'
  wordsPerMinute : 120,
  showIcon       : false,
  newestOnTop    : false,
  showProgress   : false,
  pauseOnHover   : true,
  progressUp     : false, //if true, the bar will start at 0% and increase to 100%
  opacity        : 1,
  compact        : true,
  closeIcon      : false,
  closeOnClick   : true,
  cloneModule    : true,
  actions        : false,
  preserveHTML   : true,
  showImage      : false,

  // transition settings
  transition     : {
    showMethod   : 'scale',
    showDuration : 500,
    hideMethod   : 'scale',
    hideDuration : 500,
    closeEasing  : 'easeOutCubic'  //Set to empty string to stack the closed toast area immediately (old behaviour)
  },

  error: {
    method       : 'The method you called is not defined.',
    noElement    : 'This module requires ui {element}',
    verticalCard : 'Vertical but not attached actions are not supported for card layout'
  },

  className      : {
    container    : 'ui toast-container',
    box          : 'floating toast-box',
    progress     : 'ui attached active progress',
    toast        : 'ui toast',
    icon         : 'centered icon',
    visible      : 'visible',
    content      : 'content',
    title        : 'ui header',
    actions      : 'actions',
    extraContent : 'extra content',
    button       : 'ui button',
    buttons      : 'ui buttons',
    close        : 'close icon',
    image        : 'ui image',
    vertical     : 'vertical',
    attached     : 'attached',
    inverted     : 'inverted',
    compact      : 'compact',
    pausable     : 'pausable',
    progressing  : 'progressing',
    top          : 'top',
    bottom       : 'bottom',
    left         : 'left',
    basic        : 'basic',
    unclickable  : 'unclickable'
  },

  icons          : {
    info         : 'info',
    success      : 'checkmark',
    warning      : 'warning',
    error        : 'times'
  },

  selector       : {
    container    : '.ui.toast-container',
    box          : '.toast-box',
    toast        : '.ui.toast',
    input        : 'input:not([type="hidden"]), textarea, select, button, .ui.button, ui.dropdown',
    approve      : '.actions .positive, .actions .approve, .actions .ok',
    deny         : '.actions .negative, .actions .deny, .actions .cancel'
  },

  fields         : {
    class        : 'class',
    text         : 'text',
    icon         : 'icon',
    click        : 'click'
  },

  // callbacks
  onShow         : function(){},
  onVisible      : function(){},
  onClick        : function(){},
  onHide         : function(){},
  onHidden       : function(){},
  onRemove       : function(){},
  onApprove      : function(){},
  onDeny         : function(){}
};

$.extend( $.easing, {
    easeOutBounce: function (x, t, b, c, d) {
        if ((t/=d) < (1/2.75)) {
            return c*(7.5625*t*t) + b;
        } else if (t < (2/2.75)) {
            return c*(7.5625*(t-=(1.5/2.75))*t + .75) + b;
        } else if (t < (2.5/2.75)) {
            return c*(7.5625*(t-=(2.25/2.75))*t + .9375) + b;
        } else {
            return c*(7.5625*(t-=(2.625/2.75))*t + .984375) + b;
        }
    },
    easeOutCubic: function (t) {
      return (--t)*t*t+1;
    }
});


})( jQuery, window, document );

/*!
 * # Fomantic-UI - Transition
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

$.fn.transition = function() {
  var
    $allModules     = $(this),
    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    moduleArguments = arguments,
    query           = moduleArguments[0],
    queryArguments  = [].slice.call(arguments, 1),
    methodInvoked   = (typeof query === 'string'),

    returnedValue
  ;
  $allModules
    .each(function(index) {
      var
        $module  = $(this),
        element  = this,

        // set at run time
        settings,
        instance,

        error,
        className,
        metadata,
        animationEnd,

        moduleNamespace,
        eventNamespace,
        module
      ;

      module = {

        initialize: function() {

          // get full settings
          settings        = module.get.settings.apply(element, moduleArguments);

          // shorthand
          className       = settings.className;
          error           = settings.error;
          metadata        = settings.metadata;

          // define namespace
          eventNamespace  = '.' + settings.namespace;
          moduleNamespace = 'module-' + settings.namespace;
          instance        = $module.data(moduleNamespace) || module;

          // get vendor specific events
          animationEnd    = module.get.animationEndEvent();

          if(methodInvoked) {
            methodInvoked = module.invoke(query);
          }

          // method not invoked, lets run an animation
          if(methodInvoked === false) {
            module.verbose('Converted arguments into settings object', settings);
            if(settings.interval) {
              module.delay(settings.animate);
            }
            else  {
              module.animate();
            }
            module.instantiate();
          }
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous module for', element);
          $module
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          module.verbose('Refreshing display type on next animation');
          delete module.displayType;
        },

        forceRepaint: function() {
          module.verbose('Forcing element repaint');
          var
            $parentElement = $module.parent(),
            $nextElement = $module.next()
          ;
          if($nextElement.length === 0) {
            $module.detach().appendTo($parentElement);
          }
          else {
            $module.detach().insertBefore($nextElement);
          }
        },

        repaint: function() {
          module.verbose('Repainting element');
          var
            fakeAssignment = element.offsetWidth
          ;
        },

        delay: function(interval) {
          var
            direction = module.get.animationDirection(),
            shouldReverse,
            delay
          ;
          if(!direction) {
            direction = module.can.transition()
              ? module.get.direction()
              : 'static'
            ;
          }
          interval = (interval !== undefined)
            ? interval
            : settings.interval
          ;
          shouldReverse = (settings.reverse == 'auto' && direction == className.outward);
          delay = (shouldReverse || settings.reverse == true)
            ? ($allModules.length - index) * settings.interval
            : index * settings.interval
          ;
          module.debug('Delaying animation by', delay);
          setTimeout(module.animate, delay);
        },

        animate: function(overrideSettings) {
          settings = overrideSettings || settings;
          if(!module.is.supported()) {
            module.error(error.support);
            return false;
          }
          module.debug('Preparing animation', settings.animation);
          if(module.is.animating()) {
            if(settings.queue) {
              if(!settings.allowRepeats && module.has.direction() && module.is.occurring() && module.queuing !== true) {
                module.debug('Animation is currently occurring, preventing queueing same animation', settings.animation);
              }
              else {
                module.queue(settings.animation);
              }
              return false;
            }
            else if(!settings.allowRepeats && module.is.occurring()) {
              module.debug('Animation is already occurring, will not execute repeated animation', settings.animation);
              return false;
            }
            else {
              module.debug('New animation started, completing previous early', settings.animation);
              instance.complete();
            }
          }
          if( module.can.animate() ) {
            module.set.animating(settings.animation);
          }
          else {
            module.error(error.noAnimation, settings.animation, element);
          }
        },

        reset: function() {
          module.debug('Resetting animation to beginning conditions');
          module.remove.animationCallbacks();
          module.restore.conditions();
          module.remove.animating();
        },

        queue: function(animation) {
          module.debug('Queueing animation of', animation);
          module.queuing = true;
          $module
            .one(animationEnd + '.queue' + eventNamespace, function() {
              module.queuing = false;
              module.repaint();
              module.animate.apply(this, settings);
            })
          ;
        },

        complete: function (event) {
          if(event && event.target === element) {
              event.stopPropagation();
          }
          module.debug('Animation complete', settings.animation);
          module.remove.completeCallback();
          module.remove.failSafe();
          if(!module.is.looping()) {
            if( module.is.outward() ) {
              module.verbose('Animation is outward, hiding element');
              module.restore.conditions();
              module.hide();
            }
            else if( module.is.inward() ) {
              module.verbose('Animation is outward, showing element');
              module.restore.conditions();
              module.show();
            }
            else {
              module.verbose('Static animation completed');
              module.restore.conditions();
              settings.onComplete.call(element);
            }
          }
        },

        force: {
          visible: function() {
            var
              style          = $module.attr('style'),
              userStyle      = module.get.userStyle(style),
              displayType    = module.get.displayType(),
              overrideStyle  = userStyle + 'display: ' + displayType + ' !important;',
              inlineDisplay  = $module[0].style.display,
              mustStayHidden = !displayType || (inlineDisplay === 'none' && settings.skipInlineHidden) || $module[0].tagName.match(/(script|link|style)/i)
            ;
            if (mustStayHidden){
              module.remove.transition();
              return false;
            }
            module.verbose('Overriding default display to show element', displayType);
            $module
              .attr('style', overrideStyle)
            ;
            return true;
          },
          hidden: function() {
            var
              style          = $module.attr('style'),
              currentDisplay = $module.css('display'),
              emptyStyle     = (style === undefined || style === '')
            ;
            if(currentDisplay !== 'none' && !module.is.hidden()) {
              module.verbose('Overriding default display to hide element');
              $module
                .css('display', 'none')
              ;
            }
            else if(emptyStyle) {
              $module
                .removeAttr('style')
              ;
            }
          }
        },

        has: {
          direction: function(animation) {
            var
              hasDirection = false
            ;
            animation = animation || settings.animation;
            if(typeof animation === 'string') {
              animation = animation.split(' ');
              $.each(animation, function(index, word){
                if(word === className.inward || word === className.outward) {
                  hasDirection = true;
                }
              });
            }
            return hasDirection;
          },
          inlineDisplay: function() {
            var
              style = $module.attr('style') || ''
            ;
            return Array.isArray(style.match(/display.*?;/, ''));
          }
        },

        set: {
          animating: function(animation) {
            // remove previous callbacks
            module.remove.completeCallback();

            // determine exact animation
            animation = animation || settings.animation;
            var animationClass = module.get.animationClass(animation);

              // save animation class in cache to restore class names
            module.save.animation(animationClass);

            if(module.force.visible()) {
              module.remove.hidden();
              module.remove.direction();

              module.start.animation(animationClass);
            }
          },
          duration: function(animationName, duration) {
            duration = duration || settings.duration;
            duration = (typeof duration == 'number')
              ? duration + 'ms'
              : duration
            ;
            if(duration || duration === 0) {
              module.verbose('Setting animation duration', duration);
              $module
                .css({
                  'animation-duration':  duration
                })
              ;
            }
          },
          direction: function(direction) {
            direction = direction || module.get.direction();
            if(direction == className.inward) {
              module.set.inward();
            }
            else {
              module.set.outward();
            }
          },
          looping: function() {
            module.debug('Transition set to loop');
            $module
              .addClass(className.looping)
            ;
          },
          hidden: function() {
            $module
              .addClass(className.transition)
              .addClass(className.hidden)
            ;
          },
          inward: function() {
            module.debug('Setting direction to inward');
            $module
              .removeClass(className.outward)
              .addClass(className.inward)
            ;
          },
          outward: function() {
            module.debug('Setting direction to outward');
            $module
              .removeClass(className.inward)
              .addClass(className.outward)
            ;
          },
          visible: function() {
            $module
              .addClass(className.transition)
              .addClass(className.visible)
            ;
          }
        },

        start: {
          animation: function(animationClass) {
            animationClass = animationClass || module.get.animationClass();
            module.debug('Starting tween', animationClass);
            $module
              .addClass(animationClass)
              .one(animationEnd + '.complete' + eventNamespace, module.complete)
            ;
            if(settings.useFailSafe) {
              module.add.failSafe();
            }
            module.set.duration(settings.duration);
            settings.onStart.call(element);
          }
        },

        save: {
          animation: function(animation) {
            if(!module.cache) {
              module.cache = {};
            }
            module.cache.animation = animation;
          },
          displayType: function(displayType) {
            if(displayType !== 'none') {
              $module.data(metadata.displayType, displayType);
            }
          },
          transitionExists: function(animation, exists) {
            $.fn.transition.exists[animation] = exists;
            module.verbose('Saving existence of transition', animation, exists);
          }
        },

        restore: {
          conditions: function() {
            var
              animation = module.get.currentAnimation()
            ;
            if(animation) {
              $module
                .removeClass(animation)
              ;
              module.verbose('Removing animation class', module.cache);
            }
            module.remove.duration();
          }
        },

        add: {
          failSafe: function() {
            var
              duration = module.get.duration()
            ;
            module.timer = setTimeout(function() {
              $module.triggerHandler(animationEnd);
            }, duration + settings.failSafeDelay);
            module.verbose('Adding fail safe timer', module.timer);
          }
        },

        remove: {
          animating: function() {
            $module.removeClass(className.animating);
          },
          animationCallbacks: function() {
            module.remove.queueCallback();
            module.remove.completeCallback();
          },
          queueCallback: function() {
            $module.off('.queue' + eventNamespace);
          },
          completeCallback: function() {
            $module.off('.complete' + eventNamespace);
          },
          display: function() {
            $module.css('display', '');
          },
          direction: function() {
            $module
              .removeClass(className.inward)
              .removeClass(className.outward)
            ;
          },
          duration: function() {
            $module
              .css('animation-duration', '')
            ;
          },
          failSafe: function() {
            module.verbose('Removing fail safe timer', module.timer);
            if(module.timer) {
              clearTimeout(module.timer);
            }
          },
          hidden: function() {
            $module.removeClass(className.hidden);
          },
          visible: function() {
            $module.removeClass(className.visible);
          },
          looping: function() {
            module.debug('Transitions are no longer looping');
            if( module.is.looping() ) {
              module.reset();
              $module
                .removeClass(className.looping)
              ;
            }
          },
          transition: function() {
            $module
              .removeClass(className.transition)
              .removeClass(className.visible)
              .removeClass(className.hidden)
            ;
          }
        },
        get: {
          settings: function(animation, duration, onComplete) {
            // single settings object
            if(typeof animation == 'object') {
              return $.extend(true, {}, $.fn.transition.settings, animation);
            }
            // all arguments provided
            else if(typeof onComplete == 'function') {
              return $.extend({}, $.fn.transition.settings, {
                animation  : animation,
                onComplete : onComplete,
                duration   : duration
              });
            }
            // only duration provided
            else if(typeof duration == 'string' || typeof duration == 'number') {
              return $.extend({}, $.fn.transition.settings, {
                animation : animation,
                duration  : duration
              });
            }
            // duration is actually settings object
            else if(typeof duration == 'object') {
              return $.extend({}, $.fn.transition.settings, duration, {
                animation : animation
              });
            }
            // duration is actually callback
            else if(typeof duration == 'function') {
              return $.extend({}, $.fn.transition.settings, {
                animation  : animation,
                onComplete : duration
              });
            }
            // only animation provided
            else {
              return $.extend({}, $.fn.transition.settings, {
                animation : animation
              });
            }
          },
          animationClass: function(animation) {
            var
              animationClass = animation || settings.animation,
              directionClass = (module.can.transition() && !module.has.direction())
                ? module.get.direction() + ' '
                : ''
            ;
            return className.animating + ' '
              + className.transition + ' '
              + directionClass
              + animationClass
            ;
          },
          currentAnimation: function() {
            return (module.cache && module.cache.animation !== undefined)
              ? module.cache.animation
              : false
            ;
          },
          currentDirection: function() {
            return module.is.inward()
              ? className.inward
              : className.outward
            ;
          },
          direction: function() {
            return module.is.hidden() || !module.is.visible()
              ? className.inward
              : className.outward
            ;
          },
          animationDirection: function(animation) {
            var
              direction
            ;
            animation = animation || settings.animation;
            if(typeof animation === 'string') {
              animation = animation.split(' ');
              // search animation name for out/in class
              $.each(animation, function(index, word){
                if(word === className.inward) {
                  direction = className.inward;
                }
                else if(word === className.outward) {
                  direction = className.outward;
                }
              });
            }
            // return found direction
            if(direction) {
              return direction;
            }
            return false;
          },
          duration: function(duration) {
            duration = duration || settings.duration;
            if(duration === false) {
              duration = $module.css('animation-duration') || 0;
            }
            return (typeof duration === 'string')
              ? (duration.indexOf('ms') > -1)
                ? parseFloat(duration)
                : parseFloat(duration) * 1000
              : duration
            ;
          },
          displayType: function(shouldDetermine) {
            shouldDetermine = (shouldDetermine !== undefined)
              ? shouldDetermine
              : true
            ;
            if(settings.displayType) {
              return settings.displayType;
            }
            if(shouldDetermine && $module.data(metadata.displayType) === undefined) {
              var currentDisplay = $module.css('display');
              if(currentDisplay === '' || currentDisplay === 'none'){
              // create fake element to determine display state
                module.can.transition(true);
              } else {
                module.save.displayType(currentDisplay);
              }
            }
            return $module.data(metadata.displayType);
          },
          userStyle: function(style) {
            style = style || $module.attr('style') || '';
            return style.replace(/display.*?;/, '');
          },
          transitionExists: function(animation) {
            return $.fn.transition.exists[animation];
          },
          animationStartEvent: function() {
            var
              element     = document.createElement('div'),
              animations  = {
                'animation'       :'animationstart',
                'OAnimation'      :'oAnimationStart',
                'MozAnimation'    :'mozAnimationStart',
                'WebkitAnimation' :'webkitAnimationStart'
              },
              animation
            ;
            for(animation in animations){
              if( element.style[animation] !== undefined ){
                return animations[animation];
              }
            }
            return false;
          },
          animationEndEvent: function() {
            var
              element     = document.createElement('div'),
              animations  = {
                'animation'       :'animationend',
                'OAnimation'      :'oAnimationEnd',
                'MozAnimation'    :'mozAnimationEnd',
                'WebkitAnimation' :'webkitAnimationEnd'
              },
              animation
            ;
            for(animation in animations){
              if( element.style[animation] !== undefined ){
                return animations[animation];
              }
            }
            return false;
          }

        },

        can: {
          transition: function(forced) {
            var
              animation         = settings.animation,
              transitionExists  = module.get.transitionExists(animation),
              displayType       = module.get.displayType(false),
              elementClass,
              tagName,
              $clone,
              currentAnimation,
              inAnimation,
              directionExists
            ;
            if( transitionExists === undefined || forced) {
              module.verbose('Determining whether animation exists');
              elementClass = $module.attr('class');
              tagName      = $module.prop('tagName');

              $clone = $('<' + tagName + ' />').addClass( elementClass ).insertAfter($module);
              currentAnimation = $clone
                .addClass(animation)
                .removeClass(className.inward)
                .removeClass(className.outward)
                .addClass(className.animating)
                .addClass(className.transition)
                .css('animationName')
              ;
              inAnimation = $clone
                .addClass(className.inward)
                .css('animationName')
              ;
              if(!displayType) {
                displayType = $clone
                  .attr('class', elementClass)
                  .removeAttr('style')
                  .removeClass(className.hidden)
                  .removeClass(className.visible)
                  .show()
                  .css('display')
                ;
                module.verbose('Determining final display state', displayType);
                module.save.displayType(displayType);
              }

              $clone.remove();
              if(currentAnimation != inAnimation) {
                module.debug('Direction exists for animation', animation);
                directionExists = true;
              }
              else if(currentAnimation == 'none' || !currentAnimation) {
                module.debug('No animation defined in css', animation);
                return;
              }
              else {
                module.debug('Static animation found', animation, displayType);
                directionExists = false;
              }
              module.save.transitionExists(animation, directionExists);
            }
            return (transitionExists !== undefined)
              ? transitionExists
              : directionExists
            ;
          },
          animate: function() {
            // can transition does not return a value if animation does not exist
            return (module.can.transition() !== undefined);
          }
        },

        is: {
          animating: function() {
            return $module.hasClass(className.animating);
          },
          inward: function() {
            return $module.hasClass(className.inward);
          },
          outward: function() {
            return $module.hasClass(className.outward);
          },
          looping: function() {
            return $module.hasClass(className.looping);
          },
          occurring: function(animation) {
            animation = animation || settings.animation;
            animation = '.' + animation.replace(' ', '.');
            return ( $module.filter(animation).length > 0 );
          },
          visible: function() {
            return $module.is(':visible');
          },
          hidden: function() {
            return $module.css('visibility') === 'hidden';
          },
          supported: function() {
            return(animationEnd !== false);
          }
        },

        hide: function() {
          module.verbose('Hiding element');
          if( module.is.animating() ) {
            module.reset();
          }
          element.blur(); // IE will trigger focus change if element is not blurred before hiding
          module.remove.display();
          module.remove.visible();
          if($.isFunction(settings.onBeforeHide)){
            settings.onBeforeHide.call(element,function(){
                module.hideNow();
            });
          } else {
              module.hideNow();
          }

        },

        hideNow: function() {
            module.set.hidden();
            module.force.hidden();
            settings.onHide.call(element);
            settings.onComplete.call(element);
            // module.repaint();
        },

        show: function(display) {
          module.verbose('Showing element', display);
          if(module.force.visible()) {
            module.remove.hidden();
            module.set.visible();
            settings.onShow.call(element);
            settings.onComplete.call(element);
            // module.repaint();
          }
        },

        toggle: function() {
          if( module.is.visible() ) {
            module.hide();
          }
          else {
            module.show();
          }
        },

        stop: function() {
          module.debug('Stopping current animation');
          $module.triggerHandler(animationEnd);
        },

        stopAll: function() {
          module.debug('Stopping all animation');
          module.remove.queueCallback();
          $module.triggerHandler(animationEnd);
        },

        clear: {
          queue: function() {
            module.debug('Clearing animation queue');
            module.remove.queueCallback();
          }
        },

        enable: function() {
          module.verbose('Starting animation');
          $module.removeClass(className.disabled);
        },

        disable: function() {
          module.debug('Stopping animation');
          $module.addClass(className.disabled);
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
        // modified for transition to return invoke success
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
          if ( $.isFunction( found ) ) {
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
          return (found !== undefined)
            ? found
            : false
          ;
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

// Records if CSS transition is available
$.fn.transition.exists = {};

$.fn.transition.settings = {

  // module info
  name          : 'Transition',

  // hide all output from this component regardless of other settings
  silent        : false,

  // debug content outputted to console
  debug         : false,

  // verbose debug output
  verbose       : false,

  // performance data output
  performance   : true,

  // event namespace
  namespace     : 'transition',

  // delay between animations in group
  interval      : 0,

  // whether group animations should be reversed
  reverse       : 'auto',

  // animation callback event
  onStart       : function() {},
  onComplete    : function() {},
  onShow        : function() {},
  onHide        : function() {},

  // whether timeout should be used to ensure callback fires in cases animationend does not
  useFailSafe   : true,

  // delay in ms for fail safe
  failSafeDelay : 100,

  // whether EXACT animation can occur twice in a row
  allowRepeats  : false,

  // Override final display type on visible
  displayType   : false,

  // animation duration
  animation     : 'fade',
  duration      : false,

  // new animations will occur after previous ones
  queue         : true,

// whether initially inline hidden objects should be skipped for transition
  skipInlineHidden: false,

  metadata : {
    displayType: 'display'
  },

  className   : {
    animating  : 'animating',
    disabled   : 'disabled',
    hidden     : 'hidden',
    inward     : 'in',
    loading    : 'loading',
    looping    : 'looping',
    outward    : 'out',
    transition : 'transition',
    visible    : 'visible'
  },

  // possible errors
  error: {
    noAnimation : 'Element is no longer attached to DOM. Unable to animate.  Use silent setting to surpress this warning in production.',
    repeated    : 'That animation is already occurring, cancelling repeated animation',
    method      : 'The method you called is not defined',
    support     : 'This browser does not support CSS animations'
  }

};


})( jQuery, window, document );

/*!
 * # Fomantic-UI - API
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

'use strict';

$.isWindow = $.isWindow || function(obj) {
  return obj != null && obj === obj.window;
};

  window = (typeof window != 'undefined' && window.Math == Math)
    ? window
    : (typeof self != 'undefined' && self.Math == Math)
      ? self
      : Function('return this')()
;

$.api = $.fn.api = function(parameters) {

  var
    // use window context if none specified
    $allModules     = $.isFunction(this)
        ? $(window)
        : $(this),
    moduleSelector = $allModules.selector || '',
    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),

    returnedValue
  ;

  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.api.settings, parameters)
          : $.extend({}, $.fn.api.settings),

        // internal aliases
        namespace       = settings.namespace,
        metadata        = settings.metadata,
        selector        = settings.selector,
        error           = settings.error,
        className       = settings.className,

        // define namespaces for modules
        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        // element that creates request
        $module         = $(this),
        $form           = $module.closest(selector.form),

        // context used for state
        $context        = (settings.stateContext)
          ? $(settings.stateContext)
          : $module,

        // request details
        ajaxSettings,
        requestSettings,
        url,
        data,
        requestStartTime,

        // standard module
        element         = this,
        context         = $context[0],
        instance        = $module.data(moduleNamespace),
        module
      ;

      module = {

        initialize: function() {
          if(!methodInvoked) {
            module.bind.events();
          }
          module.instantiate();
        },

        instantiate: function() {
          module.verbose('Storing instance of module', module);
          instance = module;
          $module
            .data(moduleNamespace, instance)
          ;
        },

        destroy: function() {
          module.verbose('Destroying previous module for', element);
          $module
            .removeData(moduleNamespace)
            .off(eventNamespace)
          ;
        },

        bind: {
          events: function() {
            var
              triggerEvent = module.get.event()
            ;
            if( triggerEvent ) {
              module.verbose('Attaching API events to element', triggerEvent);
              $module
                .on(triggerEvent + eventNamespace, module.event.trigger)
              ;
            }
            else if(settings.on == 'now') {
              module.debug('Querying API endpoint immediately');
              module.query();
            }
          }
        },

        decode: {
          json: function(response) {
            if(response !== undefined && typeof response == 'string') {
              try {
               response = JSON.parse(response);
              }
              catch(e) {
                // isnt json string
              }
            }
            return response;
          }
        },

        read: {
          cachedResponse: function(url) {
            var
              response
            ;
            if(window.Storage === undefined) {
              module.error(error.noStorage);
              return;
            }
            response = sessionStorage.getItem(url);
            module.debug('Using cached response', url, response);
            response = module.decode.json(response);
            return response;
          }
        },
        write: {
          cachedResponse: function(url, response) {
            if(response && response === '') {
              module.debug('Response empty, not caching', response);
              return;
            }
            if(window.Storage === undefined) {
              module.error(error.noStorage);
              return;
            }
            if( $.isPlainObject(response) ) {
              response = JSON.stringify(response);
            }
            sessionStorage.setItem(url, response);
            module.verbose('Storing cached response for url', url, response);
          }
        },

        query: function() {

          if(module.is.disabled()) {
            module.debug('Element is disabled API request aborted');
            return;
          }

          if(module.is.loading()) {
            if(settings.interruptRequests) {
              module.debug('Interrupting previous request');
              module.abort();
            }
            else {
              module.debug('Cancelling request, previous request is still pending');
              return;
            }
          }

          // pass element metadata to url (value, text)
          if(settings.defaultData) {
            $.extend(true, settings.urlData, module.get.defaultData());
          }

          // Add form content
          if(settings.serializeForm) {
            settings.data = module.add.formData(settings.data);
          }

          // call beforesend and get any settings changes
          requestSettings = module.get.settings();

          // check if before send cancelled request
          if(requestSettings === false) {
            module.cancelled = true;
            module.error(error.beforeSend);
            return;
          }
          else {
            module.cancelled = false;
          }

          // get url
          url = module.get.templatedURL();

          if(!url && !module.is.mocked()) {
            module.error(error.missingURL);
            return;
          }

          // replace variables
          url = module.add.urlData( url );
          // missing url parameters
          if( !url && !module.is.mocked()) {
            return;
          }

          requestSettings.url = settings.base + url;

          // look for jQuery ajax parameters in settings
          ajaxSettings = $.extend(true, {}, settings, {
            type       : settings.method || settings.type,
            data       : data,
            url        : settings.base + url,
            beforeSend : settings.beforeXHR,
            success    : function() {},
            failure    : function() {},
            complete   : function() {}
          });

          module.debug('Querying URL', ajaxSettings.url);
          module.verbose('Using AJAX settings', ajaxSettings);
          if(settings.cache === 'local' && module.read.cachedResponse(url)) {
            module.debug('Response returned from local cache');
            module.request = module.create.request();
            module.request.resolveWith(context, [ module.read.cachedResponse(url) ]);
            return;
          }

          if( !settings.throttle ) {
            module.debug('Sending request', data, ajaxSettings.method);
            module.send.request();
          }
          else {
            if(!settings.throttleFirstRequest && !module.timer) {
              module.debug('Sending request', data, ajaxSettings.method);
              module.send.request();
              module.timer = setTimeout(function(){}, settings.throttle);
            }
            else {
              module.debug('Throttling request', settings.throttle);
              clearTimeout(module.timer);
              module.timer = setTimeout(function() {
                if(module.timer) {
                  delete module.timer;
                }
                module.debug('Sending throttled request', data, ajaxSettings.method);
                module.send.request();
              }, settings.throttle);
            }
          }

        },

        should: {
          removeError: function() {
            return ( settings.hideError === true || (settings.hideError === 'auto' && !module.is.form()) );
          }
        },

        is: {
          disabled: function() {
            return ($module.filter(selector.disabled).length > 0);
          },
          expectingJSON: function() {
            return settings.dataType === 'json' || settings.dataType === 'jsonp';
          },
          form: function() {
            return $module.is('form') || $context.is('form');
          },
          mocked: function() {
            return (settings.mockResponse || settings.mockResponseAsync || settings.response || settings.responseAsync);
          },
          input: function() {
            return $module.is('input');
          },
          loading: function() {
            return (module.request)
              ? (module.request.state() == 'pending')
              : false
            ;
          },
          abortedRequest: function(xhr) {
            if(xhr && xhr.readyState !== undefined && xhr.readyState === 0) {
              module.verbose('XHR request determined to be aborted');
              return true;
            }
            else {
              module.verbose('XHR request was not aborted');
              return false;
            }
          },
          validResponse: function(response) {
            if( (!module.is.expectingJSON()) || !$.isFunction(settings.successTest) ) {
              module.verbose('Response is not JSON, skipping validation', settings.successTest, response);
              return true;
            }
            module.debug('Checking JSON returned success', settings.successTest, response);
            if( settings.successTest(response) ) {
              module.debug('Response passed success test', response);
              return true;
            }
            else {
              module.debug('Response failed success test', response);
              return false;
            }
          }
        },

        was: {
          cancelled: function() {
            return (module.cancelled || false);
          },
          succesful: function() {
            module.verbose('This behavior will be deleted due to typo. Use "was successful" instead.');
            return module.was.successful();
          },
          successful: function() {
            return (module.request && module.request.state() == 'resolved');
          },
          failure: function() {
            return (module.request && module.request.state() == 'rejected');
          },
          complete: function() {
            return (module.request && (module.request.state() == 'resolved' || module.request.state() == 'rejected') );
          }
        },

        add: {
          urlData: function(url, urlData) {
            var
              requiredVariables,
              optionalVariables
            ;
            if(url) {
              requiredVariables = url.match(settings.regExp.required);
              optionalVariables = url.match(settings.regExp.optional);
              urlData           = urlData || settings.urlData;
              if(requiredVariables) {
                module.debug('Looking for required URL variables', requiredVariables);
                $.each(requiredVariables, function(index, templatedString) {
                  var
                    // allow legacy {$var} style
                    variable = (templatedString.indexOf('$') !== -1)
                      ? templatedString.substr(2, templatedString.length - 3)
                      : templatedString.substr(1, templatedString.length - 2),
                    value   = ($.isPlainObject(urlData) && urlData[variable] !== undefined)
                      ? urlData[variable]
                      : ($module.data(variable) !== undefined)
                        ? $module.data(variable)
                        : ($context.data(variable) !== undefined)
                          ? $context.data(variable)
                          : urlData[variable]
                  ;
                  // remove value
                  if(value === undefined) {
                    module.error(error.requiredParameter, variable, url);
                    url = false;
                    return false;
                  }
                  else {
                    module.verbose('Found required variable', variable, value);
                    value = (settings.encodeParameters)
                      ? module.get.urlEncodedValue(value)
                      : value
                    ;
                    url = url.replace(templatedString, value);
                  }
                });
              }
              if(optionalVariables) {
                module.debug('Looking for optional URL variables', requiredVariables);
                $.each(optionalVariables, function(index, templatedString) {
                  var
                    // allow legacy {/$var} style
                    variable = (templatedString.indexOf('$') !== -1)
                      ? templatedString.substr(3, templatedString.length - 4)
                      : templatedString.substr(2, templatedString.length - 3),
                    value   = ($.isPlainObject(urlData) && urlData[variable] !== undefined)
                      ? urlData[variable]
                      : ($module.data(variable) !== undefined)
                        ? $module.data(variable)
                        : ($context.data(variable) !== undefined)
                          ? $context.data(variable)
                          : urlData[variable]
                  ;
                  // optional replacement
                  if(value !== undefined) {
                    module.verbose('Optional variable Found', variable, value);
                    url = url.replace(templatedString, value);
                  }
                  else {
                    module.verbose('Optional variable not found', variable);
                    // remove preceding slash if set
                    if(url.indexOf('/' + templatedString) !== -1) {
                      url = url.replace('/' + templatedString, '');
                    }
                    else {
                      url = url.replace(templatedString, '');
                    }
                  }
                });
              }
            }
            return url;
          },
          formData: function(data) {
            var
              canSerialize = ($.fn.serializeObject !== undefined),
              formData     = (canSerialize)
                ? $form.serializeObject()
                : $form.serialize(),
              hasOtherData
            ;
            data         = data || settings.data;
            hasOtherData = $.isPlainObject(data);

            if(hasOtherData) {
              if(canSerialize) {
                module.debug('Extending existing data with form data', data, formData);
                data = $.extend(true, {}, data, formData);
              }
              else {
                module.error(error.missingSerialize);
                module.debug('Cant extend data. Replacing data with form data', data, formData);
                data = formData;
              }
            }
            else {
              module.debug('Adding form data', formData);
              data = formData;
            }
            return data;
          }
        },

        send: {
          request: function() {
            module.set.loading();
            module.request = module.create.request();
            if( module.is.mocked() ) {
              module.mockedXHR = module.create.mockedXHR();
            }
            else {
              module.xhr = module.create.xhr();
            }
            settings.onRequest.call(context, module.request, module.xhr);
          }
        },

        event: {
          trigger: function(event) {
            module.query();
            if(event.type == 'submit' || event.type == 'click') {
              event.preventDefault();
            }
          },
          xhr: {
            always: function() {
              // nothing special
            },
            done: function(response, textStatus, xhr) {
              var
                context            = this,
                elapsedTime        = (new Date().getTime() - requestStartTime),
                timeLeft           = (settings.loadingDuration - elapsedTime),
                translatedResponse = ( $.isFunction(settings.onResponse) )
                  ? module.is.expectingJSON() && !settings.rawResponse
                    ? settings.onResponse.call(context, $.extend(true, {}, response))
                    : settings.onResponse.call(context, response)
                  : false
              ;
              timeLeft = (timeLeft > 0)
                ? timeLeft
                : 0
              ;
              if(translatedResponse) {
                module.debug('Modified API response in onResponse callback', settings.onResponse, translatedResponse, response);
                response = translatedResponse;
              }
              if(timeLeft > 0) {
                module.debug('Response completed early delaying state change by', timeLeft);
              }
              setTimeout(function() {
                if( module.is.validResponse(response) ) {
                  module.request.resolveWith(context, [response, xhr]);
                }
                else {
                  module.request.rejectWith(context, [xhr, 'invalid']);
                }
              }, timeLeft);
            },
            fail: function(xhr, status, httpMessage) {
              var
                context     = this,
                elapsedTime = (new Date().getTime() - requestStartTime),
                timeLeft    = (settings.loadingDuration - elapsedTime)
              ;
              timeLeft = (timeLeft > 0)
                ? timeLeft
                : 0
              ;
              if(timeLeft > 0) {
                module.debug('Response completed early delaying state change by', timeLeft);
              }
              setTimeout(function() {
                if( module.is.abortedRequest(xhr) ) {
                  module.request.rejectWith(context, [xhr, 'aborted', httpMessage]);
                }
                else {
                  module.request.rejectWith(context, [xhr, 'error', status, httpMessage]);
                }
              }, timeLeft);
            }
          },
          request: {
            done: function(response, xhr) {
              module.debug('Successful API Response', response);
              if(settings.cache === 'local' && url) {
                module.write.cachedResponse(url, response);
                module.debug('Saving server response locally', module.cache);
              }
              settings.onSuccess.call(context, response, $module, xhr);
            },
            complete: function(firstParameter, secondParameter) {
              var
                xhr,
                response
              ;
              // have to guess callback parameters based on request success
              if( module.was.successful() ) {
                response = firstParameter;
                xhr      = secondParameter;
              }
              else {
                xhr      = firstParameter;
                response = module.get.responseFromXHR(xhr);
              }
              module.remove.loading();
              settings.onComplete.call(context, response, $module, xhr);
            },
            fail: function(xhr, status, httpMessage) {
              var
                // pull response from xhr if available
                response     = module.get.responseFromXHR(xhr),
                errorMessage = module.get.errorFromRequest(response, status, httpMessage)
              ;
              if(status == 'aborted') {
                module.debug('XHR Aborted (Most likely caused by page navigation or CORS Policy)', status, httpMessage);
                settings.onAbort.call(context, status, $module, xhr);
                return true;
              }
              else if(status == 'invalid') {
                module.debug('JSON did not pass success test. A server-side error has most likely occurred', response);
              }
              else if(status == 'error') {
                if(xhr !== undefined) {
                  module.debug('XHR produced a server error', status, httpMessage);
                  // make sure we have an error to display to console
                  if( (xhr.status < 200 || xhr.status >= 300) && httpMessage !== undefined && httpMessage !== '') {
                    module.error(error.statusMessage + httpMessage, ajaxSettings.url);
                  }
                  settings.onError.call(context, errorMessage, $module, xhr);
                }
              }

              if(settings.errorDuration && status !== 'aborted') {
                module.debug('Adding error state');
                module.set.error();
                if( module.should.removeError() ) {
                  setTimeout(module.remove.error, settings.errorDuration);
                }
              }
              module.debug('API Request failed', errorMessage, xhr);
              settings.onFailure.call(context, response, $module, xhr);
            }
          }
        },

        create: {

          request: function() {
            // api request promise
            return $.Deferred()
              .always(module.event.request.complete)
              .done(module.event.request.done)
              .fail(module.event.request.fail)
            ;
          },

          mockedXHR: function () {
            var
              // xhr does not simulate these properties of xhr but must return them
              textStatus     = false,
              status         = false,
              httpMessage    = false,
              responder      = settings.mockResponse      || settings.response,
              asyncResponder = settings.mockResponseAsync || settings.responseAsync,
              asyncCallback,
              response,
              mockedXHR
            ;

            mockedXHR = $.Deferred()
              .always(module.event.xhr.complete)
              .done(module.event.xhr.done)
              .fail(module.event.xhr.fail)
            ;

            if(responder) {
              if( $.isFunction(responder) ) {
                module.debug('Using specified synchronous callback', responder);
                response = responder.call(context, requestSettings);
              }
              else {
                module.debug('Using settings specified response', responder);
                response = responder;
              }
              // simulating response
              mockedXHR.resolveWith(context, [ response, textStatus, { responseText: response }]);
            }
            else if( $.isFunction(asyncResponder) ) {
              asyncCallback = function(response) {
                module.debug('Async callback returned response', response);

                if(response) {
                  mockedXHR.resolveWith(context, [ response, textStatus, { responseText: response }]);
                }
                else {
                  mockedXHR.rejectWith(context, [{ responseText: response }, status, httpMessage]);
                }
              };
              module.debug('Using specified async response callback', asyncResponder);
              asyncResponder.call(context, requestSettings, asyncCallback);
            }
            return mockedXHR;
          },

          xhr: function() {
            var
              xhr
            ;
            // ajax request promise
            xhr = $.ajax(ajaxSettings)
              .always(module.event.xhr.always)
              .done(module.event.xhr.done)
              .fail(module.event.xhr.fail)
            ;
            module.verbose('Created server request', xhr, ajaxSettings);
            return xhr;
          }
        },

        set: {
          error: function() {
            module.verbose('Adding error state to element', $context);
            $context.addClass(className.error);
          },
          loading: function() {
            module.verbose('Adding loading state to element', $context);
            $context.addClass(className.loading);
            requestStartTime = new Date().getTime();
          }
        },

        remove: {
          error: function() {
            module.verbose('Removing error state from element', $context);
            $context.removeClass(className.error);
          },
          loading: function() {
            module.verbose('Removing loading state from element', $context);
            $context.removeClass(className.loading);
          }
        },

        get: {
          responseFromXHR: function(xhr) {
            return $.isPlainObject(xhr)
              ? (module.is.expectingJSON())
                ? module.decode.json(xhr.responseText)
                : xhr.responseText
              : false
            ;
          },
          errorFromRequest: function(response, status, httpMessage) {
            return ($.isPlainObject(response) && response.error !== undefined)
              ? response.error // use json error message
              : (settings.error[status] !== undefined) // use server error message
                ? settings.error[status]
                : httpMessage
            ;
          },
          request: function() {
            return module.request || false;
          },
          xhr: function() {
            return module.xhr || false;
          },
          settings: function() {
            var
              runSettings
            ;
            runSettings = settings.beforeSend.call($module, settings);
            if(runSettings) {
              if(runSettings.success !== undefined) {
                module.debug('Legacy success callback detected', runSettings);
                module.error(error.legacyParameters, runSettings.success);
                runSettings.onSuccess = runSettings.success;
              }
              if(runSettings.failure !== undefined) {
                module.debug('Legacy failure callback detected', runSettings);
                module.error(error.legacyParameters, runSettings.failure);
                runSettings.onFailure = runSettings.failure;
              }
              if(runSettings.complete !== undefined) {
                module.debug('Legacy complete callback detected', runSettings);
                module.error(error.legacyParameters, runSettings.complete);
                runSettings.onComplete = runSettings.complete;
              }
            }
            if(runSettings === undefined) {
              module.error(error.noReturnedValue);
            }
            if(runSettings === false) {
              return runSettings;
            }
            return (runSettings !== undefined)
              ? $.extend(true, {}, runSettings)
              : $.extend(true, {}, settings)
            ;
          },
          urlEncodedValue: function(value) {
            var
              decodedValue   = window.decodeURIComponent(value),
              encodedValue   = window.encodeURIComponent(value),
              alreadyEncoded = (decodedValue !== value)
            ;
            if(alreadyEncoded) {
              module.debug('URL value is already encoded, avoiding double encoding', value);
              return value;
            }
            module.verbose('Encoding value using encodeURIComponent', value, encodedValue);
            return encodedValue;
          },
          defaultData: function() {
            var
              data = {}
            ;
            if( !$.isWindow(element) ) {
              if( module.is.input() ) {
                data.value = $module.val();
              }
              else if( module.is.form() ) {

              }
              else {
                data.text = $module.text();
              }
            }
            return data;
          },
          event: function() {
            if( $.isWindow(element) || settings.on == 'now' ) {
              module.debug('API called without element, no events attached');
              return false;
            }
            else if(settings.on == 'auto') {
              if( $module.is('input') ) {
                return (element.oninput !== undefined)
                  ? 'input'
                  : (element.onpropertychange !== undefined)
                    ? 'propertychange'
                    : 'keyup'
                ;
              }
              else if( $module.is('form') ) {
                return 'submit';
              }
              else {
                return 'click';
              }
            }
            else {
              return settings.on;
            }
          },
          templatedURL: function(action) {
            action = action || $module.data(metadata.action) || settings.action || false;
            url    = $module.data(metadata.url) || settings.url || false;
            if(url) {
              module.debug('Using specified url', url);
              return url;
            }
            if(action) {
              module.debug('Looking up url for action', action, settings.api);
              if(settings.api[action] === undefined && !module.is.mocked()) {
                module.error(error.missingAction, settings.action, settings.api);
                return;
              }
              url = settings.api[action];
            }
            else if( module.is.form() ) {
              url = $module.attr('action') || $context.attr('action') || false;
              module.debug('No url or action specified, defaulting to form action', url);
            }
            return url;
          }
        },

        abort: function() {
          var
            xhr = module.get.xhr()
          ;
          if( xhr && xhr.state() !== 'resolved') {
            module.debug('Cancelling API request');
            xhr.abort();
          }
        },

        // reset state
        reset: function() {
          module.remove.error();
          module.remove.loading();
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                //'Element'        : element,
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.api.settings = {

  name              : 'API',
  namespace         : 'api',

  debug             : false,
  verbose           : false,
  performance       : true,

  // object containing all templates endpoints
  api               : {},

  // whether to cache responses
  cache             : true,

  // whether new requests should abort previous requests
  interruptRequests : true,

  // event binding
  on                : 'auto',

  // context for applying state classes
  stateContext      : false,

  // duration for loading state
  loadingDuration   : 0,

  // whether to hide errors after a period of time
  hideError         : 'auto',

  // duration for error state
  errorDuration     : 2000,

  // whether parameters should be encoded with encodeURIComponent
  encodeParameters  : true,

  // API action to use
  action            : false,

  // templated URL to use
  url               : false,

  // base URL to apply to all endpoints
  base              : '',

  // data that will
  urlData           : {},

  // whether to add default data to url data
  defaultData          : true,

  // whether to serialize closest form
  serializeForm        : false,

  // how long to wait before request should occur
  throttle             : 0,

  // whether to throttle first request or only repeated
  throttleFirstRequest : true,

  // standard ajax settings
  method            : 'get',
  data              : {},
  dataType          : 'json',

  // mock response
  mockResponse      : false,
  mockResponseAsync : false,

  // aliases for mock
  response          : false,
  responseAsync     : false,

// whether onResponse should work with response value without force converting into an object
  rawResponse       : false,

  // callbacks before request
  beforeSend  : function(settings) { return settings; },
  beforeXHR   : function(xhr) {},
  onRequest   : function(promise, xhr) {},

  // after request
  onResponse  : false, // function(response) { },

  // response was successful, if JSON passed validation
  onSuccess   : function(response, $module) {},

  // request finished without aborting
  onComplete  : function(response, $module) {},

  // failed JSON success test
  onFailure   : function(response, $module) {},

  // server error
  onError     : function(errorMessage, $module) {},

  // request aborted
  onAbort     : function(errorMessage, $module) {},

  successTest : false,

  // errors
  error : {
    beforeSend        : 'The before send function has aborted the request',
    error             : 'There was an error with your request',
    exitConditions    : 'API Request Aborted. Exit conditions met',
    JSONParse         : 'JSON could not be parsed during error handling',
    legacyParameters  : 'You are using legacy API success callback names',
    method            : 'The method you called is not defined',
    missingAction     : 'API action used but no url was defined',
    missingSerialize  : 'jquery-serialize-object is required to add form data to an existing data object',
    missingURL        : 'No URL specified for api event',
    noReturnedValue   : 'The beforeSend callback must return a settings object, beforeSend ignored.',
    noStorage         : 'Caching responses locally requires session storage',
    parseError        : 'There was an error parsing your request',
    requiredParameter : 'Missing a required URL parameter: ',
    statusMessage     : 'Server gave an error: ',
    timeout           : 'Your request timed out'
  },

  regExp  : {
    required : /\{\$*[A-z0-9]+\}/g,
    optional : /\{\/\$*[A-z0-9]+\}/g,
  },

  className: {
    loading : 'loading',
    error   : 'error'
  },

  selector: {
    disabled : '.disabled',
    form      : 'form'
  },

  metadata: {
    action  : 'action',
    url     : 'url'
  }
};



})( jQuery, window, document );

/*!
 * # Fomantic-UI - State
 * http://github.com/fomantic/Fomantic-UI/
 *
 *
 * Released under the MIT license
 * http://opensource.org/licenses/MIT
 *
 */

;(function ($, window, document, undefined) {

"use strict";

$.isFunction = $.isFunction || function(obj) {
  return typeof obj === "function" && typeof obj.nodeType !== "number";
};

window = (typeof window != 'undefined' && window.Math == Math)
  ? window
  : (typeof self != 'undefined' && self.Math == Math)
    ? self
    : Function('return this')()
;

$.fn.state = function(parameters) {
  var
    $allModules     = $(this),

    moduleSelector  = $allModules.selector || '',

    time            = new Date().getTime(),
    performance     = [],

    query           = arguments[0],
    methodInvoked   = (typeof query == 'string'),
    queryArguments  = [].slice.call(arguments, 1),

    returnedValue
  ;
  $allModules
    .each(function() {
      var
        settings          = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.state.settings, parameters)
          : $.extend({}, $.fn.state.settings),

        error           = settings.error,
        metadata        = settings.metadata,
        className       = settings.className,
        namespace       = settings.namespace,
        states          = settings.states,
        text            = settings.text,

        eventNamespace  = '.' + namespace,
        moduleNamespace = namespace + '-module',

        $module         = $(this),

        element         = this,
        instance        = $module.data(moduleNamespace),

        module
      ;
      module = {

        initialize: function() {
          module.verbose('Initializing module');

          // allow module to guess desired state based on element
          if(settings.automatic) {
            module.add.defaults();
          }

          // bind events with delegated events
          if(settings.context && moduleSelector !== '') {
            $(settings.context)
              .on(moduleSelector, 'mouseenter' + eventNamespace, module.change.text)
              .on(moduleSelector, 'mouseleave' + eventNamespace, module.reset.text)
              .on(moduleSelector, 'click'      + eventNamespace, module.toggle.state)
            ;
          }
          else {
            $module
              .on('mouseenter' + eventNamespace, module.change.text)
              .on('mouseleave' + eventNamespace, module.reset.text)
              .on('click'      + eventNamespace, module.toggle.state)
            ;
          }
          module.instantiate();
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
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
        },

        refresh: function() {
          module.verbose('Refreshing selector cache');
          $module = $(element);
        },

        add: {
          defaults: function() {
            var
              userStates = parameters && $.isPlainObject(parameters.states)
                ? parameters.states
                : {}
            ;
            $.each(settings.defaults, function(type, typeStates) {
              if( module.is[type] !== undefined && module.is[type]() ) {
                module.verbose('Adding default states', type, element);
                $.extend(settings.states, typeStates, userStates);
              }
            });
          }
        },

        is: {

          active: function() {
            return $module.hasClass(className.active);
          },
          loading: function() {
            return $module.hasClass(className.loading);
          },
          inactive: function() {
            return !( $module.hasClass(className.active) );
          },
          state: function(state) {
            if(className[state] === undefined) {
              return false;
            }
            return $module.hasClass( className[state] );
          },

          enabled: function() {
            return !( $module.is(settings.filter.active) );
          },
          disabled: function() {
            return ( $module.is(settings.filter.active) );
          },
          textEnabled: function() {
            return !( $module.is(settings.filter.text) );
          },

          // definitions for automatic type detection
          button: function() {
            return $module.is('.button:not(a, .submit)');
          },
          input: function() {
            return $module.is('input');
          },
          progress: function() {
            return $module.is('.ui.progress');
          }
        },

        allow: function(state) {
          module.debug('Now allowing state', state);
          states[state] = true;
        },
        disallow: function(state) {
          module.debug('No longer allowing', state);
          states[state] = false;
        },

        allows: function(state) {
          return states[state] || false;
        },

        enable: function() {
          $module.removeClass(className.disabled);
        },

        disable: function() {
          $module.addClass(className.disabled);
        },

        setState: function(state) {
          if(module.allows(state)) {
            $module.addClass( className[state] );
          }
        },

        removeState: function(state) {
          if(module.allows(state)) {
            $module.removeClass( className[state] );
          }
        },

        toggle: {
          state: function() {
            var
              apiRequest,
              requestCancelled
            ;
            if( module.allows('active') && module.is.enabled() ) {
              module.refresh();
              if($.fn.api !== undefined) {
                apiRequest       = $module.api('get request');
                requestCancelled = $module.api('was cancelled');
                if( requestCancelled ) {
                  module.debug('API Request cancelled by beforesend');
                  settings.activateTest   = function(){ return false; };
                  settings.deactivateTest = function(){ return false; };
                }
                else if(apiRequest) {
                  module.listenTo(apiRequest);
                  return;
                }
              }
              module.change.state();
            }
          }
        },

        listenTo: function(apiRequest) {
          module.debug('API request detected, waiting for state signal', apiRequest);
          if(apiRequest) {
            if(text.loading) {
              module.update.text(text.loading);
            }
            $.when(apiRequest)
              .then(function() {
                if(apiRequest.state() == 'resolved') {
                  module.debug('API request succeeded');
                  settings.activateTest   = function(){ return true; };
                  settings.deactivateTest = function(){ return true; };
                }
                else {
                  module.debug('API request failed');
                  settings.activateTest   = function(){ return false; };
                  settings.deactivateTest = function(){ return false; };
                }
                module.change.state();
              })
            ;
          }
        },

        // checks whether active/inactive state can be given
        change: {

          state: function() {
            module.debug('Determining state change direction');
            // inactive to active change
            if( module.is.inactive() ) {
              module.activate();
            }
            else {
              module.deactivate();
            }
            if(settings.sync) {
              module.sync();
            }
            settings.onChange.call(element);
          },

          text: function() {
            if( module.is.textEnabled() ) {
              if(module.is.disabled() ) {
                module.verbose('Changing text to disabled text', text.hover);
                module.update.text(text.disabled);
              }
              else if( module.is.active() ) {
                if(text.hover) {
                  module.verbose('Changing text to hover text', text.hover);
                  module.update.text(text.hover);
                }
                else if(text.deactivate) {
                  module.verbose('Changing text to deactivating text', text.deactivate);
                  module.update.text(text.deactivate);
                }
              }
              else {
                if(text.hover) {
                  module.verbose('Changing text to hover text', text.hover);
                  module.update.text(text.hover);
                }
                else if(text.activate){
                  module.verbose('Changing text to activating text', text.activate);
                  module.update.text(text.activate);
                }
              }
            }
          }

        },

        activate: function() {
          if( settings.activateTest.call(element) ) {
            module.debug('Setting state to active');
            $module
              .addClass(className.active)
            ;
            module.update.text(text.active);
            settings.onActivate.call(element);
          }
        },

        deactivate: function() {
          if( settings.deactivateTest.call(element) ) {
            module.debug('Setting state to inactive');
            $module
              .removeClass(className.active)
            ;
            module.update.text(text.inactive);
            settings.onDeactivate.call(element);
          }
        },

        sync: function() {
          module.verbose('Syncing other buttons to current state');
          if( module.is.active() ) {
            $allModules
              .not($module)
                .state('activate');
          }
          else {
            $allModules
              .not($module)
                .state('deactivate')
            ;
          }
        },

        get: {
          text: function() {
            return (settings.selector.text)
              ? $module.find(settings.selector.text).text()
              : $module.html()
            ;
          },
          textFor: function(state) {
            return text[state] || false;
          }
        },

        flash: {
          text: function(text, duration, callback) {
            var
              previousText = module.get.text()
            ;
            module.debug('Flashing text message', text, duration);
            text     = text     || settings.text.flash;
            duration = duration || settings.flashDuration;
            callback = callback || function() {};
            module.update.text(text);
            setTimeout(function(){
              module.update.text(previousText);
              callback.call(element);
            }, duration);
          }
        },

        reset: {
          // on mouseout sets text to previous value
          text: function() {
            var
              activeText   = text.active   || $module.data(metadata.storedText),
              inactiveText = text.inactive || $module.data(metadata.storedText)
            ;
            if( module.is.textEnabled() ) {
              if( module.is.active() && activeText) {
                module.verbose('Resetting active text', activeText);
                module.update.text(activeText);
              }
              else if(inactiveText) {
                module.verbose('Resetting inactive text', activeText);
                module.update.text(inactiveText);
              }
            }
          }
        },

        update: {
          text: function(text) {
            var
              currentText = module.get.text()
            ;
            if(text && text !== currentText) {
              module.debug('Updating text', text);
              if(settings.selector.text) {
                $module
                  .data(metadata.storedText, text)
                  .find(settings.selector.text)
                    .text(text)
                ;
              }
              else {
                $module
                  .data(metadata.storedText, text)
                  .html(text)
                ;
              }
            }
            else {
              module.debug('Text is already set, ignoring update', text);
            }
          }
        },

        setting: function(name, value) {
          module.debug('Changing setting', name, value);
          if( $.isPlainObject(name) ) {
            $.extend(true, settings, name);
          }
          else if(value !== undefined) {
            if($.isPlainObject(settings[name])) {
              $.extend(true, settings[name], value);
            }
            else {
              settings[name] = value;
            }
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.state.settings = {

  // module info
  name           : 'State',

  // debug output
  debug          : false,

  // verbose debug output
  verbose        : false,

  // namespace for events
  namespace      : 'state',

  // debug data includes performance
  performance    : true,

  // callback occurs on state change
  onActivate     : function() {},
  onDeactivate   : function() {},
  onChange       : function() {},

  // state test functions
  activateTest   : function() { return true; },
  deactivateTest : function() { return true; },

  // whether to automatically map default states
  automatic      : true,

  // activate / deactivate changes all elements instantiated at same time
  sync           : false,

  // default flash text duration, used for temporarily changing text of an element
  flashDuration  : 1000,

  // selector filter
  filter     : {
    text   : '.loading, .disabled',
    active : '.disabled'
  },

  context    : false,

  // error
  error: {
    beforeSend : 'The before send function has cancelled state change',
    method     : 'The method you called is not defined.'
  },

  // metadata
  metadata: {
    promise    : 'promise',
    storedText : 'stored-text'
  },

  // change class on state
  className: {
    active   : 'active',
    disabled : 'disabled',
    error    : 'error',
    loading  : 'loading',
    success  : 'success',
    warning  : 'warning'
  },

  selector: {
    // selector for text node
    text: false
  },

  defaults : {
    input: {
      disabled : true,
      loading  : true,
      active   : true
    },
    button: {
      disabled : true,
      loading  : true,
      active   : true,
    },
    progress: {
      active   : true,
      success  : true,
      warning  : true,
      error    : true
    }
  },

  states     : {
    active   : true,
    disabled : true,
    error    : true,
    loading  : true,
    success  : true,
    warning  : true
  },

  text     : {
    disabled   : false,
    flash      : false,
    hover      : false,
    active     : false,
    inactive   : false,
    activate   : false,
    deactivate : false
  }

};



})( jQuery, window, document );

/*!
 * # Fomantic-UI - Visibility
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

$.fn.visibility = function(parameters) {
  var
    $allModules    = $(this),
    moduleSelector = $allModules.selector || '',

    time           = new Date().getTime(),
    performance    = [],

    query          = arguments[0],
    methodInvoked  = (typeof query == 'string'),
    queryArguments = [].slice.call(arguments, 1),
    returnedValue,

    moduleCount    = $allModules.length,
    loadedCount    = 0
  ;

  $allModules
    .each(function() {
      var
        settings        = ( $.isPlainObject(parameters) )
          ? $.extend(true, {}, $.fn.visibility.settings, parameters)
          : $.extend({}, $.fn.visibility.settings),

        className       = settings.className,
        namespace       = settings.namespace,
        error           = settings.error,
        metadata        = settings.metadata,

        eventNamespace  = '.' + namespace,
        moduleNamespace = 'module-' + namespace,

        $window         = $(window),

        $module         = $(this),
        $context        = $(settings.context),

        $placeholder,

        instance        = $module.data(moduleNamespace),

        requestAnimationFrame = window.requestAnimationFrame
          || window.mozRequestAnimationFrame
          || window.webkitRequestAnimationFrame
          || window.msRequestAnimationFrame
          || function(callback) { setTimeout(callback, 0); },

        element         = this,
        disabled        = false,

        contextObserver,
        observer,
        module
      ;

      module = {

        initialize: function() {
          module.debug('Initializing', settings);

          module.setup.cache();

          if( module.should.trackChanges() ) {

            if(settings.type == 'image') {
              module.setup.image();
            }
            if(settings.type == 'fixed') {
              module.setup.fixed();
            }

            if(settings.observeChanges) {
              module.observeChanges();
            }
            module.bind.events();
          }

          module.save.position();
          if( !module.is.visible() ) {
            module.error(error.visible, $module);
          }

          if(settings.initialCheck) {
            module.checkVisibility();
          }
          module.instantiate();
        },

        instantiate: function() {
          module.debug('Storing instance', module);
          $module
            .data(moduleNamespace, module)
          ;
          instance = module;
        },

        destroy: function() {
          module.verbose('Destroying previous module');
          if(observer) {
            observer.disconnect();
          }
          if(contextObserver) {
            contextObserver.disconnect();
          }
          $window
            .off('load'   + eventNamespace, module.event.load)
            .off('resize' + eventNamespace, module.event.resize)
          ;
          $context
            .off('scroll'       + eventNamespace, module.event.scroll)
            .off('scrollchange' + eventNamespace, module.event.scrollchange)
          ;
          if(settings.type == 'fixed') {
            module.resetFixed();
            module.remove.placeholder();
          }
          $module
            .off(eventNamespace)
            .removeData(moduleNamespace)
          ;
        },

        observeChanges: function() {
          if('MutationObserver' in window) {
            contextObserver = new MutationObserver(module.event.contextChanged);
            observer        = new MutationObserver(module.event.changed);
            contextObserver.observe(document, {
              childList : true,
              subtree   : true
            });
            observer.observe(element, {
              childList : true,
              subtree   : true
            });
            module.debug('Setting up mutation observer', observer);
          }
        },

        bind: {
          events: function() {
            module.verbose('Binding visibility events to scroll and resize');
            if(settings.refreshOnLoad) {
              $window
                .on('load'   + eventNamespace, module.event.load)
              ;
            }
            $window
              .on('resize' + eventNamespace, module.event.resize)
            ;
            // pub/sub pattern
            $context
              .off('scroll'      + eventNamespace)
              .on('scroll'       + eventNamespace, module.event.scroll)
              .on('scrollchange' + eventNamespace, module.event.scrollchange)
            ;
          }
        },

        event: {
          changed: function(mutations) {
            module.verbose('DOM tree modified, updating visibility calculations');
            module.timer = setTimeout(function() {
              module.verbose('DOM tree modified, updating sticky menu');
              module.refresh();
            }, 100);
          },
          contextChanged: function(mutations) {
            [].forEach.call(mutations, function(mutation) {
              if(mutation.removedNodes) {
                [].forEach.call(mutation.removedNodes, function(node) {
                  if(node == element || $(node).find(element).length > 0) {
                    module.debug('Element removed from DOM, tearing down events');
                    module.destroy();
                  }
                });
              }
            });
          },
          resize: function() {
            module.debug('Window resized');
            if(settings.refreshOnResize) {
              requestAnimationFrame(module.refresh);
            }
          },
          load: function() {
            module.debug('Page finished loading');
            requestAnimationFrame(module.refresh);
          },
          // publishes scrollchange event on one scroll
          scroll: function() {
            if(settings.throttle) {
              clearTimeout(module.timer);
              module.timer = setTimeout(function() {
                $context.triggerHandler('scrollchange' + eventNamespace, [ $context.scrollTop() ]);
              }, settings.throttle);
            }
            else {
              requestAnimationFrame(function() {
                $context.triggerHandler('scrollchange' + eventNamespace, [ $context.scrollTop() ]);
              });
            }
          },
          // subscribes to scrollchange
          scrollchange: function(event, scrollPosition) {
            module.checkVisibility(scrollPosition);
          },
        },

        precache: function(images, callback) {
          if (!(images instanceof Array)) {
            images = [images];
          }
          var
            imagesLength  = images.length,
            loadedCounter = 0,
            cache         = [],
            cacheImage    = document.createElement('img'),
            handleLoad    = function() {
              loadedCounter++;
              if (loadedCounter >= images.length) {
                if ($.isFunction(callback)) {
                  callback();
                }
              }
            }
          ;
          while (imagesLength--) {
            cacheImage         = document.createElement('img');
            cacheImage.onload  = handleLoad;
            cacheImage.onerror = handleLoad;
            cacheImage.src     = images[imagesLength];
            cache.push(cacheImage);
          }
        },

        enableCallbacks: function() {
          module.debug('Allowing callbacks to occur');
          disabled = false;
        },

        disableCallbacks: function() {
          module.debug('Disabling all callbacks temporarily');
          disabled = true;
        },

        should: {
          trackChanges: function() {
            if(methodInvoked) {
              module.debug('One time query, no need to bind events');
              return false;
            }
            module.debug('Callbacks being attached');
            return true;
          }
        },

        setup: {
          cache: function() {
            module.cache = {
              occurred : {},
              screen   : {},
              element  : {},
            };
          },
          image: function() {
            var
              src = $module.data(metadata.src)
            ;
            if(src) {
              module.verbose('Lazy loading image', src);
              settings.once           = true;
              settings.observeChanges = false;

              // show when top visible
              settings.onOnScreen = function() {
                module.debug('Image on screen', element);
                module.precache(src, function() {
                  module.set.image(src, function() {
                    loadedCount++;
                    if(loadedCount == moduleCount) {
                      settings.onAllLoaded.call(this);
                    }
                    settings.onLoad.call(this);
                  });
                });
              };
            }
          },
          fixed: function() {
            module.debug('Setting up fixed');
            settings.once           = false;
            settings.observeChanges = false;
            settings.initialCheck   = true;
            settings.refreshOnLoad  = true;
            if(!parameters.transition) {
              settings.transition = false;
            }
            module.create.placeholder();
            module.debug('Added placeholder', $placeholder);
            settings.onTopPassed = function() {
              module.debug('Element passed, adding fixed position', $module);
              module.show.placeholder();
              module.set.fixed();
              if(settings.transition) {
                if($.fn.transition !== undefined) {
                  $module.transition(settings.transition, settings.duration);
                }
              }
            };
            settings.onTopPassedReverse = function() {
              module.debug('Element returned to position, removing fixed', $module);
              module.hide.placeholder();
              module.remove.fixed();
            };
          }
        },

        create: {
          placeholder: function() {
            module.verbose('Creating fixed position placeholder');
            $placeholder = $module
              .clone(false)
              .css('display', 'none')
              .addClass(className.placeholder)
              .insertAfter($module)
            ;
          }
        },

        show: {
          placeholder: function() {
            module.verbose('Showing placeholder');
            $placeholder
              .css('display', 'block')
              .css('visibility', 'hidden')
            ;
          }
        },
        hide: {
          placeholder: function() {
            module.verbose('Hiding placeholder');
            $placeholder
              .css('display', 'none')
              .css('visibility', '')
            ;
          }
        },

        set: {
          fixed: function() {
            module.verbose('Setting element to fixed position');
            $module
              .addClass(className.fixed)
              .css({
                position : 'fixed',
                top      : settings.offset + 'px',
                left     : 'auto',
                zIndex   : settings.zIndex
              })
            ;
            settings.onFixed.call(element);
          },
          image: function(src, callback) {
            $module
              .attr('src', src)
            ;
            if(settings.transition) {
              if( $.fn.transition !== undefined) {
                if($module.hasClass(className.visible)) {
                  module.debug('Transition already occurred on this image, skipping animation');
                  return;
                }
                $module.transition(settings.transition, settings.duration, callback);
              }
              else {
                $module.fadeIn(settings.duration, callback);
              }
            }
            else {
              $module.show();
            }
          }
        },

        is: {
          onScreen: function() {
            var
              calculations   = module.get.elementCalculations()
            ;
            return calculations.onScreen;
          },
          offScreen: function() {
            var
              calculations   = module.get.elementCalculations()
            ;
            return calculations.offScreen;
          },
          visible: function() {
            if(module.cache && module.cache.element) {
              return !(module.cache.element.width === 0 && module.cache.element.offset.top === 0);
            }
            return false;
          },
          verticallyScrollableContext: function() {
            var
              overflowY = ($context.get(0) !== window)
                ? $context.css('overflow-y')
                : false
            ;
            return (overflowY == 'auto' || overflowY == 'scroll');
          },
          horizontallyScrollableContext: function() {
            var
              overflowX = ($context.get(0) !== window)
                ? $context.css('overflow-x')
                : false
            ;
            return (overflowX == 'auto' || overflowX == 'scroll');
          }
        },

        refresh: function() {
          module.debug('Refreshing constants (width/height)');
          if(settings.type == 'fixed') {
            module.resetFixed();
          }
          module.reset();
          module.save.position();
          if(settings.checkOnRefresh) {
            module.checkVisibility();
          }
          settings.onRefresh.call(element);
        },

        resetFixed: function () {
          module.remove.fixed();
          module.remove.occurred();
        },

        reset: function() {
          module.verbose('Resetting all cached values');
          if( $.isPlainObject(module.cache) ) {
            module.cache.screen = {};
            module.cache.element = {};
          }
        },

        checkVisibility: function(scroll) {
          module.verbose('Checking visibility of element', module.cache.element);

          if( !disabled && module.is.visible() ) {

            // save scroll position
            module.save.scroll(scroll);

            // update calculations derived from scroll
            module.save.calculations();

            // percentage
            module.passed();

            // reverse (must be first)
            module.passingReverse();
            module.topVisibleReverse();
            module.bottomVisibleReverse();
            module.topPassedReverse();
            module.bottomPassedReverse();

            // one time
            module.onScreen();
            module.offScreen();
            module.passing();
            module.topVisible();
            module.bottomVisible();
            module.topPassed();
            module.bottomPassed();

            // on update callback
            if(settings.onUpdate) {
              settings.onUpdate.call(element, module.get.elementCalculations());
            }
          }
        },

        passed: function(amount, newCallback) {
          var
            calculations   = module.get.elementCalculations()
          ;
          // assign callback
          if(amount && newCallback) {
            settings.onPassed[amount] = newCallback;
          }
          else if(amount !== undefined) {
            return (module.get.pixelsPassed(amount) > calculations.pixelsPassed);
          }
          else if(calculations.passing) {
            $.each(settings.onPassed, function(amount, callback) {
              if(calculations.bottomVisible || calculations.pixelsPassed > module.get.pixelsPassed(amount)) {
                module.execute(callback, amount);
              }
              else if(!settings.once) {
                module.remove.occurred(callback);
              }
            });
          }
        },

        onScreen: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onOnScreen,
            callbackName = 'onScreen'
          ;
          if(newCallback) {
            module.debug('Adding callback for onScreen', newCallback);
            settings.onOnScreen = newCallback;
          }
          if(calculations.onScreen) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback !== undefined) {
            return calculations.onOnScreen;
          }
        },

        offScreen: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onOffScreen,
            callbackName = 'offScreen'
          ;
          if(newCallback) {
            module.debug('Adding callback for offScreen', newCallback);
            settings.onOffScreen = newCallback;
          }
          if(calculations.offScreen) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback !== undefined) {
            return calculations.onOffScreen;
          }
        },

        passing: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onPassing,
            callbackName = 'passing'
          ;
          if(newCallback) {
            module.debug('Adding callback for passing', newCallback);
            settings.onPassing = newCallback;
          }
          if(calculations.passing) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback !== undefined) {
            return calculations.passing;
          }
        },


        topVisible: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onTopVisible,
            callbackName = 'topVisible'
          ;
          if(newCallback) {
            module.debug('Adding callback for top visible', newCallback);
            settings.onTopVisible = newCallback;
          }
          if(calculations.topVisible) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return calculations.topVisible;
          }
        },

        bottomVisible: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onBottomVisible,
            callbackName = 'bottomVisible'
          ;
          if(newCallback) {
            module.debug('Adding callback for bottom visible', newCallback);
            settings.onBottomVisible = newCallback;
          }
          if(calculations.bottomVisible) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return calculations.bottomVisible;
          }
        },

        topPassed: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onTopPassed,
            callbackName = 'topPassed'
          ;
          if(newCallback) {
            module.debug('Adding callback for top passed', newCallback);
            settings.onTopPassed = newCallback;
          }
          if(calculations.topPassed) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return calculations.topPassed;
          }
        },

        bottomPassed: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onBottomPassed,
            callbackName = 'bottomPassed'
          ;
          if(newCallback) {
            module.debug('Adding callback for bottom passed', newCallback);
            settings.onBottomPassed = newCallback;
          }
          if(calculations.bottomPassed) {
            module.execute(callback, callbackName);
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return calculations.bottomPassed;
          }
        },

        passingReverse: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onPassingReverse,
            callbackName = 'passingReverse'
          ;
          if(newCallback) {
            module.debug('Adding callback for passing reverse', newCallback);
            settings.onPassingReverse = newCallback;
          }
          if(!calculations.passing) {
            if(module.get.occurred('passing')) {
              module.execute(callback, callbackName);
            }
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback !== undefined) {
            return !calculations.passing;
          }
        },


        topVisibleReverse: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onTopVisibleReverse,
            callbackName = 'topVisibleReverse'
          ;
          if(newCallback) {
            module.debug('Adding callback for top visible reverse', newCallback);
            settings.onTopVisibleReverse = newCallback;
          }
          if(!calculations.topVisible) {
            if(module.get.occurred('topVisible')) {
              module.execute(callback, callbackName);
            }
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return !calculations.topVisible;
          }
        },

        bottomVisibleReverse: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onBottomVisibleReverse,
            callbackName = 'bottomVisibleReverse'
          ;
          if(newCallback) {
            module.debug('Adding callback for bottom visible reverse', newCallback);
            settings.onBottomVisibleReverse = newCallback;
          }
          if(!calculations.bottomVisible) {
            if(module.get.occurred('bottomVisible')) {
              module.execute(callback, callbackName);
            }
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return !calculations.bottomVisible;
          }
        },

        topPassedReverse: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onTopPassedReverse,
            callbackName = 'topPassedReverse'
          ;
          if(newCallback) {
            module.debug('Adding callback for top passed reverse', newCallback);
            settings.onTopPassedReverse = newCallback;
          }
          if(!calculations.topPassed) {
            if(module.get.occurred('topPassed')) {
              module.execute(callback, callbackName);
            }
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return !calculations.onTopPassed;
          }
        },

        bottomPassedReverse: function(newCallback) {
          var
            calculations = module.get.elementCalculations(),
            callback     = newCallback || settings.onBottomPassedReverse,
            callbackName = 'bottomPassedReverse'
          ;
          if(newCallback) {
            module.debug('Adding callback for bottom passed reverse', newCallback);
            settings.onBottomPassedReverse = newCallback;
          }
          if(!calculations.bottomPassed) {
            if(module.get.occurred('bottomPassed')) {
              module.execute(callback, callbackName);
            }
          }
          else if(!settings.once) {
            module.remove.occurred(callbackName);
          }
          if(newCallback === undefined) {
            return !calculations.bottomPassed;
          }
        },

        execute: function(callback, callbackName) {
          var
            calculations = module.get.elementCalculations(),
            screen       = module.get.screenCalculations()
          ;
          callback = callback || false;
          if(callback) {
            if(settings.continuous) {
              module.debug('Callback being called continuously', callbackName, calculations);
              callback.call(element, calculations, screen);
            }
            else if(!module.get.occurred(callbackName)) {
              module.debug('Conditions met', callbackName, calculations);
              callback.call(element, calculations, screen);
            }
          }
          module.save.occurred(callbackName);
        },

        remove: {
          fixed: function() {
            module.debug('Removing fixed position');
            $module
              .removeClass(className.fixed)
              .css({
                position : '',
                top      : '',
                left     : '',
                zIndex   : ''
              })
            ;
            settings.onUnfixed.call(element);
          },
          placeholder: function() {
            module.debug('Removing placeholder content');
            if($placeholder) {
              $placeholder.remove();
            }
          },
          occurred: function(callback) {
            if(callback) {
              var
                occurred = module.cache.occurred
              ;
              if(occurred[callback] !== undefined && occurred[callback] === true) {
                module.debug('Callback can now be called again', callback);
                module.cache.occurred[callback] = false;
              }
            }
            else {
              module.cache.occurred = {};
            }
          }
        },

        save: {
          calculations: function() {
            module.verbose('Saving all calculations necessary to determine positioning');
            module.save.direction();
            module.save.screenCalculations();
            module.save.elementCalculations();
          },
          occurred: function(callback) {
            if(callback) {
              if(module.cache.occurred[callback] === undefined || (module.cache.occurred[callback] !== true)) {
                module.verbose('Saving callback occurred', callback);
                module.cache.occurred[callback] = true;
              }
            }
          },
          scroll: function(scrollPosition) {
            scrollPosition      = scrollPosition + settings.offset || $context.scrollTop() + settings.offset;
            module.cache.scroll = scrollPosition;
          },
          direction: function() {
            var
              scroll     = module.get.scroll(),
              lastScroll = module.get.lastScroll(),
              direction
            ;
            if(scroll > lastScroll && lastScroll) {
              direction = 'down';
            }
            else if(scroll < lastScroll && lastScroll) {
              direction = 'up';
            }
            else {
              direction = 'static';
            }
            module.cache.direction = direction;
            return module.cache.direction;
          },
          elementPosition: function() {
            var
              element = module.cache.element,
              screen  = module.get.screenSize()
            ;
            module.verbose('Saving element position');
            // (quicker than $.extend)
            element.fits          = (element.height < screen.height);
            element.offset        = $module.offset();
            element.width         = $module.outerWidth();
            element.height        = $module.outerHeight();
            // compensate for scroll in context
            if(module.is.verticallyScrollableContext()) {
              element.offset.top += $context.scrollTop() - $context.offset().top;
            }
            if(module.is.horizontallyScrollableContext()) {
              element.offset.left += $context.scrollLeft - $context.offset().left;
            }
            // store
            module.cache.element = element;
            return element;
          },
          elementCalculations: function() {
            var
              screen     = module.get.screenCalculations(),
              element    = module.get.elementPosition()
            ;
            // offset
            if(settings.includeMargin) {
              element.margin        = {};
              element.margin.top    = parseInt($module.css('margin-top'), 10);
              element.margin.bottom = parseInt($module.css('margin-bottom'), 10);
              element.top    = element.offset.top - element.margin.top;
              element.bottom = element.offset.top + element.height + element.margin.bottom;
            }
            else {
              element.top    = element.offset.top;
              element.bottom = element.offset.top + element.height;
            }

            // visibility
            element.topPassed        = (screen.top >= element.top);
            element.bottomPassed     = (screen.top >= element.bottom);
            element.topVisible       = (screen.bottom >= element.top) && !element.topPassed;
            element.bottomVisible    = (screen.bottom >= element.bottom) && !element.bottomPassed;
            element.pixelsPassed     = 0;
            element.percentagePassed = 0;

            // meta calculations
            element.onScreen  = ((element.topVisible || element.passing) && !element.bottomPassed);
            element.passing   = (element.topPassed && !element.bottomPassed);
            element.offScreen = (!element.onScreen);

            // passing calculations
            if(element.passing) {
              element.pixelsPassed     = (screen.top - element.top);
              element.percentagePassed = (screen.top - element.top) / element.height;
            }
            module.cache.element = element;
            module.verbose('Updated element calculations', element);
            return element;
          },
          screenCalculations: function() {
            var
              scroll = module.get.scroll()
            ;
            module.save.direction();
            module.cache.screen.top    = scroll;
            module.cache.screen.bottom = scroll + module.cache.screen.height;
            return module.cache.screen;
          },
          screenSize: function() {
            module.verbose('Saving window position');
            module.cache.screen = {
              height: $context.height()
            };
          },
          position: function() {
            module.save.screenSize();
            module.save.elementPosition();
          }
        },

        get: {
          pixelsPassed: function(amount) {
            var
              element = module.get.elementCalculations()
            ;
            if(amount.search('%') > -1) {
              return ( element.height * (parseInt(amount, 10) / 100) );
            }
            return parseInt(amount, 10);
          },
          occurred: function(callback) {
            return (module.cache.occurred !== undefined)
              ? module.cache.occurred[callback] || false
              : false
            ;
          },
          direction: function() {
            if(module.cache.direction === undefined) {
              module.save.direction();
            }
            return module.cache.direction;
          },
          elementPosition: function() {
            if(module.cache.element === undefined) {
              module.save.elementPosition();
            }
            return module.cache.element;
          },
          elementCalculations: function() {
            if(module.cache.element === undefined) {
              module.save.elementCalculations();
            }
            return module.cache.element;
          },
          screenCalculations: function() {
            if(module.cache.screen === undefined) {
              module.save.screenCalculations();
            }
            return module.cache.screen;
          },
          screenSize: function() {
            if(module.cache.screen === undefined) {
              module.save.screenSize();
            }
            return module.cache.screen;
          },
          scroll: function() {
            if(module.cache.scroll === undefined) {
              module.save.scroll();
            }
            return module.cache.scroll;
          },
          lastScroll: function() {
            if(module.cache.screen === undefined) {
              module.debug('First scroll event, no last scroll could be found');
              return false;
            }
            return module.cache.screen.top;
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
                module.error(error.method, query);
                return false;
              }
            });
          }
          if ( $.isFunction( found ) ) {
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

      if(methodInvoked) {
        if(instance === undefined) {
          module.initialize();
        }
        instance.save.scroll();
        instance.save.calculations();
        module.invoke(query);
      }
      else {
        if(instance !== undefined) {
          instance.invoke('destroy');
        }
        module.initialize();
      }
    })
  ;

  return (returnedValue !== undefined)
    ? returnedValue
    : this
  ;
};

$.fn.visibility.settings = {

  name                   : 'Visibility',
  namespace              : 'visibility',

  debug                  : false,
  verbose                : false,
  performance            : true,

  // whether to use mutation observers to follow changes
  observeChanges         : true,

  // check position immediately on init
  initialCheck           : true,

  // whether to refresh calculations after all page images load
  refreshOnLoad          : true,

  // whether to refresh calculations after page resize event
  refreshOnResize        : true,

  // should call callbacks on refresh event (resize, etc)
  checkOnRefresh         : true,

  // callback should only occur one time
  once                   : true,

  // callback should fire continuously whe evaluates to true
  continuous             : false,

  // offset to use with scroll top
  offset                 : 0,

  // whether to include margin in elements position
  includeMargin          : false,

  // scroll context for visibility checks
  context                : window,

  // visibility check delay in ms (defaults to animationFrame)
  throttle               : false,

  // special visibility type (image, fixed)
  type                   : false,

  // z-index to use with visibility 'fixed'
  zIndex                 : '10',

  // image only animation settings
  transition             : 'fade in',
  duration               : 1000,

  // array of callbacks for percentage
  onPassed               : {},

  // standard callbacks
  onOnScreen             : false,
  onOffScreen            : false,
  onPassing              : false,
  onTopVisible           : false,
  onBottomVisible        : false,
  onTopPassed            : false,
  onBottomPassed         : false,

  // reverse callbacks
  onPassingReverse       : false,
  onTopVisibleReverse    : false,
  onBottomVisibleReverse : false,
  onTopPassedReverse     : false,
  onBottomPassedReverse  : false,

  // special callbacks for image
  onLoad                 : function() {},
  onAllLoaded            : function() {},

  // special callbacks for fixed position
  onFixed                : function() {},
  onUnfixed              : function() {},

  // utility callbacks
  onUpdate               : false, // disabled by default for performance
  onRefresh              : function(){},

  metadata : {
    src: 'src'
  },

  className: {
    fixed       : 'fixed',
    placeholder : 'constraint',
    visible     : 'visible'
  },

  error : {
    method  : 'The method you called is not defined.',
    visible : 'Element is hidden, you must call refresh after element becomes visible'
  }

};

})( jQuery, window, document );
