import {onInputDebounce, toggleElem} from '../utils/dom.js';

export function initLabelSearchInput() {
    if (document.querySelector('.labels-filter-input')){
        document.querySelector('.labels-filter-input').addEventListener('input', onInputDebounce(() => {
            const dividers = document.querySelectorAll('[data-divider-index]');
            for (const divider of dividers) {
                const dividerIndex = divider.getAttribute('data-divider-index');
                let showDivider = false;
                for (const el of document.querySelectorAll(`[data-divider-group="${dividerIndex}"]`)) {
                    if (!el.classList.contains('filtered')) {
                        showDivider = true;
                    }
                }
                toggleElem(divider, showDivider);
            }
        }))
    }
}
