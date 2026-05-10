import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiCreateRepo, apiCreateIssue, apiDeleteRepo, createProject, createProjectColumn, randomString} from './utils.ts';

test('assign issue to project and change column', async ({page}) => {
  const repoName = `e2e-issue-project-${randomString(8)}`;
  const user = env.GITEA_TEST_E2E_USER;
  await Promise.all([login(page), apiCreateRepo(page.request, {name: repoName})]);
  await page.goto(`/${user}/${repoName}/projects/new`);
  await page.locator('input[name="title"]').fill('Kanban Board');
  await page.getByRole('button', {name: 'Create Project'}).click();
  const projectLink = page.locator('.milestone-list a', {hasText: 'Kanban Board'}).first();
  await expect(projectLink).toBeVisible();
  const href = await projectLink.getAttribute('href');
  const projectID = href!.split('/').pop()!;
  // columns created via POST because the web UI uses modals that are hard to drive
  await Promise.all([
    ...['Backlog', 'In Progress', 'Done'].map((title) => createProjectColumn(page.request, user, repoName, projectID, title)),
    apiCreateIssue(page.request, {owner: user, repo: repoName, title: 'Column picker test'}),
  ]);
  await page.goto(`/${user}/${repoName}/issues/1`);
  await page.locator('.sidebar-project-combo > .ui.dropdown').click();
  await page.locator('.sidebar-project-combo > .ui.dropdown .item:has-text("Kanban Board")').click();
  await page.locator('.sidebar-project-combo > .ui.dropdown').click();
  await page.locator('.sidebar-project-column-combo .ui.dropdown').click();
  await page.locator('.sidebar-project-column-combo .ui.dropdown .item:has-text("In Progress")').click();
  await expect(page.locator('.sidebar-project-column-combo .ui.dropdown .fixed-text')).toHaveText('In Progress');
  await apiDeleteRepo(page.request, user, repoName);
});

test('create a project', async ({page}) => {
  const repoName = `e2e-project-repo-${Date.now()}`;
  const projectTitle = 'Test Project';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Navigate to new project page
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/projects/new`);

    // Fill in project details
    await page.getByLabel('Title').fill(projectTitle);

    // Submit the form
    await page.getByRole('button', {name: 'Create Project'}).click();

    // Verify project was created by checking we're redirected to the projects list
    await expect(page).toHaveURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}/projects$`));

    // Verify the project appears in the list
    await expect(page.locator('.milestone-list')).toContainText(projectTitle);
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('assign issue to multiple projects via sidebar', async ({page}) => {
  const repoName = `e2e-multi-project-${Date.now()}`;
  const project1Title = 'Project Alpha';
  const project2Title = 'Project Beta';
  const issueTitle = 'Test issue for multiple projects';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects via UI
    const project1 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project1Title,
    });
    const project2 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project2Title,
    });

    // Create an issue without any project
    const issue = await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: issueTitle,
    });

    // Navigate to the issue page
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/${issue.index}`);

    // Open the projects dropdown in the sidebar
    await page.locator('.sidebar-project-combo > .ui.dropdown').click();

    // Select both projects
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project1.id}"]`).click();
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project2.id}"]`).click();

    // Click outside to close the dropdown and trigger the update
    await page.locator('.issue-content-left').click();

    // Verify both projects are shown in the sidebar
    await expect(page.locator(`.item.sidebar-project-card:has-text("${project1Title}")`)).toBeVisible();
    await expect(page.locator(`.item.sidebar-project-card:has-text("${project2Title}")`)).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('create issue with multiple projects pre-selected', async ({page}) => {
  const repoName = `e2e-issue-multi-proj-${Date.now()}`;
  const project1Title = 'Project One';
  const project2Title = 'Project Two';
  const issueTitle = 'Issue with multiple projects';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects via UI
    const project1 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project1Title,
    });
    const project2 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project2Title,
    });

    // Navigate to new issue page
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);

    // Fill in the issue title
    await page.locator('input[name="title"]').fill(issueTitle);

    // Open the projects dropdown
    await page.locator('.sidebar-project-combo > .ui.dropdown').click();

    // Select both projects
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project1.id}"]`).click();
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project2.id}"]`).click();

    // Click outside to close the dropdown
    await page.locator('.issue-content-left').click();

    // Submit the form
    await page.getByRole('button', {name: 'Create Issue'}).click();

    // Wait for issue to be created and page to redirect
    await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/\\d+`));

    // Verify both projects are shown in the sidebar
    await expect(page.locator(`.item.sidebar-project-card:has-text("${project1Title}")`)).toBeVisible();
    await expect(page.locator(`.item.sidebar-project-card:has-text("${project2Title}")`)).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('filter issues by multiple projects in issue list', async ({page}) => {
  const repoName = `e2e-filter-projects-${Date.now()}`;
  const project1Title = 'Filter Project A';
  const project2Title = 'Filter Project B';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects via UI
    const project1 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project1Title,
    });
    const project2 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project2Title,
    });

    // Create issues: one in project1, one in project2, one in both
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue in Project A only',
      projects: [project1.id],
    });
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue in Project B only',
      projects: [project2.id],
    });
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue in both projects',
      projects: [project1.id, project2.id],
    });
    // Create an issue with no project
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue with no project',
    });

    // Verify only project1 issues are visible
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues?project=${project1.id}`);
    await expect(page.locator('#issue-list')).toContainText('Issue in Project A only');
    await expect(page.locator('#issue-list')).toContainText('Issue in both projects');
    await expect(page.locator('#issue-list')).not.toContainText('Issue in Project B only');
    await expect(page.locator('#issue-list')).not.toContainText('Issue with no project');

    // Verify only project2 issues are visible
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues?project=${project2.id}`);
    await expect(page.locator('#issue-list')).toContainText('Issue in Project B only');
    await expect(page.locator('#issue-list')).toContainText('Issue in both projects');
    await expect(page.locator('#issue-list')).not.toContainText('Issue in Project A only');
    await expect(page.locator('#issue-list')).not.toContainText('Issue with no project');
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('remove issue from one project keeping others', async ({page}) => {
  const repoName = `e2e-remove-project-${Date.now()}`;
  const project1Title = 'Keep This Project';
  const project2Title = 'Remove This Project';
  const issueTitle = 'Issue to modify projects';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects via UI
    const project1 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project1Title,
    });
    const project2 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project2Title,
    });

    // Create an issue in both projects
    const issue = await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: issueTitle,
      projects: [project1.id, project2.id],
    });

    // Navigate to the issue page
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/${issue.index}`);

    // Verify both projects are initially shown
    await expect(page.locator(`.item.sidebar-project-card:has-text("${project1Title}")`)).toBeVisible();
    await expect(page.locator(`.item.sidebar-project-card.item:has-text("${project2Title}")`)).toBeVisible();

    // Open the projects dropdown
    await page.locator('.sidebar-project-combo > .ui.dropdown').click();

    // Deselect project2 (click on the already selected item to deselect)
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project2.id}"]`).click();

    // Click outside to close the dropdown and trigger the update
    await page.locator('.issue-content-left').click();

    // Verify project1 is still shown but project2 is removed
    await expect(page.locator(`.item.sidebar-project-card.item:has-text("${project1Title}")`)).toBeVisible();
    await expect(page.locator(`.item.sidebar-project-card.item:has-text("${project2Title}")`)).toBeHidden();

    // Reload the page to see the timeline comment
    await page.reload();

    // Verify the timeline shows "removed this from the project" comment
    const timelineComments = page.locator('.timeline-item.event');
    await expect(timelineComments.filter({hasText: 'removed this from the'})).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('filter issues with no project using project=-1', async ({page}) => {
  const repoName = `e2e-no-project-filter-${Date.now()}`;
  const projectTitle = 'Some Project';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create a project via UI
    const project = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: projectTitle,
    });

    // Create an issue with a project
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue with project assigned',
      projects: [project.id],
    });

    // Create issues with no project
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Issue without any project',
    });
    await apiCreateIssue(page.request, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: 'Another unassigned issue',
    });

    // First verify we can see all issues without the filter
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues?type=all&state=open`);
    await expect(page.locator('#issue-list')).toContainText('Issue with project assigned');
    await expect(page.locator('#issue-list')).toContainText('Issue without any project');
    await expect(page.locator('#issue-list')).toContainText('Another unassigned issue');

    // Navigate to issue list filtering for issues with no project (project=-1)
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues?type=all&state=open&project=-1`);

    // Verify only issues with no project are visible
    await expect(page.locator('#issue-list')).toContainText('Issue without any project');
    await expect(page.locator('#issue-list')).toContainText('Another unassigned issue');

    // Verify the issue with a project is NOT visible
    await expect(page.locator('#issue-list')).not.toContainText('Issue with project assigned');

    // Verify the last item in the list is NOT the issue with a project
    const issueItems = page.locator('#issue-list .item');
    const lastIssueItem = issueItems.last();
    await expect(lastIssueItem).not.toContainText('Issue with project assigned');
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('close project and view in closed projects list', async ({page}) => {
  const repoName = `e2e-close-project-${Date.now()}`;
  const openProjectTitle = 'Open Project';
  const closedProjectTitle = 'Project To Close';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects via UI
    await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: openProjectTitle,
    });
    const projectToClose = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: closedProjectTitle,
    });

    // Navigate to projects list
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/projects`);

    // Verify both projects are visible in open state
    await expect(page.locator('.milestone-list')).toContainText(openProjectTitle);
    await expect(page.locator('.milestone-list')).toContainText(closedProjectTitle);

    // Close the second project by clicking the close link
    const projectCard = page.locator('.milestone-card').filter({hasText: closedProjectTitle});
    await projectCard.locator('a.link-action[data-url$="/close"]').click();

    // Wait for redirect back to project view page
    await page.waitForURL(new RegExp(`/${env.GITEA_TEST_E2E_USER}/${repoName}/projects/${projectToClose.id}`));

    // Navigate to projects list
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/projects`);

    // Click on "Closed" tab to view closed projects
    await page.locator('.list-header-toggle a.item').filter({hasText: 'Closed'}).click();

    // Wait for the page to load with closed projects
    await page.waitForURL(/state=closed/);

    // Verify only the closed project is visible
    await expect(page.locator('.milestone-list')).toContainText(closedProjectTitle);
    await expect(page.locator('.milestone-list')).not.toContainText(openProjectTitle);

    // Verify the "Closed" tab is active
    await expect(page.locator('.list-header-toggle a.item.active')).toContainText('Closed');
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});

test('select projects on new issue page shows in sidebar', async ({page}) => {
  const repoName = `e2e-new-issue-project-${Date.now()}`;
  const project1Title = 'Project One';
  const project2Title = 'Project Two';

  await login(page);
  await apiCreateRepo(page.request, {name: repoName});

  try {
    // Create two projects
    const project1 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project1Title,
    });
    const project2 = await createProject(page, {
      owner: env.GITEA_TEST_E2E_USER,
      repo: repoName,
      title: project2Title,
    });

    // Navigate to new issue page
    await page.goto(`/${env.GITEA_TEST_E2E_USER}/${repoName}/issues/new`);

    // Open the projects dropdown in the sidebar
    await page.locator('.sidebar-project-combo > .ui.dropdown').click();

    // Select both projects
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project1.id}"]`).click();
    await page.locator(`.sidebar-project-combo > .ui.dropdown .item[data-value="${project2.id}"]`).click();

    // Click outside to close dropdown
    await page.locator('.issue-content-left').click();

    // Verify both projects appear in the sidebar list below the dropdown
    // On new issue page, these are simple cloned items rendered in the list container
    const projectList = page.locator('.sidebar-project-combo > .ui.list');
    await expect(projectList.locator(`.item:has-text("${project1Title}")`).first()).toBeVisible();
    await expect(projectList.locator(`.item:has-text("${project2Title}")`).first()).toBeVisible();
  } finally {
    await apiDeleteRepo(page.request, env.GITEA_TEST_E2E_USER, repoName);
  }
});
