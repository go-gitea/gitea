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
type CommitType = typeof allowedTypes[number];

export const allowedTypesList = allowedTypes.join(', ');

const titlePattern = new RegExp(`^(${allowedTypes.join('|')})(\\([\\w.-]+\\))?(!)?: .+$`);

export function parsePrTitle(title: string): {type: CommitType; breaking: boolean} | null {
  const match = titlePattern.exec(title);
  return match ? {type: match[1] as CommitType, breaking: Boolean(match[3])} : null;
}

export const breakingLabel = 'pr/breaking';

// Mutually exclusive type labels, fully synced with the title type (added and removed).
const typeLabels: Partial<Record<CommitType, string>> = {
  feat: 'type/feature',
  enhance: 'type/enhancement',
  fix: 'type/bug',
  docs: 'type/docs',
  test: 'type/testing',
};

// Non-type labels, only added, never auto-removed, so manual labeling is not clobbered.
const extraLabels: Partial<Record<CommitType, string>> = {
  chore: 'skip-changelog',
  ci: 'skip-changelog',
  build: 'topic/build',
};

// Labels this tool may remove when the title no longer implies them.
export const removableLabels = [...Object.values(typeLabels), breakingLabel];

export function labelsForPrTitle(title: string): string[] {
  const parsed = parsePrTitle(title);
  if (!parsed) return [];
  return [typeLabels[parsed.type], extraLabels[parsed.type], parsed.breaking ? breakingLabel : undefined]
    .filter((label): label is string => label !== undefined);
}
