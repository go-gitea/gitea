// Some timestamps are not meant to be localized in the user's timezone, only in their language.
// This is true for due date timestamps. These include a specific year, month, and day combination. If a user in one timezone
// sets the date YYYY-MM-DD, another user should see the same date, regardless of their timezone. So when a relative-time element
// has their datetime attribute specified only as YYYY-MM-DD, we will update it to YYYY-MM-DD midnight in the user's timezone.
// When the date is rendered, the only localization that will happen is the language.

const dateRegex = /^\d{4}-\d{2}-\d{2}$/;

// for all relative-time elements, if their datetime attribute is YYYY-MM-DD, we will update it to YYYY-MM-DD midnight in the user's timezone
export function initMidnightRelativeTime() {
  const relativeTimeElements = document.querySelectorAll('relative-time[datetime]');

  for (const element of relativeTimeElements) {
    const datetimeAttr = element.getAttribute('datetime');

    if (dateRegex.test(datetimeAttr)) {
      const [year, month, day] = datetimeAttr.split('-');
      const userMidnight = new Date(year, month - 1, day);
      element.setAttribute('datetime', userMidnight.toISOString());
    }
  }
}
