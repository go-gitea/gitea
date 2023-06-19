export function formatTrackedTime(duration) {
  let formattedTime = '';

  const hours = Math.floor(duration / 3600);
  const minutes = Math.floor((duration / 60) % 60);
  const seconds = duration % 60;

  if (hours > 0) {
    formattedTime = formatTime(hours, 'hour', formattedTime);
    formattedTime = formatTime(minutes, 'minute', formattedTime);
  } else {
    formattedTime = formatTime(minutes, 'minute', formattedTime);
    formattedTime = formatTime(seconds, 'second', formattedTime);
  }

  formattedTime = formattedTime.trimEnd();
  return formattedTime;
}

function formatTime(value, name, formattedTime) {
  if (value === 1) {
    formattedTime = `${formattedTime}1 ${name} `;
  } else if (value > 1) {
    formattedTime = `${formattedTime}${value} ${name}s `;
  }
  return formattedTime;
}
