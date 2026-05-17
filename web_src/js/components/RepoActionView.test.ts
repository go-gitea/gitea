import {aggregateIterationStatus, groupJobsByMatrix} from './ActionRunView.ts';
import type {ActionsJob, ActionsStatus} from '../modules/gitea-actions.ts';

function makeJob(id: number, jobId: string, name = jobId, status: ActionsStatus = 'success'): ActionsJob {
  return {
    id,
    link: `/run/job/${id}`,
    jobId,
    name,
    status,
    canRerun: false,
    duration: '1s',
  };
}

describe('groupJobsByMatrix', () => {
  test('empty list', () => {
    expect(groupJobsByMatrix([])).toEqual([]);
  });

  test('single job becomes a single-iteration group', () => {
    const out = groupJobsByMatrix([makeJob(1, 'build')]);
    expect(out).toHaveLength(1);
    expect(out[0].jobId).toBe('build');
    expect(out[0].iterations).toHaveLength(1);
  });

  test('contiguous same-jobId entries form one group', () => {
    const out = groupJobsByMatrix([
      makeJob(1, 'build', 'build (linux)'),
      makeJob(2, 'build', 'build (windows)'),
      makeJob(3, 'build', 'build (macos)'),
    ]);
    expect(out).toHaveLength(1);
    expect(out[0].iterations.map((j) => j.id)).toEqual([1, 2, 3]);
  });

  test('distinct jobIds become separate groups', () => {
    const out = groupJobsByMatrix([
      makeJob(1, 'lint'),
      makeJob(2, 'test'),
      makeJob(3, 'deploy'),
    ]);
    expect(out).toHaveLength(3);
    expect(out.map((g) => g.jobId)).toEqual(['lint', 'test', 'deploy']);
    expect(out.every((g) => g.iterations.length === 1)).toBe(true);
  });

  test('mixed singleton + matrix is grouped correctly', () => {
    const out = groupJobsByMatrix([
      makeJob(1, 'job1'),
      makeJob(2, 'job2', 'job2 (afile)'),
      makeJob(3, 'job2', 'job2 (another file)'),
      makeJob(4, 'job3'),
    ]);
    expect(out.map((g) => ({id: g.jobId, n: g.iterations.length}))).toEqual([
      {id: 'job1', n: 1},
      {id: 'job2', n: 2},
      {id: 'job3', n: 1},
    ]);
  });

  test('aggregateStatus reflects worst pending iteration', () => {
    const out = groupJobsByMatrix([
      makeJob(1, 'test', 'test (a)', 'success'),
      makeJob(2, 'test', 'test (b)', 'running'),
      makeJob(3, 'test', 'test (c)', 'waiting'),
    ]);
    expect(out[0].aggregateStatus).toBe('running');
  });

  test('aggregateStatus rolls up to success when all pass', () => {
    const out = groupJobsByMatrix([
      makeJob(1, 'test', 'test (a)', 'success'),
      makeJob(2, 'test', 'test (b)', 'success'),
    ]);
    expect(out[0].aggregateStatus).toBe('success');
  });
});

describe('aggregateIterationStatus', () => {
  test('empty -> unknown', () => {
    expect(aggregateIterationStatus([])).toBe('unknown');
  });

  test('all skipped collapses to skipped (not success)', () => {
    expect(aggregateIterationStatus(['skipped', 'skipped'])).toBe('skipped');
  });

  test('success + skipped -> success', () => {
    expect(aggregateIterationStatus(['success', 'skipped'])).toBe('success');
  });

  test('cancelling outranks running', () => {
    expect(aggregateIterationStatus(['cancelling', 'running'])).toBe('cancelling');
  });

  test('failure only emerges when nothing pending', () => {
    expect(aggregateIterationStatus(['failure', 'success'])).toBe('failure');
    expect(aggregateIterationStatus(['failure', 'running'])).toBe('running');
  });
});
