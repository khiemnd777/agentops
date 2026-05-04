import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { WorkflowPlayback } from './WorkflowPlayback';

vi.mock('../api/client', () => ({
  get: () => Promise.resolve({
    task_run_id: 'tr1',
    external_run_id: 'run_1',
    workflow: { id: 'fullstack-feature-workflow', version: '1.0.0' },
    review_status: 'pass',
    nodes: [{ id: 'repo_architect', label: 'Repo Architect', type: 'skill', status: 'completed', purpose: 'Analyze repo', action_summary: ['Scanned'], output_summary: ['Plan'] }],
    edges: [],
    events: [{ node_key: 'repo_architect', event_type: 'completed', event_time: '2026-05-04T10:00:00Z', summary: 'Done' }],
    review: { summary: 'Passed', violations: [], warnings: [] }
  })
}));

describe('WorkflowPlayback', () => {
  it('renders playback controls and node detail', async () => {
    render(<WorkflowPlayback taskRunId="tr1" />);
    expect(await screen.findByText('run_1')).toBeInTheDocument();
    expect(await screen.findAllByText('Repo Architect')).toHaveLength(2);
    expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
    expect(screen.getByText('Play')).toBeInTheDocument();
    expect(screen.getByText('Previous')).toBeInTheDocument();
  });
});
