<template>
  <div class="scoped-access-token-category">
    <div class="field gt-pl-2">
      <div class="ui checkbox">
        <input ref="category" v-model="categorySelected" class="enable-system" type="checkbox" name="scope" :value="'delete:' + category" @change="onCategoryInput">
        <label>{{ category }}</label>
      </div>
    </div>
    <div class="field gt-pl-4">
      <div class="field">
        <div class="ui checkbox">
          <input ref="read" v-model="readSelected" :disabled="categorySelected" class="enable-system" type="checkbox" name="scope" :value="'read:' + category" @change="onIndividualInput">
          <label>read:{{ category }}</label>
        </div>
      </div>
      <div class="field">
        <div class="ui checkbox">
          <input ref="write" v-model="writeSelected" :disabled="categorySelected" class="enable-system" type="checkbox" name="scope" :value="'write:' + category" @change="onIndividualInput">
          <label>write:{{ category }}</label>
        </div>
      </div>
      <div class="field">
        <div class="ui checkbox">
          <input ref="delete" v-model="deleteSelected" :disabled="categorySelected" class="enable-system" type="checkbox" name="scope" :value="'delete:' + category" @change="onIndividualInput">
          <label>delete:{{ category }}</label>
        </div>
      </div>
    </div>
  </div>
</template>

<script>

import {createApp} from 'vue';

const sfc = {
  name: 'ScopedAccessTokenSelector',

  props: {
    category: {
      type: String,
      required: true,
    },
  },

  data: () => ({
    categorySelected: false,
    readSelected: false,
    writeSelected: false,
    deleteSelected: false,
  }),

  methods: {
    /**
     * When entire category is selected
     */
    onCategoryInput(event) {
      event.preventDefault();
      this.readSelected = this.$refs.category.checked;
      this.writeSelected = this.$refs.category.checked;
      this.deleteSelected = this.$refs.category.checked;
    },

    /**
     * When entire category is selected
     */
    onIndividualInput(event) {
      event.preventDefault();
      this.categorySelected = this.$refs.read.checked && this.$refs.write.checked && this.$refs.delete.checked;
    },
  }
};

export default sfc;

export function initScopedAccessTokenCategories() {
  const els = [
    ...document.getElementsByTagName('scoped-access-token-category'),
  ];

  for (const el of els) {
    createApp(sfc, {
      category: el.getAttribute('category'),
    }).mount(el);
  }
}

</script>

<style scoped>
.scoped-access-token-category {
  padding-bottom: 20px;
}
</style>
