export const allowedTypes = [
  'build',
  'chore',
  'ci',
  'docs',
  'enhance',
  'feat',
  'fix',
  'perf',
  'refactor',
  'revert',
  'style',
  'test',
] as const;

export const allowedTypesList = allowedTypes.join(', ');

const titleTypePattern = new RegExp(`^(${allowedTypes.join('|')})(\\([\\w.-]+\\))?(!)?: .+$`);

export type ParsedPrTitle = {
  type: string;
  breaking: boolean;
};

export function parsePrTitle(title: string): ParsedPrTitle | null {
  const match = titleTypePattern.exec(title);
  if (!match) return null;
  return {type: match[1], breaking: Boolean(match[3])};
}

export const commitTypeToLabel: Record<string, string> = {
  feat: 'type/feature',
  enhance: 'type/enhancement',
  fix: 'type/bug',
  docs: 'type/docs',
  test: 'type/testing',
  chore: 'skip-changelog',
  ci: 'skip-changelog',
  build: 'topic/build',
};

export const managedTypeLabels: string[] = Object.values(commitTypeToLabel);

export const breakingLabel = 'pr/breaking';

export const managedLabels = [...managedTypeLabels, breakingLabel];

export function labelsForPrTitle(title: string): string[] {
  const parsed = parsePrTitle(title);
  if (!parsed) return [];

  const labels: string[] = [];
  const typeLabel = commitTypeToLabel[parsed.type];
  if (typeLabel) labels.push(typeLabel);
  if (parsed.breaking) labels.push(breakingLabel);
  return labels;
}
