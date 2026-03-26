// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {env} from 'node:process';
import {test, expect} from '@playwright/test';
import {login, apiBaseUrl, apiHeaders} from './utils.ts';

test.beforeEach(async ({page}) => {
  await login(page);
});

test.describe('User Settings - Delete Actions with Confirm Dialog', () => {
  test('SSH key delete shows confirm dialog with correct content', async ({page}) => {
    // Generate a unique SSH key for testing
    const keyName = `e2e-test-key-${Date.now()}`;
    const sshKey = 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= e2e-test@example.com';

    // Add SSH key via API
    const response = await page.request.post(`${apiBaseUrl()}/api/v1/user/keys`, {
      headers: apiHeaders(),
      data: {
        title: keyName,
        key: sshKey,
      },
    });
    expect(response.ok()).toBeTruthy();

    // Go to SSH keys settings
    await page.goto('/user/settings/keys');

    // Find the delete button for our test key
    const keyRow = page.locator('.flex-item', {hasText: keyName});
    await expect(keyRow).toBeVisible();

    const deleteButton = keyRow.locator('button.link-action', {hasText: 'Delete'});
    await expect(deleteButton).toBeVisible();

    // Click delete button - should show confirm dialog
    await deleteButton.click();

    // Verify confirm dialog appears
    const confirmModal = page.locator('.ui.g-modal-confirm.modal.visible');
    await expect(confirmModal).toBeVisible();

    // Verify dialog header
    const modalHeader = confirmModal.locator('.header');
    await expect(modalHeader).toContainText('SSH Key Deletion');

    // Verify dialog content
    const modalContent = confirmModal.locator('.content');
    await expect(modalContent).toBeVisible();

    // Verify buttons exist
    const cancelButton = confirmModal.locator('button.cancel');
    const confirmButton = confirmModal.locator('button.ok');
    await expect(cancelButton).toBeVisible();
    await expect(confirmButton).toBeVisible();

    // Confirm button should be red (for dangerous action)
    await expect(confirmButton).toHaveClass(/red/);

    // Cancel the deletion
    await cancelButton.click();
    await expect(confirmModal).toBeHidden();

    // Verify key still exists
    await expect(keyRow).toBeVisible();

    // Now delete for real
    await deleteButton.click();
    await expect(confirmModal).toBeVisible();
    await confirmButton.click();

    // Wait for page reload and verify key is gone
    await expect(keyRow).toBeHidden();
  });

  test('Email delete shows confirm dialog', async ({page}) => {
    // Add a test email via API
    const testEmail = `e2e-test-${Date.now()}@${env.GITEA_TEST_E2E_DOMAIN}`;

    const response = await page.request.post(`${apiBaseUrl()}/api/v1/user/emails`, {
      headers: apiHeaders(),
      data: {
        emails: [testEmail],
      },
    });
    expect(response.ok()).toBeTruthy();

    // Go to account settings
    await page.goto('/user/settings/account');

    // Find the delete button for our test email
    const emailRow = page.locator('.flex-item', {hasText: testEmail});
    await expect(emailRow).toBeVisible();

    const deleteButton = emailRow.locator('button.link-action', {hasText: 'Delete'});
    await expect(deleteButton).toBeVisible();

    // Click delete button
    await deleteButton.click();

    // Verify confirm dialog
    const confirmModal = page.locator('.ui.g-modal-confirm.modal.visible');
    await expect(confirmModal).toBeVisible();

    // Cancel first
    await confirmModal.locator('button.cancel').click();
    await expect(confirmModal).toBeHidden();

    // Verify email still exists
    await expect(emailRow).toBeVisible();

    // Now delete for real
    await deleteButton.click();
    await expect(confirmModal).toBeVisible();
    await confirmModal.locator('button.ok').click();

    // Verify email is gone
    await expect(emailRow).toBeHidden();
  });

  test('Access token delete shows confirm dialog', async ({page}) => {
    const tokenName = `e2e-test-token-${Date.now()}`;

    // Create access token via API
    const response = await page.request.post(`${apiBaseUrl()}/api/v1/user/applications/oauth2`, {
      headers: apiHeaders(),
      data: {
        name: tokenName,
        confidential_client: true,
      },
    });
    expect(response.ok()).toBeTruthy();

    // Go to applications settings
    await page.goto('/user/settings/applications');

    // Find the OAuth2 application we created
    const appRow = page.locator('.flex-item', {hasText: tokenName});
    await expect(appRow).toBeVisible();

    const deleteButton = appRow.locator('button.link-action', {hasText: 'Delete'});
    await expect(deleteButton).toBeVisible();

    // Click delete and verify confirm dialog
    await deleteButton.click();

    const confirmModal = page.locator('.ui.g-modal-confirm.modal.visible');
    await expect(confirmModal).toBeVisible();

    // Cancel first
    await confirmModal.locator('button.cancel').click();
    await expect(confirmModal).toBeHidden();

    // Delete for real
    await deleteButton.click();
    await expect(confirmModal).toBeVisible();
    await confirmModal.locator('button.ok').click();

    // Verify app is gone
    await expect(appRow).toBeHidden();
  });
});

test.describe('Confirm Modal Accessibility', () => {
  test('confirm modal has correct ARIA attributes', async ({page}) => {
    // Create an SSH key first to ensure we have a delete button to test
    const keyName = `e2e-accessibility-test-${Date.now()}`;
    const sshKey = 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= e2e-test@example.com';

    const response = await page.request.post(`${apiBaseUrl()}/api/v1/user/keys`, {
      headers: apiHeaders(),
      data: {title: keyName, key: sshKey},
    });
    expect(response.ok()).toBeTruthy();

    // Navigate to the SSH keys page
    await page.goto('/user/settings/keys');

    // Find the delete button for our test key
    const keyRow = page.locator('.flex-item', {hasText: keyName});
    await expect(keyRow).toBeVisible();

    const deleteButton = keyRow.locator('button.link-action', {hasText: 'Delete'});
    await expect(deleteButton).toBeVisible();

    // Click delete to show confirm modal
    await deleteButton.click();

    const confirmModal = page.locator('.ui.g-modal-confirm.modal.visible');
    await expect(confirmModal).toBeVisible();

    // Check modal has proper role
    await expect(confirmModal).toHaveAttribute('role', 'dialog');

    // Check modal has aria-modal
    await expect(confirmModal).toHaveAttribute('aria-modal', 'true');

    // Check that focus is on the confirm button
    const confirmButton = confirmModal.locator('button.ok');
    await expect(confirmButton).toBeFocused();

    // Close with Escape
    await page.keyboard.press('Escape');
    await expect(confirmModal).toBeHidden();

    // Clean up - delete the key via API
    const keyId = (await response.json()).id;
    await page.request.delete(`${apiBaseUrl()}/api/v1/user/keys/${keyId}`, {
      headers: apiHeaders(),
    });
  });

  test('confirm modal keyboard navigation', async ({page}) => {
    // Create an SSH key first
    const keyName = `e2e-keyboard-test-${Date.now()}`;
    const sshKey = 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= e2e-test@example.com';

    const response = await page.request.post(`${apiBaseUrl()}/api/v1/user/keys`, {
      headers: apiHeaders(),
      data: {title: keyName, key: sshKey},
    });
    expect(response.ok()).toBeTruthy();

    await page.goto('/user/settings/keys');

    const keyRow = page.locator('.flex-item', {hasText: keyName});
    await expect(keyRow).toBeVisible();

    const deleteButton = keyRow.locator('button.link-action', {hasText: 'Delete'});

    // Focus the delete button
    await deleteButton.focus();
    await expect(deleteButton).toBeFocused();

    // Press Enter to activate
    await page.keyboard.press('Enter');

    const confirmModal = page.locator('.ui.g-modal-confirm.modal.visible');
    await expect(confirmModal).toBeVisible();

    // Tab should cycle within modal
    await page.keyboard.press('Tab');
    const cancelButton = confirmModal.locator('button.cancel');
    await expect(cancelButton).toBeFocused();

    // Tab again to go back to confirm button
    await page.keyboard.press('Tab');
    const confirmButton = confirmModal.locator('button.ok');
    await expect(confirmButton).toBeFocused();

    // Press Escape to close without deleting
    await page.keyboard.press('Escape');
    await expect(confirmModal).toBeHidden();

    // Clean up
    const keyId = (await response.json()).id;
    await page.request.delete(`${apiBaseUrl()}/api/v1/user/keys/${keyId}`, {
      headers: apiHeaders(),
    });
  });
});
