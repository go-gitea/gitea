const {lang} = document.documentElement;

// given a month (0-11), returns it in the documents language
const formatMonth = (month) => new Date(Date.UTC(2022, month, 12)).toLocaleString(lang, {month: 'short'});

// given a weekday (0-6, Sunday to Saturday), returns it in the documents language
const formatDay = (day) => new Date(Date.UTC(2022, 7, day)).toLocaleString(lang, {weekday: 'short'});

const months = new Array(12).fill().map((_, idx) => formatMonth(idx));
const days = new Array(7).fill().map((_, idx) => formatDay(idx));

const heatmapLocale = {months, days};

export default heatmapLocale;
