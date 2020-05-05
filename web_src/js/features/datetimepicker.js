export default async function initDateTimePicker(locale) {
  await Promise.all([
    import(/* webpackChunkName: "datetimepicker" */'jquery-datetimepicker'),
    import(/* webpackChunkName: "datetimepicker" */'jquery-datetimepicker/build/jquery.datetimepicker.min.css'),
  ]);

  $.datetimepicker.setLocale(locale);
}
