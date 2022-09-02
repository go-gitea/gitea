<template>
  <div v-show="show">
    <div class="item">
      <div>
        <SvgIcon
          v-if="item.isFile"
          data-position="right center"
          name="octicon-file"
          :class="[
            getDiffType(item.file.Type),
            'tooltip',
          ]"
        />
        <a
          v-if="item.isFile"
          class="file"
          :href="item.isFile ? '#diff-' + item.file.NameHash : ''"
        >{{ item.name }}</a>
        <div v-if="!item.isFile" @click.stop="handleClick(item.isFile)">
          <SvgIcon
            :name="collapsed ? 'octicon-chevron-right' : 'octicon-chevron-down'"
          />
          {{ item.name }}
        </div>
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
    getDiffType(pType) {
      const diffTypes = {
        1: 'add',
        2: 'modify',
        3: 'del',
        4: 'rename',
        5: 'copy',
      };
      return diffTypes[pType];
    },
  },
};
</script>

<style scoped>
span.modify {
  color: var(--color-yellow);
}

span.add {
  color: var(--color-green);
}

span.del {
  color: var(--color-red);
}

span.rename {
  color: var(--color-teal);
}
</style>
