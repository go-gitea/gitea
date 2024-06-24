import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc.js';
import {getCurrentLocale} from '../utils.js';

dayjs.extend(utc);

/**
 * Returns an array of millisecond-timestamps of start-of-week days (Sundays)
 *
 * @param startConfig The start date. Can take any type that `Date` accepts.
 * @param endConfig The end date. Can take any type that `Date` accepts.
 */
export function startDaysBetween(startDate, endDate) {
  const start = dayjs.utc(startDate);
  const end = dayjs.utc(endDate);

  let current = start;

  // Ensure the start date is a Sunday
  while (current.day() !== 0) {
    current = current.add(1, 'day');
  }

  const startDays = [];
  while (current.isBefore(end)) {
    startDays.push(current.valueOf());
    current = current.add(1, 'week');
  }

  return startDays;
}

export function firstStartDateAfterDate(inputDate) {
  if (!(inputDate instanceof Date)) {
    throw new Error('Invalid date');
  }
  const dayOfWeek = inputDate.getUTCDay();
  const daysUntilSunday = 7 - dayOfWeek;
  const resultDate = new Date(inputDate.getTime());
  resultDate.setUTCDate(resultDate.getUTCDate() + daysUntilSunday);
  return resultDate.valueOf();
}

export function fillEmptyStartDaysWithZeroes(startDays, data) {
  const result = {};

  for (const startDay of startDays) {
    result[startDay] = data[startDay] || {'week': startDay, 'additions': 0, 'deletions': 0, 'commits': 0};
  }

  return Object.values(result);
}

let dateFormat;

// format a Date object to document's locale, but with 24h format from user's current locale because this
// option is a personal preference of the user, not something that the document's locale should dictate.
export function formatDatetime(date) {
  if (!dateFormat) {
    // TODO: replace `hour12` with `Intl.Locale.prototype.getHourCycles` once there is broad browser support
    dateFormat = new Intl.DateTimeFormat(getCurrentLocale(), {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
      hour: 'numeric',
      hour12: !Number.isInteger(Number(new Intl.DateTimeFormat([], {hour: 'numeric'}).format())),
      minute: '2-digit',
      timeZoneName: 'short',
    });
  }
  return dateFormat.format(date);
}
