<script lang="ts" setup>
import {computed, nextTick, onMounted, shallowRef, useTemplateRef, watch} from 'vue';
import SvgIcon from './SvgIcon.vue';
import {createTippy} from '../modules/tippy.ts';

type MergeStyle = {
  name: string,
  textDoMerge: string,
  textConfirmMerge?: string,
  textDescription: string,
  textAutoMerge?: string,
  textConfirmAutoMerge?: string,
  textBypassMerge?: string,
  textConfirmBypassMerge?: string,
  mergeTitleFieldText?: string,
  mergeMessageFieldText?: string,
  hideMergeMessageTexts?: boolean,
  hideAutoMerge: boolean,
};

type MergeForm = {
  allOverridableChecksOk: boolean,
  baseLink: string,
  canMergeNow: boolean,
  defaultMergeStyle: string,
  isReady: boolean,
  mergeMessageFieldPlaceHolder: string,
  mergeStyles: MergeStyle[],
  pullHeadCommitID: string,
  showMergeInstructions: boolean,
  showPullCommands: boolean,
  textBypassRules: string,
  textCancel: string,
  textCmdHint: string,
  textCmdMergeHint: string,
  textMergeBlocked: string,
  textMergeCommitId: string,
  textMergeTitle: string,
  textSelectMergeStyle: string,
};

const props = defineProps<{
  mergeFormProps: MergeForm,
}>();
const emit = defineEmits<{mergeStyleChange: [style: string]}>();

const mergeStyleManuallyMerged = 'manually-merged';

const mergeForm = props.mergeFormProps;

const mergeTitleFieldValue = shallowRef<string | undefined>('');
const mergeMessageFieldValue = shallowRef<string | undefined>('');
const forceMerge = shallowRef(false);

const findMergeStyle = (name: string) => mergeForm.mergeStyles.find((msd) => msd.name === name);
const mergeStyle = shallowRef((findMergeStyle(mergeForm.defaultMergeStyle) ?? mergeForm.mergeStyles[0]).name);
const mergeStyleDetail = computed(() => findMergeStyle(mergeStyle.value)!);

const showActionForm = shallowRef(false);
const actionForm = useTemplateRef<HTMLFormElement>('actionForm');
const mergeButton = useTemplateRef<HTMLButtonElement>('mergeButton');
const menuTrigger = useTemplateRef<HTMLButtonElement>('menuTrigger');
const menuPanel = useTemplateRef<HTMLDivElement>('menuPanel');

const autoMergeWhenSucceed = computed(() => !mergeStyleDetail.value.hideAutoMerge && !forceMerge.value);
const actionDisabled = computed(() => !autoMergeWhenSucceed.value && !mergeForm.canMergeNow && mergeStyle.value !== mergeStyleManuallyMerged);

const buttonTexts = computed(() => {
  const detail = mergeStyleDetail.value;
  if (autoMergeWhenSucceed.value) return {button: detail.textAutoMerge, confirm: detail.textConfirmAutoMerge};
  if (forceMerge.value && detail.textBypassMerge) return {button: detail.textBypassMerge, confirm: detail.textConfirmBypassMerge};
  return {button: detail.textDoMerge, confirm: detail.textConfirmMerge ?? detail.textDoMerge};
});

const mergeButtonStyleClass = computed(() => mergeForm.isReady && mergeStyle.value !== mergeStyleManuallyMerged ? 'green' : '');

watch(mergeStyle, (val) => {
  forceMerge.value = false;
  emit('mergeStyleChange', val);
});

onMounted(() => {
  if (!menuTrigger.value) return;
  const menuTippy = createTippy(menuTrigger.value, {
    content: menuPanel.value!,
    getReferenceClientRect: () => menuTrigger.value!.parentElement!.getBoundingClientRect(),
    theme: 'menu',
    arrow: false,
    maxWidth: 400,
    limitSizeToViewport: {vertical: true},
    placement: 'bottom-start',
    trigger: 'click',
    interactive: true,
    hideOnClick: true,
  });
  menuPanel.value!.addEventListener('click', () => menuTippy.hide());
});

async function toggleActionForm(show: boolean) {
  showActionForm.value = show;
  if (show) {
    mergeTitleFieldValue.value = mergeStyleDetail.value.mergeTitleFieldText;
    mergeMessageFieldValue.value = mergeStyleDetail.value.mergeMessageFieldText;
  }
  await nextTick();
  (show ? actionForm.value!.querySelector<HTMLElement>('input:not([type="hidden"]), button[type="submit"]')! : mergeButton.value!).focus();
}
</script>

<template>
  <div>
    <div class="ui checkbox merge-box-bypass" v-if="!showActionForm && mergeForm.canMergeNow && !mergeForm.allOverridableChecksOk && mergeStyle !== mergeStyleManuallyMerged">
      <input type="checkbox" v-model="forceMerge" id="merge-bypass-rules">
      <label for="merge-bypass-rules">{{ mergeForm.textBypassRules }}</label>
    </div>

    <form ref="actionForm" class="ui form form-fetch-action" v-if="showActionForm" :action="mergeForm.baseLink+'/merge'" method="post">
      <input type="hidden" name="head_commit_id" v-model="mergeForm.pullHeadCommitID">
      <input type="hidden" name="merge_when_checks_succeed" v-model="autoMergeWhenSucceed">
      <input type="hidden" name="force_merge" v-model="forceMerge">

      <template v-if="!mergeStyleDetail.hideMergeMessageTexts">
        <div class="field">
          <input type="text" name="merge_title_field" :aria-label="mergeForm.textMergeTitle" v-model="mergeTitleFieldValue">
        </div>
        <div class="field">
          <textarea name="merge_message_field" rows="5" :placeholder="mergeForm.mergeMessageFieldPlaceHolder" v-model="mergeMessageFieldValue"/>
        </div>
      </template>

      <div class="field" v-if="mergeStyle === mergeStyleManuallyMerged">
        <input type="text" name="merge_commit_id" :placeholder="mergeForm.textMergeCommitId" required>
      </div>

      <div class="flex-text-block merge-box-actions">
        <button class="ui button" :class="forceMerge ? 'red' : mergeButtonStyleClass" type="submit" name="do" :value="mergeStyle" :disabled="actionDisabled">
          {{ buttonTexts.confirm }}
        </button>

        <button class="ui button merge-cancel" type="button" @click="toggleActionForm(false)">
          {{ mergeForm.textCancel }}
        </button>
      </div>
    </form>

    <div v-show="!showActionForm" class="flex-text-block merge-box-actions">
      <div class="ui buttons" :class="mergeButtonStyleClass" :data-tooltip-content="actionDisabled ? mergeForm.textMergeBlocked : undefined">
        <button ref="mergeButton" class="ui button" type="button" :disabled="actionDisabled" @click="toggleActionForm(true)">
          {{ buttonTexts.button }}
        </button>
        <button v-if="mergeForm.mergeStyles.length > 1" ref="menuTrigger" class="ui icon button" type="button" :aria-label="mergeForm.textSelectMergeStyle">
          <svg-icon name="octicon-triangle-down" :size="14"/>
        </button>
      </div>

      <span v-if="mergeForm.showPullCommands" class="merge-box-cmd-hint">
        <template v-if="mergeForm.showMergeInstructions">{{ mergeForm.textCmdMergeHint }}</template>
        <a class="show-modal tw-whitespace-nowrap" href="" data-modal="#pull-merge-cmd-modal">{{ mergeForm.textCmdHint }}</a>
      </span>
    </div>
    <div v-if="mergeForm.mergeStyles.length > 1" ref="menuPanel" class="tippy-target merge-box-menu">
      <a v-for="msd in mergeForm.mergeStyles" :key="msd.name" class="item" role="menuitemradio" :aria-checked="msd.name === mergeStyle" @click="mergeStyle = msd.name">
        <svg-icon name="octicon-check"/>
        <div><strong>{{ msd.textDoMerge }}</strong><small>{{ msd.textDescription }}</small></div>
      </a>
    </div>
  </div>
</template>

<style scoped>
.ui.checkbox label {
  cursor: pointer;
}

.merge-box-bypass.ui.checkbox {
  display: flex;
  margin-bottom: 16px;
}

.merge-box-bypass.ui.checkbox label {
  color: var(--color-red);
}

.merge-box-actions {
  flex-wrap: wrap;
}
</style>
