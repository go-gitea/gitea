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
};

export type DayDataObject = {
  [timestamp: string]: DayData,
};

export function fillEmptyStartDaysWithZeroes(startDays: number[], data: DayDataObject): DayData[] {
  const result: Record<string, any> = {};

  for (const startDay of startDays) {
    result[startDay] = data[startDay] || {'week': startDay, 'additions': 0, 'deletions': 0, 'commits': 0};
  }

  return Object.values(result);
}

let dateFormat: Intl.DateTimeFormat;

// ISO 8601 UTC with 7-digit fractional seconds, matching Go's `2006-01-02T15:04:05.0000000Z07:00`
export function formatDatetimeISO(unixSeconds: number): string {
  const base = new Date(unixSeconds * 1000).toISOString().slice(0, 19);
  const frac = unixSeconds - Math.floor(unixSeconds);
  const fracInt = Math.floor(frac * 10_000_000);
  return `${base}.${String(fracInt).padStart(7, '0')}Z`;
}

/** Format a Date to a localized format, for example "21 May 2026, 14:30:45". */
export function formatDatetime(date: Date | number): string {
  if (!dateFormat) {
    dateFormat = new Intl.DateTimeFormat(getCurrentLocale(), {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      hourCycle: new Intl.DateTimeFormat([], {hour: 'numeric'}).resolvedOptions().hourCycle === 'h12' ? 'h12' : 'h23',
      minute: '2-digit',
      second: '2-digit',
    });
  }
  return dateFormat.format(date);
}
