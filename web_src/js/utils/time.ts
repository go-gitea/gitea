import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc.js';
import {getCurrentLocale} from '../utils.ts';
import type {ConfigType} from 'dayjs';

dayjs.extend(utc);

/**
 * Returns an array of millisecond-timestamps of start-of-week days (Sundays)
 *
 * @param startDate The start date. Can take any type that dayjs accepts.
 * @param endDate The end date. Can take any type that dayjs accepts.
 */
export function startDaysBetween(startDate: ConfigType, endDate: ConfigType): number[] {
  const start = dayjs.utc(startDate);
  const end = dayjs.utc(endDate);

  let current = start;

  // Ensure the start date is a Sunday
  while (current.day() !== 0) {
    current = current.add(1, 'day');
  }

  const startDays: number[] = [];
  while (current.isBefore(end)) {
    startDays.push(current.valueOf());
    current = current.add(1, 'week');
  }

  return startDays;
}

export function firstStartDateAfterDate(inputDate: Date): number {
  if (!(inputDate instanceof Date)) {
    throw new Error('Invalid date');
  }
  const dayOfWeek = inputDate.getUTCDay();
  const daysUntilSunday = 7 - dayOfWeek;
  const resultDate = new Date(inputDate.getTime());
  resultDate.setUTCDate(resultDate.getUTCDate() + daysUntilSunday);
  return resultDate.valueOf();
}

export type DayData = {
  week: number,
  additions: number,
  deletions: number,
  commits: number,
}

export type DayDataObject = {
  [timestamp: string]: DayData,
}

export function fillEmptyStartDaysWithZeroes(startDays: number[], data: DayDataObject): DayData[] {
  const result = {};

  for (const startDay of startDays) {
    result[startDay] = data[startDay] || {'week': startDay, 'additions': 0, 'deletions': 0, 'commits': 0};
  }

  return Object.values(result);
}

let dateFormat: Intl.DateTimeFormat;

// format a Date object to document's locale, but with 24h format from user's current locale because this
// option is a personal preference of the user, not something that the document's locale should dictate.
export function formatDatetime(date: Date | number): string {
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
