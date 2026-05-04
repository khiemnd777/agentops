import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RepoTree } from './RepoTree';

const getMock = vi.hoisted(() => vi.fn((path: string) => {
  if (path.includes('/repo-file')) {
    return Promise.resolve({
      name: 'AGENTS.md',
      path: 'AGENTS.md',
      size_bytes: 18,
      content: '# Agent guide',
      modified_at: '2026-05-04T10:00:00Z'
    });
  }
  return Promise.resolve({
    root: {
      name: 'repo',
      path: '.',
      type: 'directory',
      children: [
        { name: 'empty', path: 'empty', type: 'directory', children: [] },
        { name: 'AGENTS.md', path: 'AGENTS.md', type: 'managed_file' }
      ]
    }
  });
}));
const putMock = vi.hoisted(() => vi.fn());

vi.mock('../api/client', () => ({
  get: getMock,
  put: putMock
}));

describe('RepoTree', () => {
  afterEach(() => {
    cleanup();
  });

  it('renders managed files', async () => {
    render(<RepoTree projectId="p1" />);
    expect(await screen.findByText('AGENTS.md')).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith('/api/projects/p1/repo-tree?max_depth=8&show_managed_files=true');
  });

  it('previews a managed file after click', async () => {
    render(<RepoTree projectId="p1" />);
    fireEvent.click(await screen.findByRole('button', { name: /AGENTS.md/ }));
    expect(await screen.findByText('Agent guide')).toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Save' })).toBeDisabled();
    expect(getMock).toHaveBeenCalledWith('/api/projects/p1/repo-file?path=AGENTS.md');
  });

  it('collapses and expands folders', async () => {
    render(<RepoTree projectId="p1" />);
    const root = await screen.findByRole('button', { name: /repo/ });
    expect(screen.getByRole('button', { name: /AGENTS.md/ })).toBeInTheDocument();

    fireEvent.click(root);
    expect(screen.queryByRole('button', { name: /AGENTS.md/ })).not.toBeInTheDocument();

    fireEvent.click(root);
    expect(screen.getByRole('button', { name: /AGENTS.md/ })).toBeInTheDocument();
  });

  it('does not mark empty folders as expandable', async () => {
    render(<RepoTree projectId="p1" />);
    const empty = await screen.findByRole('button', { name: /empty/ });
    expect(empty).not.toHaveAttribute('aria-expanded');
  });
});
