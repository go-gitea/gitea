import type {ActionsArtifact} from '../modules/gitea-actions.ts';
import {formatDatetime} from '../utils/time.ts';

export type ArtifactTooltipLocale = {
  artifactExpired: string;
  artifactExpiresAt: string;
  artifactSize: string;
  status: {
    unknown: string;
  };
};

const sizeUnits = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB'];

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

export function formatArtifactTimestamp(expiresUnix: number): string | null {
  if (expiresUnix === null || !Number.isFinite(expiresUnix) || expiresUnix <= 0) return null;
  return formatDatetime(new Date(expiresUnix * 1000));
}

export function createArtifactTooltipContent(artifact: ActionsArtifact, locale: ArtifactTooltipLocale): string {
  const details = [];

  if (artifact.status === 'expired') {
    details.push(locale.artifactExpired);
  } else {
    details.push(`${locale.artifactExpiresAt}: ${formatArtifactTimestamp(artifact.expiresUnix) ?? locale.status.unknown}`);
  }
  details.push(`${locale.artifactSize}: ${formatArtifactSize(artifact.size)}`);

  return details.join(' | ');
}
