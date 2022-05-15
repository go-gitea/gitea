<template>
  <div>
    <div class="ui form" v-if="showActionForm">
      <form :action="mergeForm.baseLink+'/merge'" method="post">
        <input type="hidden" name="_csrf" :value="csrfToken">
        <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">

        <template v-if="!mergeStyleDetail.hideMergeMessageTexts">
          <div class="field">
            <input type="text" name="merge_title_field" v-model="mergeTitleFieldValue">
          </div>
          <div class="field">
            <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" v-model="mergeMessageFieldValue"/>
          </div>
        </template>

        <button class="ui button" :class="[mergeForm.allOverridableChecksOk?'green':'red']" type="submit" name="do" :value="mergeStyle">
          {{ mergeStyleDetail.textDoMerge }}
        </button>

        <button class="ui button merge-cancel" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>

        <div class="ui checkbox ml-2" v-if="mergeForm.isPullBranchDeletable">
          <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
          <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
        </div>
      </form>
    </div>

    <template v-if="!showActionForm">
      <div class="ui buttons merge-button" :class="[mergeForm.allOverridableChecksOk?'green':'red']" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">{{ mergeStyleDetail.textDoMerge }}</span>
        </button>
        <div class="ui dropdown icon button no-text" @click.stop="showMergeStyleMenu = !showMergeStyleMenu" v-if="mergeStyleAllowedCount>1">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <template v-for="msd in mergeForm.mergeStyles">
              <div class="item" v-if="msd.allowed" :key="msd.name" @click.stop="mergeStyle=msd.name">
                {{ msd.textDoMerge }}
              </div>
            </template>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script>
import {SvgIcon} from '../svg.js';

const {csrfToken, pageData} = window.config;

export default {
  name: 'PullRequestMergeForm',
  components: {
    SvgIcon,
  },

  data: () => ({
    csrfToken,
    mergeForm: pageData.pullRequestMergeForm,

    mergeTitleFieldValue: '',
    mergeMessageFieldValue: '',
    deleteBranchAfterMerge: false,

    mergeStyle: '',
    mergeStyleDetail: { // dummy only, these values will come from one of the mergeForm.mergeStyles
      hideMergeMessageTexts: false,
      textDoMerge: '',
      mergeTitleFieldText: '',
      mergeMessageFieldText: '',
    },
    mergeStyleAllowedCount: 0,

    showMergeStyleMenu: false,
    showActionForm: false,
  }),

  watch: {
    mergeStyle(val) {
      this.mergeStyleDetail = this.mergeForm.mergeStyles.find((e) => e.name === val);
    }
  },

  created() {
    this.mergeStyleAllowedCount = this.mergeForm.mergeStyles.reduce((v, msd) => v + (msd.allowed ? 1 : 0), 0);
    this.mergeStyle = this.mergeForm.mergeStyles.find((e) => e.allowed)?.name;
  },

  mounted() {
    document.addEventListener('mouseup', this.hideMergeStyleMenu);
  },

  unmounted() {
    document.removeEventListener('mouseup', this.hideMergeStyleMenu);
  },

  methods: {
    hideMergeStyleMenu() {
      this.showMergeStyleMenu = false;
    },
    toggleActionForm(show) {
      this.showActionForm = show;
      if (!show) return;
      this.deleteBranchAfterMerge = this.mergeForm.defaultDeleteBranchAfterMerge;
      this.mergeTitleFieldValue = this.mergeStyleDetail.mergeTitleFieldText;
      this.mergeMessageFieldValue = this.mergeStyleDetail.mergeMessageFieldText;
    }
  },
};
</script>

<style scoped>
/* to keep UI the same, at the moment we are still using some Fomantic UI styles, but we do not use their scripts, so we need to fine tune some styles */
.ui.dropdown .menu.show {
  display: block;
}
.ui.checkbox label {
  cursor: pointer;
}
</style>
