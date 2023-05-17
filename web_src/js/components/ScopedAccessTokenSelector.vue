<template>
  <div class="scoped-access-token-category">
    <div class="field gt-pl-2">
      <label class="checkbox-label">
        <input ref="category" v-model="categorySelected" class="scope-checkbox" type="checkbox" name="scope" :value="'delete:' + category" @change="onCategoryInput">
        {{ category }}
      </label>
    </div>
    <div class="field gt-pl-4">
      <div class="inline field">
        <label class="checkbox-label">
          <input ref="read" v-model="readSelected" :disabled="categorySelected || writeSelected" class="scope-checkbox" type="checkbox" name="scope" :value="'read:' + category" @change="onIndividualInput">
          read:{{ category }}
        </label>
      </div>
      <div class="inline field">
        <label class="checkbox-label">
          <input ref="write" v-model="writeSelected" :disabled="categorySelected" class="scope-checkbox" type="checkbox" name="scope" :value="'write:' + category" @change="onIndividualInput">
          write:{{ category }}
        </label>
      </div>
      <div class="inline field">
        <label class="checkbox-label">
          <input ref="delete" v-model="deleteSelected" :disabled="categorySelected" class="scope-checkbox" type="checkbox" name="scope" :value="'delete:' + category" @change="onIndividualInput">
          delete:{{ category }}
        </label>
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
     * When entire category is toggled
     * @param {Event} event
     */
    onCategoryInput(event) {
      event.preventDefault();
      this.deleteSelected = this.$refs.category.checked;
      this.writeSelected = this.$refs.category.checked;
      this.readSelected = this.$refs.category.checked;
    },

    /**
     * When an individual level of category is toggled
     * @param {Event} event
     */
    onIndividualInput(event) {
      event.preventDefault();
      if (this.$refs.delete.checked) {
        this.readSelected = true;
        this.writeSelected = true;
        this.categorySelected = true;
      }
      if (this.$refs.write.checked) {
        this.readSelected = true;
      }
    },
  }
};

export default sfc;

/**
 * Initialize category toggle sections
 */
export function initScopedAccessTokenCategories() {
  for (const el of document.getElementsByTagName('scoped-access-token-category')) {
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

.checkbox-label {
  cursor: pointer;
}

.scope-checkbox {
  margin: 4px 5px 0 0;
}
</style>
