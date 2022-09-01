<template>
  <!-- eslint-disable -->
  <div>
    <div v-for="item in children" class="item">
      <div>
        <i
          v-if="item.isFile"
          :class="['icon', 'file', 'status', getDiffType(item.file.Type), 'tooltip']"
          :data-content="getDiffType(item.file.Type)"
          data-position="right center"
          ></i>
        <a
          v-if="item.isFile"
          class="file mono tooltip"
          :data-content="item.Name"
          :href="item.isFile ? '#diff-' + item.file.NameHash : ''"
          >{{ item.name }}</a
        >
        <div v-if="!item.isFile">
          <i class="folder icon"></i>{{ item.name }}
        </div>
      </div>
      <div v-if="item.children.length > 0" class="list">
        <DiffFileTreeItem :children="item.children"> </DiffFileTreeItem>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'DiffFileTreeItem',
  components: {},

  props: {
    children: {
      type: Array,
      required: true,
    },
  },

  computed: {},

  watch: {},

  created() {},

  mounted() {},

  unmounted() {},

  methods: {
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
i.modify {
  color: var(--color-yellow);
}

i.add {
  color: var(--color-green);
}

i.del {
  color: var(--color-red);
}

i.rename {
  color: var(--color-teal);
}
</style>
