import type {ActionsArtifact} from '../modules/gitea-actions.ts';

export type ArtifactTooltipLocale = {
  artifactExpired: string;
  artifactExpiresAt: string;
  artifactSize: string;
  status: {
    unknown: string;
  };
};

const sizeUnits = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB'];
let artifactDateFormat: Intl.DateTimeFormat;

export function formatArtifactSize(size: number): string {
  let value = size;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < sizeUnits.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  const formattedValue = unitIndex === 0 ? String(Math.round(value)) : value.toFixed(value >= 10 ? 0 : 1);
  return `${formattedValue} ${sizeUnits[unitIndex]}`;
}

function getArtifactDateFormat(): Intl.DateTimeFormat {
  return artifactDateFormat ??= new Intl.DateTimeFormat(document.documentElement.lang || undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: 'numeric',
    hourCycle: new Intl.DateTimeFormat(undefined, {hour: 'numeric'}).resolvedOptions().hourCycle,
  });
}

export function formatArtifactTimestamp(expiresUnix: number): string | null {
  if (expiresUnix === null || !Number.isFinite(expiresUnix) || expiresUnix <= 0) return null;
  return getArtifactDateFormat().format(new Date(expiresUnix * 1000));
}

export function createArtifactTooltipContent(artifact: ActionsArtifact, locale: ArtifactTooltipLocale): string {
  const details = [];

  if (artifact.status === 'expired') {
    details.push(locale.artifactExpired);
  } else {
    details.push(locale.artifactExpiresAt.replace('%s', formatArtifactTimestamp(artifact.expiresUnix) ?? locale.status.unknown));
  }
  details.push(`${locale.artifactSize}: ${formatArtifactSize(artifact.size)}`);

  return details.join(' | ');
}
