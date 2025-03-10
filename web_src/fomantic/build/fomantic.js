import './components/api.js';
import './components/dropdown.js';
import './components/modal.js';
import './components/search.js';

// Hard forked from Fomantic 2.8.7

// TODO: need to apply the patch from Makefile
// # fomantic uses "touchstart" as click event for some browsers, it's not ideal, so we force fomantic to always use "click" as click event
// $(SED_INPLACE) -e 's/clickEvent[ \t]*=/clickEvent = "click", unstableClickEvent =/g' $(FOMANTIC_WORK_DIR)/build/semantic.js
