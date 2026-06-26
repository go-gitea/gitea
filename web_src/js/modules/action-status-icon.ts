import type {SvgName} from '../svg.ts';
import type {ActionsStatus} from './gitea-actions.ts';

export type ActionStatusIconVariant = 'circle-fill' | '';

export type ActionStatusIconSpec = {
  name: SvgName,
  colorClass: string,
};

// Keep in sync with templates/repo/icons/action_status.tmpl and ActionStatusIcon.vue.
export function getActionStatusIcon(status: ActionsStatus, iconVariant: ActionStatusIconVariant = ''): ActionStatusIconSpec {
  const circleFill = iconVariant === 'circle-fill';
  switch (status) {
    case 'success':
      return {name: circleFill ? 'octicon-check-circle-fill' : 'octicon-check', colorClass: 'tw-text-green'};
    case 'skipped':
      return {name: 'octicon-skip', colorClass: 'tw-text-text-light'};
    case 'cancelled':
      return {name: 'octicon-stop', colorClass: 'tw-text-text-light'};
    case 'waiting':
      return {name: 'octicon-circle', colorClass: 'tw-text-text-light'};
    case 'blocked':
      return {name: 'octicon-blocked', colorClass: 'tw-text-yellow'};
    case 'running':
      return {name: 'gitea-running', colorClass: 'tw-text-yellow'};
    case 'cancelling':
      return {name: 'octicon-stop', colorClass: 'tw-text-yellow'};
    case 'failure':
    case 'unknown':
      return {name: circleFill ? 'octicon-x-circle-fill' : 'octicon-x', colorClass: 'tw-text-red'};
    default: {
      const _exhaustive: never = status;
      return _exhaustive;
    }
  }
}
