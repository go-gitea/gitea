<template>
  <div v-show="show">
    <div class="item" :class="item.isFile ? 'filewrapper p-1' : ''">
      <!-- Files -->
      <SvgIcon
        v-if="item.isFile"
        data-position="right center"
        name="octicon-file"
        class="svg-icon file"
      />
      <a
        v-if="item.isFile"
        class="file ellipsis"
        :href="item.isFile ? '#diff-' + item.file.NameHash : ''"
      >{{ item.name }}</a>
      <SvgIcon
        v-if="item.isFile"
        data-position="right center"
        :name="getIconForDiffType(item.file.Type)"
        :class="['svg-icon', getIconForDiffType(item.file.Type), 'status']"
      />

      <!-- Directories -->
      <div v-if="!item.isFile" class="directory p-1" @click.stop="handleClick(item.isFile)">
        <SvgIcon
          class="svg-icon"
          :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"
        />
        <SvgIcon
          class="svg-icon directory"
          name="octicon-file-directory-fill"
        />
        <span class="ellipsis">{{ item.name }}</span>
      </div>
      <div v-show="!collapsed">
        <DiffFileTreeItem v-for="childItem in item.children" :key="childItem.name" :item="childItem" class="list" />
      </div>
    </div>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';

export default {
  name: 'DiffFileTreeItem',
  components: {
    SvgIcon,
  },

  props: {
    item: {
      type: Object,
      required: true
    },
    show: {
      type: Boolean,
      required: false,
      default: true
    }
  },

  data: () => ({
    collapsed: false,
  }),
  methods: {
    handleClick(itemIsFile) {
      if (itemIsFile) {
        return;
      }
      this.$set(this, 'collapsed', !this.collapsed);
    },
    getIconForDiffType(pType) {
      const diffTypes = {
        1: 'octicon-diff-added',
        2: 'octicon-diff-modified',
        3: 'octicon-diff-removed',
        4: 'octicon-diff-renamed',
        5: 'octicon-diff-modified', // there is no octicon for copied, so modified should be ok
      };
      return diffTypes[pType];
    },
  },
};
</script>

<style scoped>
span.svg-icon.status {
  float: right;
}
span.svg-icon.file {
  color: var(--color-secondary-dark-7);
}

span.svg-icon.directory {
  color: var(--color-primary);
}

span.svg-icon.octicon-diff-modified {
  color: var(--color-yellow);
}

span.svg-icon.octicon-diff-added {
  color: var(--color-green);
}

span.svg-icon.octicon-diff-removed {
  color: var(--color-red);
}

span.svg-icon.octicon-diff-renamed {
  color: var(--color-teal);
}

.item.filewrapper {
  display: grid !important;
  grid-template-columns: 20px 7fr 1fr;
  padding-left: 18px !important;
}

.item.filewrapper:hover {
  color: var(--color-text);
  background: var(--color-hover);
  border-radius: 4px;
}

div.directory {
  display: grid;
  grid-template-columns: 18px 20px auto;
}

div.directory:hover {
  color: var(--color-text);
  background: var(--color-hover);
  border-radius: 4px;
}

div.list {
  padding-bottom: 0 !important;
  padding-top: inherit !important;
}

a {
  text-decoration: none;
}

a:hover {
  text-decoration: none;
}
</style>
