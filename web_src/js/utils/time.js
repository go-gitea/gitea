import dayjs from 'dayjs';
import {getCurrentLocale} from '../utils.js';

// Returns an array of millisecond-timestamps of start-of-week days (Sundays)
export function startDaysBetween(startDate, endDate) {
  // Ensure the start date is a Sunday
  while (startDate.getDay() !== 0) {
    startDate.setDate(startDate.getDate() + 1);
  }

  const start = dayjs(startDate);
  const end = dayjs(endDate);
  const startDays = [];

  let current = start;
  while (current.isBefore(end)) {
    startDays.push(current.valueOf());
    // we are adding 7 * 24 hours instead of 1 week because we don't want
    // date library to use local time zone to calculate 1 week from now.
    // local time zone is problematic because of daylight saving time (dst)
    // used on some countries
    current = current.add(7 * 24, 'hour');
  }

  return startDays;
}

export function firstStartDateAfterDate(inputDate) {
  if (!(inputDate instanceof Date)) {
    throw new Error('Invalid date');
  }
  const dayOfWeek = inputDate.getDay();
  const daysUntilSunday = 7 - dayOfWeek;
  const resultDate = new Date(inputDate.getTime());
  resultDate.setDate(resultDate.getDate() + daysUntilSunday);
  return resultDate.valueOf();
}

export function fillEmptyStartDaysWithZeroes(startDays, data) {
  const result = {};

  for (const startDay of startDays) {
    result[startDay] = data[startDay] || {'week': startDay, 'additions': 0, 'deletions': 0, 'commits': 0};
  }

  return Object.values(result);
}

// TODO: replace with Intl.Locale.prototype.getHourCycles once there is broad browser support
const use24h = Number.isInteger(Number(new Intl.DateTimeFormat([], {hour: 'numeric'}).format()));

// format a Date object to document's locale, but with 24h format from user's current locale
export function formatDateTime(date) {
  return new Intl.DateTimeFormat(getCurrentLocale(), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: 'numeric',
    hour12: !use24h,
    minute: '2-digit',
    timeZoneName: 'short',
  }).format(date);
}
