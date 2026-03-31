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
  },
  unmounted() {
    document.body.removeEventListener('click', this.onBodyClick, true);
  },
  methods: {
    onBodyClick(event: MouseEvent) {
      // close this menu on click outside of this element when the dropdown is currently visible opened
      if (this.$el.contains(event.target)) return;
      if (this.menuVisible) {
        this.toggleMenu();
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

      // Count extensions and track visibility
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
        }
      });

      // Convert to array and sort by count descending
      // checked = true if any files of this extension are visible
      this.extensions = Array.from(extensionMap.entries())
        .map(([ext, stats]) => ({
          ext,
          checked: stats.visible > 0,
          count: stats.total,
        }))
        .sort((a, b) => b.count - a.count);

      // Update filtering state based on current visibility
      let hiddenCount = 0;
      fileBoxes.forEach((box) => {
        if ((box as HTMLElement).classList.contains('tw-hidden')) {
          hiddenCount += 1;
        }
      });
      this.isFiltering = hiddenCount > 0;
    },
    /**
     * Open dropdown, rescan extensions
     */
    toggleMenu() {
      this.menuVisible = !this.menuVisible;
      if (this.menuVisible) {
        this.scanExtensions();
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

      // Update filtering state
      this.isFiltering = hiddenCount > 0;

      // Close the menu after applying
      this.menuVisible = false;
    },
  },
});
</script>
<template>
  <div class="ui scrolling dropdown custom diff-file-extension-filter">
    <button
      ref="expandBtn"
      class="ui tiny basic button tw-relative"
      :class="{'diff-ext-filter-btn-active': isFiltering}"
      @click="toggleMenu()"
      :data-tooltip-content="locale.filter_by_file_extension"
      aria-haspopup="true"
      :aria-label="locale.filter_by_file_extension"
      :aria-controls="uniqueIdMenu"
    >
      <svg-icon name="octicon-filter"/>
      <span v-if="isFiltering" class="filter-indicator-dot"/>
    </button>
    <!-- this dropdown is not managed by Fomantic UI, so it needs some classes like "transition" explicitly -->
    <div class="left menu transition" :id="uniqueIdMenu" :class="{visible: menuVisible}" v-show="menuVisible" v-cloak :aria-expanded="menuVisible ? 'true': 'false'">
      <div class="header">{{ locale.filter_by_file_extension }}</div>
      <div class="ui divider tw-mt-2 tw-mb-0"/>
      <div class="ui form">
        <!-- Extension checkboxes -->
        <div class="grouped fields">
          <template v-for="ext in extensions" :key="ext.ext">
            <div class="field">
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
        <button type="button" class="diff-ext-text-btn" @click="selectAll()">{{ locale.select_all }}</button>
        <button type="button" class="diff-ext-text-btn" @click="deselectAll()">{{ locale.deselect_all }}</button>
      </div>

      <!-- Apply button -->
      <div class="ui divider tw-my-2"/>
      <button class="ui button fluid" @click="applyFilter()">
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
    color: var(--color-red-700);
    border-color: var(--color-red-300);
    background: var(--color-red-50);
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

  .ui.dropdown.diff-file-extension-filter .filter-indicator-dot {
    position: absolute;
    top: 0.15rem;
    right: 0.15rem;
    width: 0.5rem;
    height: 0.5rem;
    border-radius: 9999px;
    background: var(--color-red-600);
    box-shadow: 0 0 0 2px var(--color-body);
  }
</style>
