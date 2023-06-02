<template>
  <div class="scoped-access-token-category">
    <div class="field gt-pl-2">
      <label class="checkbox-label">
        <input
          ref="category"
          v-model="categorySelected"
          class="scope-checkbox scoped-access-token-input"
          type="checkbox"
          name="scope"
          :value="'write:' + category"
          @input="onCategoryInput"
        >
        {{ category }}
      </label>
    </div>
    <div class="field gt-pl-4">
      <div class="inline field">
        <label class="checkbox-label">
          <input
            ref="read"
            v-model="readSelected"
            :disabled="disableIndividual || writeSelected"
            class="scope-checkbox scoped-access-token-input"
            type="checkbox"
            name="scope"
            :value="'read:' + category"
            @input="onIndividualInput"
          >
          read:{{ category }}
        </label>
      </div>
      <div class="inline field">
        <label class="checkbox-label">
          <input
            ref="write"
            v-model="writeSelected"
            :disabled="disableIndividual"
            class="scope-checkbox scoped-access-token-input"
            type="checkbox"
            name="scope"
            :value="'write:' + category"
            @input="onIndividualInput"
          >
          write:{{ category }}
        </label>
      </div>
    </div>
  </div>
</template>

<script>
import {createApp} from 'vue';
import {showElem} from '../utils/dom.js';

const sfc = {
  props: {
    category: {
      type: String,
      required: true,
    },
  },

  data: () => ({
    categorySelected: false,
    disableIndividual: false,
    readSelected: false,
    writeSelected: false,
  }),

  methods: {
    /**
     * When entire category is toggled
     * @param {Event} e
     */
    onCategoryInput(e) {
      e.preventDefault();
      this.disableIndividual = this.$refs.category.checked;
      this.writeSelected = this.$refs.category.checked;
      this.readSelected = this.$refs.category.checked;
    },

    /**
     * When an individual level of category is toggled
     * @param {Event} e
     */
    onIndividualInput(e) {
      e.preventDefault();
      if (this.$refs.write.checked) {
        this.readSelected = true;
      }
      this.categorySelected = this.$refs.write.checked;
    },
  }
};

export default sfc;

/**
 * Initialize category toggle sections
 */
export function initScopedAccessTokenCategories() {
  for (const el of document.getElementsByTagName('scoped-access-token-category')) {
    const category = el.getAttribute('category');
    createApp(sfc, {
      category,
    }).mount(el);
  }

  document.getElementById('scoped-access-submit')?.addEventListener('click', (e) => {
    e.preventDefault();
    // check that at least one scope has been selected
    for (const el of document.getElementsByClassName('scoped-access-token-input')) {
      if (el.checked) {
        document.getElementById('scoped-access-form').submit();
      }
    }
    // no scopes selected, show validation error
    showElem(document.getElementById('scoped-access-warning'));
  });
}

</script>

<style scoped>
.scoped-access-token-category {
  padding-top: 10px;
  padding-bottom: 10px;
}

.checkbox-label {
  cursor: pointer;
}

.scope-checkbox {
  margin: 4px 5px 0 0;
}
</style>
