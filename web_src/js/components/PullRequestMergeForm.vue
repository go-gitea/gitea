<script>
import {SvgIcon} from '../svg.js';
import {toggleElem} from '../utils/dom.js';

const {csrfToken, pageData} = window.config;

export default {
  components: {SvgIcon},
  data: () => ({
    csrfToken,
    mergeForm: pageData.pullRequestMergeForm,

    mergeTitleFieldValue: '',
    mergeMessageFieldValue: '',
    deleteBranchAfterMerge: false,
    autoMergeWhenSucceed: false,

    mergeStyle: '',
    mergeStyleDetail: { // dummy only, these values will come from one of the mergeForm.mergeStyles
      hideMergeMessageTexts: false,
      textDoMerge: '',
      mergeTitleFieldText: '',
      mergeMessageFieldText: '',
      hideAutoMerge: false,
    },
    mergeStyleAllowedCount: 0,

    showMergeStyleMenu: false,
    showActionForm: false,
  }),
  computed: {
    mergeButtonStyleClass() {
      if (this.mergeForm.allOverridableChecksOk) return 'primary';
      return this.autoMergeWhenSucceed ? 'primary' : 'red';
    },
    forceMerge() {
      return this.mergeForm.canMergeNow && !this.mergeForm.allOverridableChecksOk;
    },
  },
  watch: {
    mergeStyle(val) {
      this.mergeStyleDetail = this.mergeForm.mergeStyles.find((e) => e.name === val);
      for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
        toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
      }
    }
  },
  created() {
    this.mergeStyleAllowedCount = this.mergeForm.mergeStyles.reduce((v, msd) => v + (msd.allowed ? 1 : 0), 0);

    let mergeStyle = this.mergeForm.mergeStyles.find((e) => e.allowed && e.name === this.mergeForm.defaultMergeStyle)?.name;
    if (!mergeStyle) mergeStyle = this.mergeForm.mergeStyles.find((e) => e.allowed)?.name;
    this.switchMergeStyle(mergeStyle, !this.mergeForm.canMergeNow);
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
    },
    switchMergeStyle(name, autoMerge = false) {
      this.mergeStyle = name;
      this.autoMergeWhenSucceed = autoMerge;
    },
    clearMergeMessage() {
      this.mergeMessageFieldValue = this.mergeForm.defaultMergeMessage;
    },
  },
};
</script>
<template>
  <!--
  if this component is shown, either the user is an admin (can do a merge without checks), or they are a writer who has the permission to do a merge
  if the user is a writer and can't do a merge now (canMergeNow==false), then only show the Auto Merge for them
  How to test the UI manually:
  * Method 1: manually set some variables in pull.tmpl, eg: {{$notAllOverridableChecksOk = true}} {{$canMergeNow = false}}
  * Method 2: make a protected branch, then set state=pending/success :
    curl -X POST ${root_url}/api/v1/repos/${owner}/${repo}/statuses/${sha} \
      -H "accept: application/json" -H "authorization: Basic $base64_auth" -H "Content-Type: application/json" \
      -d '{"context": "test/context", "description": "description", "state": "${state}", "target_url": "http://localhost"}'
  -->
  <div>
    <!-- eslint-disable-next-line vue/no-v-html -->
    <div v-if="mergeForm.hasPendingPullRequestMerge" v-html="mergeForm.hasPendingPullRequestMergeTip" class="ui info message"/>

    <div class="ui form" v-if="showActionForm">
      <form :action="mergeForm.baseLink+'/merge'" method="post">
        <input type="hidden" name="_csrf" :value="csrfToken">
        <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">
        <input type="hidden" name="merge_when_checks_succeed" v-model="autoMergeWhenSucceed">
        <input type="hidden" name="force_merge" v-model="forceMerge">

        <template v-if="!mergeStyleDetail.hideMergeMessageTexts">
          <div class="field">
            <input type="text" name="merge_title_field" v-model="mergeTitleFieldValue">
          </div>
          <div class="field">
            <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" v-model="mergeMessageFieldValue"/>
            <template v-if="mergeMessageFieldValue !== mergeForm.defaultMergeMessage">
              <button @click.prevent="clearMergeMessage" class="btn gt-mt-2 gt-p-2 interact-fg" :data-tooltip-content="mergeForm.textClearMergeMessageHint">
                {{ mergeForm.textClearMergeMessage }}
              </button>
            </template>
          </div>
        </template>

        <div class="field" v-if="mergeStyle === 'manually-merged'">
          <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId">
        </div>

        <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
          {{ mergeStyleDetail.textDoMerge }}
          <template v-if="autoMergeWhenSucceed">
            {{ mergeForm.textAutoMergeButtonWhenSucceed }}
          </template>
        </button>

        <button class="ui button merge-cancel" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>

        <div class="ui checkbox gt-ml-2" v-if="mergeForm.isPullBranchDeletable && !autoMergeWhenSucceed">
          <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
          <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
        </div>
      </form>
    </div>

    <div v-if="!showActionForm" class="gt-df">
      <!-- the merge button -->
      <div class="ui buttons merge-button" :class="[mergeForm.emptyCommit ? 'grey' : mergeForm.allOverridableChecksOk ? 'primary' : 'red']" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            {{ mergeStyleDetail.textDoMerge }}
            <template v-if="autoMergeWhenSucceed">
              {{ mergeForm.textAutoMergeButtonWhenSucceed }}
            </template>
          </span>
        </button>
        <div class="ui dropdown icon button" @click.stop="showMergeStyleMenu = !showMergeStyleMenu" v-if="mergeStyleAllowedCount>1">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <template v-for="msd in mergeForm.mergeStyles">
              <!-- if can merge now, show one action "merge now", and an action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && mergeForm.canMergeNow" :key="msd.name" @click.stop="switchMergeStyle(msd.name)">
                <div class="action-text">
                  {{ msd.textDoMerge }}
                </div>
                <div v-if="!msd.hideAutoMerge" class="auto-merge-small" @click.stop="switchMergeStyle(msd.name, true)">
                  <svg-icon name="octicon-clock" :size="14"/>
                  <div class="auto-merge-tip">
                    {{ mergeForm.textAutoMergeWhenSucceed }}
                  </div>
                </div>
              </div>

              <!-- if can NOT merge now, only show one action "auto merge when succeed" -->
              <div class="item" v-if="msd.allowed && !mergeForm.canMergeNow && !msd.hideAutoMerge" :key="msd.name" @click.stop="switchMergeStyle(msd.name, true)">
                <div class="action-text">
                  {{ msd.textDoMerge }} {{ mergeForm.textAutoMergeButtonWhenSucceed }}
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>

      <!-- the cancel auto merge button -->
      <form v-if="mergeForm.hasPendingPullRequestMerge" :action="mergeForm.baseLink+'/cancel_auto_merge'" method="post" class="gt-ml-4">
        <input type="hidden" name="_csrf" :value="csrfToken">
        <button class="ui button">
          {{ mergeForm.textAutoMergeCancelSchedule }}
        </button>
      </form>
    </div>
  </div>
</template>
<style scoped>
/* to keep UI the same, at the moment we are still using some Fomantic UI styles, but we do not use their scripts, so we need to fine tune some styles */
.ui.dropdown .menu.show {
  display: block;
}
.ui.checkbox label {
  cursor: pointer;
}

/* make the dropdown list left-aligned */
.ui.merge-button {
  position: relative;
}
.ui.merge-button .ui.dropdown {
  position: static;
}
.ui.merge-button > .ui.dropdown:last-child > .menu:not(.left) {
  left: 0;
  right: auto;
}
.ui.merge-button .ui.dropdown .menu > .item {
  display: flex;
  align-items: stretch;
  padding: 0 !important; /* polluted by semantic.css: .ui.dropdown .menu > .item { !important } */
}

/* merge style list item */
.action-text {
  padding: 0.8rem;
  flex: 1
}

.auto-merge-small {
  width: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}
.auto-merge-small .auto-merge-tip {
  display: none;
  left: 38px;
  top: -1px;
  bottom: -1px;
  position: absolute;
  align-items: center;
  color: var(--color-info-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-left: none;
  padding-right: 1rem;
}

.auto-merge-small:hover {
  color: var(--color-info-text);
  background-color: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
}

.auto-merge-small:hover .auto-merge-tip {
  display: flex;
}

</style>
