import {updateWorkflowBadgeFields} from './repo-actions.ts';

test('updateWorkflowBadgeFields updates badge snippets for selected branch', () => {
  document.body.innerHTML = `
    <div
      data-badge-url="https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=main"
      data-workflow-url="https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml"
      data-workflow-display-name="CI [prod]\\build &quot;fast&quot; &lt;ok&gt;"
    >
      <img data-workflow-badge-image src="">
      <input id="workflow-badge-url" readonly>
      <textarea id="workflow-badge-markdown" readonly></textarea>
      <textarea id="workflow-badge-html" readonly></textarea>
    </div>
  `;

  const form = document.querySelector<HTMLElement>('[data-badge-url]')!;

  updateWorkflowBadgeFields(form, 'release/1.0 & hotfix');

  const badgeURL = 'https://gitea.example.com/user1/repo1/actions/workflows/build/test%20workflow.yml/badge.svg?branch=release%2F1.0+%26+hotfix';
  expect(form.querySelector<HTMLImageElement>('[data-workflow-badge-image]')!.src).toBe(badgeURL);
  expect(form.querySelector<HTMLInputElement>('#workflow-badge-url')!.value).toBe(badgeURL);
  expect(form.querySelector<HTMLTextAreaElement>('#workflow-badge-markdown')!.value).toBe(
    `[![CI \\[prod\\]\\\\build "fast" <ok>](${badgeURL})](https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml)`,
  );
  expect(form.querySelector<HTMLTextAreaElement>('#workflow-badge-html')!.value).toBe(
    `<a href="https://gitea.example.com/user1/repo1/actions?workflow=build%2Ftest+workflow.yml"><img src="${badgeURL}" alt="CI [prod]\\build &quot;fast&quot; &lt;ok&gt;"></a>`,
  );
});
