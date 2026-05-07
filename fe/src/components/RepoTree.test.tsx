import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RepoTree } from './RepoTree';

const getMock = vi.hoisted(() => vi.fn((path: string) => {
  if (path.includes('/repo-file')) {
    return Promise.resolve({
      name: 'AGENTS.md',
      path: 'AGENTS.md',
      size_bytes: 18,
      content: '# Agent guide',
      modified_at: '2026-05-04T10:00:00Z',
      can_write: true
    });
  }
  return Promise.resolve({
    root: {
      name: 'repo',
      path: '.',
      type: 'directory',
      children: [
        {
          name: 'empty',
          path: 'empty',
          type: 'directory',
          children: [
            { name: 'nested', path: 'empty/nested', type: 'directory', children: [] },
            { name: 'docs', path: 'empty/docs', type: 'directory', children: [{ name: 'guide.md', path: 'empty/docs/guide.md', type: 'managed_file' }] },
            { name: 'README.md', path: 'empty/README.md', type: 'managed_file' }
          ]
        },
        { name: 'AGENTS.md', path: 'AGENTS.md', type: 'managed_file' }
      ]
    }
  });
}));
const putMock = vi.hoisted(() => vi.fn());
const originalScrollIntoView = window.HTMLElement.prototype.scrollIntoView;

vi.mock('../api/client', () => ({
  get: getMock,
  put: putMock
}));

describe('RepoTree', () => {
  afterEach(() => {
    cleanup();
    if (originalScrollIntoView) window.HTMLElement.prototype.scrollIntoView = originalScrollIntoView;
    else delete (window.HTMLElement.prototype as Partial<HTMLElement>).scrollIntoView;
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
    const tree = screen.getByRole('tree', { name: /Repository tree/ });
    expect(within(tree).getByRole('button', { name: /AGENTS.md/ })).toBeInTheDocument();

    fireEvent.click(root);
    expect(within(tree).getByRole('button', { name: /AGENTS.md/ })).toBeInTheDocument();

    fireEvent.click(root.querySelector('.repo-tree-toggle')!);
    expect(within(tree).queryByRole('button', { name: /AGENTS.md/ })).not.toBeInTheDocument();

    fireEvent.click(root.querySelector('.repo-tree-toggle')!);
    expect(within(tree).getByRole('button', { name: /AGENTS.md/ })).toBeInTheDocument();
  });

  it('does not mark empty folders as expandable', async () => {
    getMock.mockImplementationOnce(() => Promise.resolve({
      root: {
        name: 'repo',
        path: '.',
        type: 'directory',
        children: [{ name: 'empty', path: 'empty', type: 'directory', children: [] }]
      }
    }));

    render(<RepoTree projectId="p1" />);
    const empty = await screen.findByRole('button', { name: /empty/ });
    expect(empty).not.toHaveAttribute('aria-expanded');
  });

  it('lists folder children in the detail panel', async () => {
    render(<RepoTree projectId="p1" />);
    const emptyFolder = (await screen.findAllByRole('button', { name: /empty/ }))[0];
    fireEvent.click(emptyFolder);

    expect(screen.getAllByText('nested').length).toBeGreaterThan(0);
    expect(screen.getAllByText('docs').length).toBeGreaterThan(0);
    expect(screen.getAllByText('README.md').length).toBeGreaterThan(0);
    expect(screen.queryByText('empty/nested')).not.toBeInTheDocument();
    expect(screen.queryByText('empty/README.md')).not.toBeInTheDocument();
    expect(screen.queryByText('1 items')).not.toBeInTheDocument();
  });

  it('shows direct item counts on folder thumbnails', async () => {
    render(<RepoTree projectId="p1" />);

    fireEvent.click(await screen.findByRole('button', { name: /repo/ }));
    expect(await screen.findByTestId('folder-thumb-count-empty')).toHaveTextContent('3');

    const emptyFolder = (await screen.findAllByRole('button', { name: /empty/ }))[0];
    fireEvent.click(emptyFolder);
    expect(screen.getByTestId('folder-thumb-count-empty/docs')).toHaveTextContent('1');
  });

  it('opens folder and file thumbnails from the detail panel', async () => {
    render(<RepoTree projectId="p1" />);
    const emptyFolder = (await screen.findAllByRole('button', { name: /empty/ }))[0];
    fireEvent.click(emptyFolder);

    const readmeThumbs = screen.getAllByText('README.md');
    fireEvent.click(readmeThumbs[readmeThumbs.length - 1]);
    expect(getMock).toHaveBeenCalledWith('/api/projects/p1/repo-file?path=empty%2FREADME.md');

    fireEvent.click(emptyFolder);
    const docsThumbs = screen.getAllByText('docs');
    fireEvent.click(docsThumbs[docsThumbs.length - 1]);
    expect(screen.getAllByText('guide.md').length).toBeGreaterThan(0);
  });

  it('auto-expands the tree when opening thumbnails from a collapsed folder', async () => {
    render(<RepoTree projectId="p1" />);
    const tree = await screen.findByRole('tree', { name: /Repository tree/ });
    const root = await within(tree).findByRole('button', { name: /repo/ });

    fireEvent.click(root);
    expect(screen.getAllByText('empty').length).toBeGreaterThan(0);

    fireEvent.click(root.querySelector('.repo-tree-toggle')!);
    expect(within(tree).queryByRole('button', { name: /empty/ })).not.toBeInTheDocument();

    const emptyThumbs = screen.getAllByText('empty');
    fireEvent.click(emptyThumbs[emptyThumbs.length - 1]);
    expect(within(tree).getAllByRole('button', { name: /empty/ }).length).toBeGreaterThan(0);
    expect(within(tree).getByRole('button', { name: /nested/ })).toBeInTheDocument();

    fireEvent.click(root.querySelector('.repo-tree-toggle')!);
    const readmeThumbs = screen.getAllByText('README.md');
    fireEvent.click(readmeThumbs[readmeThumbs.length - 1]);
    expect(within(tree).getByRole('button', { name: /README.md/ })).toBeInTheDocument();
    expect(getMock).toHaveBeenCalledWith('/api/projects/p1/repo-file?path=empty%2FREADME.md');
  });

  it('scrolls the left tree to the matching row when opening thumbnails', async () => {
    const scrollIntoView = vi.fn();
    window.HTMLElement.prototype.scrollIntoView = scrollIntoView;

    render(<RepoTree projectId="p1" />);
    const emptyFolder = (await screen.findAllByRole('button', { name: /empty/ }))[0];
    fireEvent.click(emptyFolder);

    const docsThumbs = screen.getAllByText('docs');
    fireEvent.click(docsThumbs[docsThumbs.length - 1]);

    await waitFor(() => {
      expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'center', inline: 'nearest' });
    });
  });

  it('marks non-writable managed files as read only', async () => {
    getMock.mockImplementationOnce(() => Promise.resolve({
      root: {
        name: 'repo',
        path: '.',
        type: 'directory',
        children: [{ name: 'DESIGN.md', path: 'DESIGN.md', type: 'managed_file' }]
      }
    })).mockImplementationOnce(() => Promise.resolve({
      name: 'DESIGN.md',
      path: 'DESIGN.md',
      size_bytes: 14,
      content: '# Design',
      modified_at: '2026-05-04T10:00:00Z',
      can_write: false
    }));

    render(<RepoTree projectId="p1" />);
    fireEvent.click(await screen.findByRole('button', { name: /DESIGN.md/ }));
    expect(await screen.findByText('Read only')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
  });
});
