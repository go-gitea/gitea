<script lang="ts" setup>
import {computed, onMounted, onUnmounted, shallowRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {toggleElem} from '../utils/dom.ts';

type MergeStyle = {
  name: string,
  allowed: boolean,
  textDoMerge: string,
  textAutoMerge?: string,
  mergeTitleFieldText?: string,
  mergeMessageFieldText?: string,
  hideMergeMessageTexts?: boolean,
  hideAutoMerge: boolean,
};

type MergeForm = {
  allOverridableChecksOk: boolean,
  baseLink: string,
  canMergeNow: boolean,
  defaultDeleteBranchAfterMerge: boolean,
  defaultMergeMessage: string,
  defaultMergeStyle: string,
  emptyCommit: boolean,
  hasPendingPullRequestMerge: boolean,
  hasPendingPullRequestMergeTip: string,
  isPullBranchDeletable: boolean,
  mergeMessageFieldPlaceHolder: string,
  mergeStyles: MergeStyle[],
  pullHeadCommitID: string,
  showPullCommands: boolean,
  textAutoMergeCancelSchedule: string,
  textBypassRules: string,
  textCancel: string,
  textCmdHint: string,
  textCmdMergeHint: string,
  textClearMergeMessage: string,
  textClearMergeMessageHint: string,
  textDeleteBranch: string,
  textMergeCommitId: string,
};

const props = defineProps<{
  mergeFormProps: MergeForm,
}>();

const mergeStyleManuallyMerged = 'manually-merged';

const mergeForm = props.mergeFormProps;

const mergeTitleFieldValue = shallowRef<string | undefined>('');
const mergeMessageFieldValue = shallowRef<string | undefined>('');
const deleteBranchAfterMerge = shallowRef(false);
const forceMerge = shallowRef(false);

const mergeStyle = shallowRef('');
const mergeStyleDetail = shallowRef<MergeStyle>({name: '', allowed: false, textDoMerge: '', hideAutoMerge: false});

const allowedMergeStyles = mergeForm.mergeStyles.filter((msd) => msd.allowed);

const showMergeStyleMenu = shallowRef(false);
const showActionForm = shallowRef(false);

const autoMergeWhenSucceed = computed(() => !mergeStyleDetail.value.hideAutoMerge && !forceMerge.value);

const mergeButtonText = computed(() => autoMergeWhenSucceed.value ? mergeStyleDetail.value.textAutoMerge : mergeStyleDetail.value.textDoMerge);

const mergeButtonStyleClass = computed(() => {
  if (!mergeForm.canMergeNow || !mergeForm.allOverridableChecksOk) return '';
  return mergeStyle.value === mergeStyleManuallyMerged ? '' : 'primary';
});

watch(mergeStyle, (val) => {
  mergeStyleDetail.value = mergeForm.mergeStyles.find((e) => e.name === val)!;
  for (const elem of document.querySelectorAll('[data-pull-merge-style]')) {
    toggleElem(elem, elem.getAttribute('data-pull-merge-style') === val);
  }
});

onMounted(() => {
  let defaultStyle = mergeForm.mergeStyles.find((e) => e.allowed && e.name === mergeForm.defaultMergeStyle)?.name;
  if (!defaultStyle) defaultStyle = mergeForm.mergeStyles.find((e) => e.allowed)?.name;
  if (defaultStyle) mergeStyle.value = defaultStyle;

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
  mergeStyle.value = name;
  showMergeStyleMenu.value = false;
}

function clearMergeMessage() {
  mergeMessageFieldValue.value = mergeForm.defaultMergeMessage;
}
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

    <div class="ui checkbox tw-mb-3" v-if="mergeForm.canMergeNow && !mergeForm.allOverridableChecksOk">
      <input type="checkbox" v-model="forceMerge" id="merge-bypass-rules">
      <label class="tw-text-red" for="merge-bypass-rules">{{ mergeForm.textBypassRules }}</label>
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

      <div class="flex-text-block">
        <button class="ui button" :class="mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle">
          {{ mergeButtonText }}
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

    <div v-if="!showActionForm" class="flex-text-block tw-flex-wrap">
      <!-- the merge button -->
      <div class="ui buttons merge-button" :class="mergeForm.emptyCommit ? '' : mergeButtonStyleClass" @click="toggleActionForm(true)">
        <button class="ui button">
          <svg-icon name="octicon-git-merge"/>
          <span class="button-text">
            {{ mergeButtonText }}
          </span>
        </button>
        <div class="ui dropdown icon button" v-if="allowedMergeStyles.length > 1" @click.stop="showMergeStyleMenu = !showMergeStyleMenu">
          <svg-icon name="octicon-triangle-down" :size="14"/>
          <div class="menu" :class="{'show':showMergeStyleMenu}">
            <div class="item" v-for="msd in allowedMergeStyles" :key="msd.name" @click.stop="selectMergeStyle(msd.name)">
              {{ msd.textDoMerge }}
            </div>
          </div>
        </div>
      </div>

      <!-- the cancel auto merge button -->
      <form v-if="mergeForm.hasPendingPullRequestMerge" :action="mergeForm.baseLink+'/cancel_auto_merge'" method="post">
        <button class="ui button">
          {{ mergeForm.textAutoMergeCancelSchedule }}
        </button>
      </form>

      <span v-if="mergeForm.showPullCommands" class="tw-text-12 tw-text-text-light">
        {{ mergeForm.textCmdMergeHint }}
        <a class="show-modal" href="" data-modal="#pull-merge-cmd-modal">{{ mergeForm.textCmdHint }}</a>
      </span>
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

</style>
