<script lang="ts">
import {defineComponent} from 'vue';
import {SvgIcon} from '../svg.ts';
import {generateElemId} from '../utils/dom.ts';

type Extension = {
  ext: string,
  checked: boolean,
  count: number,
}

export default defineComponent({
  components: {SvgIcon},
  data: () => {
    const el = document.querySelector('#diff-extension-filter')!;
    return {
      menuVisible: false,
      extensions: [] as Array<Extension>,
      isFiltering: false,
      locale: {
        filter_by_file_extension: el.getAttribute('data-filter_by_file_extension'),
        select_all: el.getAttribute('data-select_all'),
        deselect_all: el.getAttribute('data-deselect_all'),
        apply: el.getAttribute('data-apply'),
      } as Record<string, string>,
      uniqueIdMenu: generateElemId('diff-extension-filter-menu-'),
    };
  },
  mounted() {
    document.body.addEventListener('click', this.onBodyClick, true);
    this.$el.addEventListener('keydown', this.onKeyDown);
  },
  unmounted() {
    document.body.removeEventListener('click', this.onBodyClick, true);
    this.$el.removeEventListener('keydown', this.onKeyDown);
  },
  methods: {
    onBodyClick(event: MouseEvent) {
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) {
        this.toggleMenu();
      }
    },
    onKeyDown(event: KeyboardEvent) {
      if (!this.menuVisible) return;
      const currentFocused = document.activeElement as HTMLElement;
      if (!this.$el.contains(currentFocused)) return;

      const menu = this.$el.querySelector('.menu') as HTMLElement;
      const focusableItems = Array.from(menu.querySelectorAll('[role="menuitem"]')) as HTMLElement[];

      if (!focusableItems.length) return;

      const currentIndex = focusableItems.indexOf(currentFocused.closest('[role="menuitem"]') as HTMLElement);

      switch (event.key) {
        case 'ArrowDown': {
          event.preventDefault();
          const nextIndex = currentIndex === -1 ? 0 : Math.min(currentIndex + 1, focusableItems.length - 1);
          this.focusElem(focusableItems[nextIndex], currentIndex >= 0 ? focusableItems[currentIndex] : null);
          break;
        }
        case 'ArrowUp': {
          event.preventDefault();
          const prevIndex = currentIndex === -1 ? focusableItems.length - 1 : Math.max(currentIndex - 1, 0);
          this.focusElem(focusableItems[prevIndex], currentIndex >= 0 ? focusableItems[currentIndex] : null);
          break;
        }
        case 'Escape':
          event.preventDefault();
          if (currentIndex >= 0) {
            focusableItems[currentIndex].tabIndex = -1;
          }
          this.toggleMenu();
          break;
      }
    },
    /** Focus given element */
    focusElem(elem: HTMLElement | null, prevElem: HTMLElement | null) {
      if (elem) {
        elem.tabIndex = 0;
        if (prevElem) prevElem.tabIndex = -1;
        // Focus the input/button inside the menuitem if it exists, otherwise focus the item itself
        const focusTarget = elem.querySelector('input, button') as HTMLElement || elem;
        focusTarget.focus();
      }
    },
    /**
     * Extract file extension from filename
     * Returns the extension with dot (e.g., ".ts", ".go")
     * Returns "(no extension)" for files without extension
     */
    getExtension(filename: string): string {
      const lastDot = filename.lastIndexOf('.');
      if (lastDot === -1 || lastDot === 0) {
        return '(no extension)';
      }
      return filename.substring(lastDot);
    },
    /**
     * Scan all diff-file-box elements and build extension list
     * Check current visibility state and set checked state accordingly
     */
    scanExtensions() {
      const extensionMap = new Map<string, {total: number, visible: number}>();
      const fileBoxes = document.querySelectorAll('#diff-file-boxes .diff-file-box[data-new-filename]');

      let hiddenCount = 0;
      fileBoxes.forEach((box) => {
        const filename = (box as HTMLElement).getAttribute('data-new-filename') || '';
        const ext = this.getExtension(filename);
        const isHidden = (box as HTMLElement).classList.contains('tw-hidden');
        if (!extensionMap.has(ext)) {
          extensionMap.set(ext, {total: 0, visible: 0});
        }
        const stats = extensionMap.get(ext)!;
        stats.total += 1;
        if (!isHidden) {
          stats.visible += 1;
        } else {
          hiddenCount += 1;
        }
      });

      this.extensions = Array.from(extensionMap.entries())
        .map(([ext, stats]) => ({
          ext,
          checked: stats.visible > 0,
          count: stats.total,
        }))
        .sort((a, b) => b.count - a.count);

      this.isFiltering = hiddenCount > 0;
    },
    /**
     * Open dropdown, rescan extensions
     */
    toggleMenu(refocus = true) {
      this.menuVisible = !this.menuVisible;
      if (this.menuVisible) {
        this.scanExtensions();
      } else if (refocus) {
        const button = this.$refs.expandBtn as HTMLElement;
        button.tabIndex = 0;
        button.focus();
      }
    },
    /**
     * Select all extensions
     */
    selectAll() {
      for (const ext of this.extensions) {
        ext.checked = true;
      }
    },
    /**
     * Deselect all extensions
     */
    deselectAll() {
      for (const ext of this.extensions) {
        ext.checked = false;
      }
    },
    /**
     * Apply the filter: hide/show diff-file-box elements based on checked extensions
     */
    applyFilter() {
      const checkedExtensions = new Set(this.extensions.filter((e) => e.checked).map((e) => e.ext));
      const fileBoxes = document.querySelectorAll('#diff-file-boxes .diff-file-box[data-new-filename]');
      let hiddenCount = 0;

      fileBoxes.forEach((box) => {
        const filename = (box as HTMLElement).getAttribute('data-new-filename') || '';
        const ext = this.getExtension(filename);
        const isChecked = checkedExtensions.has(ext);

        if (isChecked) {
          (box as HTMLElement).classList.remove('tw-hidden');
        } else {
          (box as HTMLElement).classList.add('tw-hidden');
          hiddenCount += 1;
        }
      });

      this.isFiltering = hiddenCount > 0;
      this.toggleMenu(false);
    },
  },
});
</script>
<template>
  <div class="ui scrolling dropdown custom diff-file-extension-filter">
    <button
      ref="expandBtn"
      class="ui tiny basic button"
      :class="{'diff-ext-filter-btn-active': isFiltering}"
      @click="toggleMenu()"
      :data-tooltip-content="locale.filter_by_file_extension"
      aria-haspopup="true"
      :aria-label="locale.filter_by_file_extension"
      :aria-controls="uniqueIdMenu"
    >
      <svg-icon name="octicon-filter"/>
    </button>
    <!-- this dropdown is not managed by Fomantic UI, so it needs some classes like "transition" explicitly -->
    <div class="left menu transition" :id="uniqueIdMenu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="header">{{ locale.filter_by_file_extension }}</div>
      <div class="ui divider tw-mt-2 tw-mb-0"/>
      <div class="ui form">
        <!-- Extension checkboxes -->
        <div class="grouped fields">
          <template v-for="ext in extensions" :key="ext.ext">
            <div class="field" role="menuitem" tabindex="-1">
              <div class="ui checkbox">
                <input
                  type="checkbox"
                  :id="`ext-filter-${ext.ext}`"
                  v-model="ext.checked"
                />
                <label :for="`ext-filter-${ext.ext}`" class="tw-cursor-pointer">
                  <span class="tw-font-mono">{{ ext.ext }}</span>
                  <span class="tw-text-text-light-2"> ({{ ext.count }})</span>
                </label>
              </div>
            </div>
          </template>
        </div>
      </div>

      <!-- Select all / Deselect all buttons -->
      <div class="ui divider tw-my-2"/>
      <div class="tw-flex tw-items-center tw-justify-center tw-gap-4 tw-px-2 tw-py-1">
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="selectAll()">{{ locale.select_all }}</button>
        <button type="button" class="diff-ext-text-btn" tabindex="-1" role="menuitem" @click="deselectAll()">{{ locale.deselect_all }}</button>
      </div>

      <!-- Apply button -->
      <div class="ui divider tw-my-2"/>
      <button class="ui button fluid" tabindex="-1" role="menuitem" @click="applyFilter()">
        {{ locale.apply }}
      </button>
    </div>
  </div>
</template>
<style scoped>
  .ui.dropdown.diff-file-extension-filter .menu {
    margin-top: 0.25em;
    overflow-x: hidden;
    max-height: 450px;
    padding: 0.75rem;
    padding-top: 0.5rem;
  }

  .ui.dropdown.diff-file-extension-filter .menu > .header {
    margin-top: 0;
    padding-top: 0;
  }

  .ui.dropdown.diff-file-extension-filter .menu .ui.form {
    margin: 0;
  }

  .ui.dropdown.diff-file-extension-filter .grouped.fields > .field {
    margin-bottom: 0.5rem;
  }

  .ui.dropdown.diff-file-extension-filter .grouped.fields > .field:last-child {
    margin-bottom: 0;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-filter-btn-active {
    outline: 1px solid currentColor;
    outline-offset: -1px;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-text-btn {
    background: none;
    border: none;
    padding: 0;
    color: var(--color-primary);
    cursor: pointer;
    font-size: inherit;
    text-align: center;
  }

  .ui.dropdown.diff-file-extension-filter .diff-ext-text-btn:hover {
    text-decoration: underline;
  }
</style>
