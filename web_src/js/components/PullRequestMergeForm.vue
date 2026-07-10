<script lang="ts" setup>
import {computed, onMounted, onUnmounted, shallowRef, watch} from 'vue';
import {SvgIcon} from '../svg.ts';
import {toggleElem} from '../utils/dom.ts';

const props = defineProps<{
  mergeFormProps: any, // TODO: this is a huge object, need to be refactored in the future
}>();

const mergeStyleManuallyMerged = 'manually-merged';

const mergeForm = props.mergeFormProps;

const mergeTitleFieldValue = shallowRef('');
const mergeMessageFieldValue = shallowRef('');
const deleteBranchAfterMerge = shallowRef(false);
const bypassProtection = shallowRef(false);

const mergeStyle = shallowRef('');
const mergeStyleDetail = shallowRef({
  hideMergeMessageTexts: false,
  textDoMerge: '',
  textAutoMerge: '',
  mergeTitleFieldText: '',
  mergeMessageFieldText: '',
  hideAutoMerge: false,
});

const mergeStyleAllowedCount = computed(() => mergeForm.mergeStyles.reduce((v: number, msd: any) => v + (msd.allowed ? 1 : 0), 0));

const showMergeStyleMenu = shallowRef(false);
const showActionForm = shallowRef(false);

// the bypass checkbox is only meaningful when the user can bypass and there are overridable blockers
const showBypassProtection = computed(() => {
  return mergeForm.canBypassProtection && !mergeForm.allOverridableChecksOk;
});

const forceMerge = computed(() => {
  return showBypassProtection.value && bypassProtection.value;
});

// the merge mode is derived, not hand-managed: with overridable blockers present and no explicit bypass,
// the only valid action is to schedule an auto merge (unless the selected style has no auto merge, eg manual merge)
const autoMergeWhenSucceed = computed(() => {
  return !mergeForm.allOverridableChecksOk && !forceMerge.value && !mergeStyleDetail.value.hideAutoMerge;
});

const mergeButtonStyleClass = computed(() => {
  if (mergeStyle.value === mergeStyleManuallyMerged) return 'red';
  return forceMerge.value ? 'red' : 'primary';
});

const mergeSelectStyleClass = computed(() => {
  if (mergeForm.emptyCommit) return '';
  if (mergeStyle.value === mergeStyleManuallyMerged) return 'red';
  return forceMerge.value ? 'red' : 'primary';
});

watch(mergeStyle, (val) => {
  mergeStyleDetail.value = mergeForm.mergeStyles.find((e: any) => e.name === val);
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
  }
});

onMounted(() => {
  let defaultStyle = mergeForm.mergeStyles.find((e: any) => e.allowed && e.name === mergeForm.defaultMergeStyle)?.name;
  if (!defaultStyle) defaultStyle = mergeForm.mergeStyles.find((e: any) => e.allowed)?.name;
  mergeStyle.value = defaultStyle;

  document.addEventListener('mouseup', hideMergeStyleMenu);
});

onUnmounted(() => {
  document.removeEventListener('mouseup', hideMergeStyleMenu);
});

function hideMergeStyleMenu() {
  showMergeStyleMenu.value = false;
}

function toggleActionForm(show: boolean) {
  showActionForm.value = show;
  if (!show) return;
  deleteBranchAfterMerge.value = mergeForm.defaultDeleteBranchAfterMerge;
  mergeTitleFieldValue.value = mergeStyleDetail.value.mergeTitleFieldText;
  mergeMessageFieldValue.value = mergeStyleDetail.value.mergeMessageFieldText;
}

function selectMergeStyle(name: string) {
  // the dropdown only chooses the merge style; the merge mode (now / auto / bypass) is derived
  mergeStyle.value = name;
  showMergeStyleMenu.value = false;
}

function clearMergeMessage() {
  mergeMessageFieldValue.value = mergeForm.defaultMergeMessage;
}
</script>

<template>
  <!--
  if this component is shown, the user has permission to merge.
  the dropdown only chooses the merge style; the merge mode is derived:
  - no overridable blockers => merge now
  - overridable blockers, no bypass => enable auto merge (merge when checks succeed)
  - overridable blockers + bypass checkbox (only offered when the user can bypass) => merge now, skipping the blockers
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

    <!-- explicit opt-in to bypass branch protection, kept above the merge button like GitHub -->
    <div class="ui checkbox tw-mb-3" v-if="showBypassProtection">
      <input type="checkbox" v-model="bypassProtection" id="bypass-protection">
      <label for="bypass-protection" class="tw-text-red">{{ mergeForm.textBypassProtection }}</label>
    </div>

    <!-- another similar form is in pull.tmpl (manual merge)-->
    <form class="ui form form-fetch-action" v-if="showActionForm" :action="mergeForm.baseLink+'/merge'" method="post">
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
            <button @click.prevent="clearMergeMessage" class="btn tw-mt-1 tw-p-1 interact-fg" :data-tooltip-content="mergeForm.textClearMergeMessageHint">
              {{ mergeForm.textClearMergeMessage }}
            </button>
          </template>
        </div>
      </template>

      <div class="field" v-if="mergeStyle === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId">
      </div>

      <div class="flex-text-block tw-gap-3">
        <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
          <template v-if="autoMergeWhenSucceed">{{ mergeStyleDetail.textAutoMerge }}</template>
          <template v-else>{{ mergeStyleDetail.textDoMerge }}</template>
        </button>

        <button class="ui button merge-cancel" type="button" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>

        <div class="ui checkbox" v-if="mergeForm.isPullBranchDeletable">
          <input name="delete_branch_after_merge" type="checkbox" v-model="deleteBranchAfterMerge" id="delete-branch-after-merge">
          <label for="delete-branch-after-merge">{{ mergeForm.textDeleteBranch }}</label>
        </div>
      </div>
    </form>

    <div v-if="!showActionForm" class="tw-flex">
      <!-- the merge button -->
      <div class="ui buttons merge-button" :class="mergeSelectStyleClass" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            <template v-if="autoMergeWhenSucceed">{{ mergeStyleDetail.textAutoMerge }}</template>
            <template v-else>{{ mergeStyleDetail.textDoMerge }}</template>
          </span>
        </button>
        <!-- the dropdown only chooses the merge style; hidden when there is nothing to choose -->
        <div class="ui dropdown icon button" v-if="mergeStyleAllowedCount > 1" @click.stop="showMergeStyleMenu = !showMergeStyleMenu">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <template v-for="msd in mergeForm.mergeStyles">
              <div class="item" v-if="msd.allowed" :key="msd.name" @click.stop="selectMergeStyle(msd.name)">
                {{ msd.textDoMerge }}
              </div>
            </template>
          </div>
        </div>
      </div>

      <!-- the cancel auto merge button -->
      <form v-if="mergeForm.hasPendingPullRequestMerge" :action="mergeForm.baseLink+'/cancel_auto_merge'" method="post" class="tw-ml-4">
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
  padding: 0.8rem !important; /* polluted by semantic.css: .ui.dropdown .menu > .item { !important } */
}

</style>
