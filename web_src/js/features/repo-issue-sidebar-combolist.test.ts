import {syncIssueMainContentTimelineItems} from './repo-issue-sidebar-combolist.ts';
import {createElementFromHTML} from '../utils/dom.ts';

describe('syncIssueMainContentTimelineItems', () => {
  test('InsertNew', () => {
    const oldContent = createElementFromHTML(`
    <div>
        <div class="timeline-item">First</div>
        <div class="timeline-item" id="timeline-comments-end"></div>
    </div>
  `);
    const newContent = createElementFromHTML(`
    <div>
        <div class="timeline-item" id="a">New</div>
    </div>
  `);
    syncIssueMainContentTimelineItems(oldContent, newContent);
    expect(oldContent.innerHTML.replace(/>\s+</g, '><').trim()).toBe(
      `<div class="timeline-item">First</div>` +
      `<div class="timeline-item" id="a">New</div>` +
      `<div class="timeline-item" id="timeline-comments-end"></div>`,
    );
  });

  test('Sync', () => {
    const oldContent = createElementFromHTML(`
    <div>
      <div class="timeline-item">First</div>
      <div class="timeline-item" id="it-1">Item 1</div>
      <div class="timeline-item event" id="it-2">Item 2</div>
      <div class="timeline-item" id="it-3">Item 3</div>
      <div class="timeline-item event" id="it-4">Item 4</div>
      <div class="timeline-item" id="timeline-comments-end"></div>
      <div class="timeline-item">Other</div>
    </div>
  `);
    const newContent = createElementFromHTML(`
    <div>
      <div class="timeline-item" id="it-1">New 1</div>
      <div class="timeline-item event" id="it-2">New 2</div>
      <div class="timeline-item" id="it-x">New X</div>
    </div>
  `);
    syncIssueMainContentTimelineItems(oldContent, newContent);

    // Item 1 won't be replaced because it's not an event
    // Item 2 will be replaced with New 2
    // Item 3 will be kept because it's not in new content
    // Item 4 will be removed because it's not in new content, and it's an event
    // New X will be inserted at the end of timeline items (before timeline-comments-end)
    expect(oldContent.innerHTML.replace(/>\s+</g, '><').trim()).toBe(
      `<div class="timeline-item">First</div>` +
      `<div class="timeline-item" id="it-1">Item 1</div>` +
      `<div class="timeline-item event" id="it-2">New 2</div>` +
      `<div class="timeline-item" id="it-3">Item 3</div>` +
      `<div class="timeline-item" id="it-x">New X</div>` +
      `<div class="timeline-item" id="timeline-comments-end"></div>` +
      `<div class="timeline-item">Other</div>`,
    );
  });
});
