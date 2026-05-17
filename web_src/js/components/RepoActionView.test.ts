import {decorateJobsForMatrixGrouping} from './ActionRunView.ts';
import type {ActionsJob} from '../modules/gitea-actions.ts';

function makeJob(id: number, jobId: string, name = jobId): ActionsJob {
  return {
    id,
    link: `/run/job/${id}`,
    jobId,
    name,
    status: 'success',
    canRerun: false,
    duration: '1s',
  };
}

describe('decorateJobsForMatrixGrouping', () => {
  test('empty list', () => {
    expect(decorateJobsForMatrixGrouping([])).toEqual([]);
  });

  test('single job is not a matrix child', () => {
    const out = decorateJobsForMatrixGrouping([makeJob(1, 'build')]);
    expect(out).toHaveLength(1);
    expect(out[0].isMatrixChild).toBe(false);
  });

  test('siblings with same jobId: first is parent, rest are children', () => {
    const out = decorateJobsForMatrixGrouping([
      makeJob(1, 'build', 'build (linux)'),
      makeJob(2, 'build', 'build (windows)'),
      makeJob(3, 'build', 'build (macos)'),
    ]);
    expect(out.map((d) => d.isMatrixChild)).toEqual([false, true, true]);
  });

  test('distinct jobIds are never grouped', () => {
    const out = decorateJobsForMatrixGrouping([
      makeJob(1, 'lint'),
      makeJob(2, 'test'),
      makeJob(3, 'deploy'),
    ]);
    expect(out.map((d) => d.isMatrixChild)).toEqual([false, false, false]);
  });

  test('mixed matrix and non-matrix jobs', () => {
    const out = decorateJobsForMatrixGrouping([
      makeJob(1, 'job1'),
      makeJob(2, 'job2', 'job2 (afile)'),
      makeJob(3, 'job2', 'job2 (another file)'),
      makeJob(4, 'job3'),
    ]);
    expect(out.map((d) => d.isMatrixChild)).toEqual([false, false, true, false]);
  });

  test('preserves the job objects', () => {
    const job = makeJob(42, 'build');
    const [decorated] = decorateJobsForMatrixGrouping([job]);
    expect(decorated.job).toBe(job);
  });
});
