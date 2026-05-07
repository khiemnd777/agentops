import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkspaceV2Page } from './WorkspaceV2Page';

const apiMock = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn()
}));

vi.mock('../api/client', () => ({
  get: apiMock.get,
  post: apiMock.post
}));

vi.mock('../components/RepoTree', () => ({
  RepoTree: ({ projectId }: { projectId: string }) => <div>RepoTree {projectId}</div>
}));

const project = {
  id: 'p1',
  name: 'Demo',
  slug: 'demo',
  repo_path: '/repo/demo',
  default_branch: 'main',
  description: '',
  status: 'active'
};

describe('WorkspaceV2Page', () => {
  beforeEach(() => {
    apiMock.get.mockReset();
    apiMock.post.mockReset();
    apiMock.get.mockImplementation((path: string) => {
      if (path === '/api/projects') return Promise.resolve([project]);
      if (path.includes('/sync-runs')) return Promise.resolve([]);
      return Promise.reject(new Error('not found'));
    });
  });

  afterEach(() => {
    cleanup();
  });

  it('renders the V2 shell and reuses the repository tree', async () => {
    render(<WorkspaceV2Page projectId="p1" section="repository" />);

    expect(await screen.findByText('Workspace V2')).toBeInTheDocument();
    expect(screen.getByText('Codex control plane')).toBeInTheDocument();
    expect(screen.getAllByText('Repository').length).toBeGreaterThan(1);
    expect(screen.getByRole('tab', { name: /Repository/ })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /Agent Setup/ })).toBeInTheDocument();
    expect(screen.getByText('RepoTree p1')).toBeInTheDocument();
  });

  it('renders project agent setup without exposing a token value', async () => {
    render(<WorkspaceV2Page projectId="p1" section="agent-setup" />);

    expect(await screen.findByText('Project Agent Setup')).toBeInTheDocument();
    expect(screen.getByText('AGENTOPS_MCP_TOKEN')).toBeInTheDocument();
    expect(screen.getByText('.codex/reports/runs/{run_id}/run.report.json')).toBeInTheDocument();
    await waitFor(() => expect(apiMock.get).toHaveBeenCalledWith('/api/projects'));
  });
});
