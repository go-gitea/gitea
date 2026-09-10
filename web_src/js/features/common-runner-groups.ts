import {registerGlobalInitFunc} from '../modules/observer.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';

const {appSubUrl} = window.config;

type RepoSearchResponse = {data: Array<{repository: {id: number; full_name: string}}>};

export function initRunnerGroupInputs(): void {
  registerGlobalInitFunc('initRunnerAccessInput', (el: HTMLElement) => {
    const uid = el.getAttribute('data-uid')!;
    fomanticQuery(el).dropdown({
      preserveHTML: false,
      labelHref: (_value: string, text: string) => `${appSubUrl}/${text}`,
      apiSettings: {
        url: `${appSubUrl}/repo/search?q={query}&uid=${uid}&exclusive=${uid !== '0'}`,
        throttle: 500,
        onResponse: (res: RepoSearchResponse) => ({success: true, results: res.data.map((item) => ({value: String(item.repository.id), name: item.repository.full_name}))}),
      },
    });
  });

  registerGlobalInitFunc('initRunnerGroupMembersInput', (el: HTMLElement) => {
    const runnerLink = el.getAttribute('data-runner-link')!;
    fomanticQuery(el).dropdown({
      preserveHTML: false,
      labelHref: (value: string) => `${runnerLink}/${value}`,
    });
  });
}
