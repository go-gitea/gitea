import dayjs from 'dayjs';

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
    // do not change the following line unless you are sure what you are doing
    // see https://github.com/go-gitea/gitea/pull/27882#issuecomment-1945594269
    // for details
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
