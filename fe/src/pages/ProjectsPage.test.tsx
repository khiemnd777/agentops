import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ProjectsPage } from './ProjectsPage';

const apiMock = vi.hoisted(() => ({
  ApiError: class ApiError extends Error {
    status: number;
    code?: string;

    constructor(message: string, status: number, code?: string) {
      super(message);
      this.name = 'ApiError';
      this.status = status;
      this.code = code;
    }
  },
  get: vi.fn(),
  post: vi.fn(),
  del: vi.fn()
}));

vi.mock('../api/client', () => ({
  ApiError: apiMock.ApiError,
  get: apiMock.get,
  post: apiMock.post,
  del: apiMock.del
}));

describe('ProjectsPage', () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    apiMock.get.mockReset();
    apiMock.post.mockReset();
    apiMock.del.mockReset();
    apiMock.get.mockResolvedValue([]);
    apiMock.del.mockResolvedValue({ ok: true });
  });

  it('renders the explicit create-first project initialization wizard', async () => {
    render(<ProjectsPage />);
    expect(await screen.findByText('Project Initialization Wizard')).toBeInTheDocument();
    expect(screen.getByText('Basic info')).toBeInTheDocument();
    expect(screen.getByText('Repository location')).toBeInTheDocument();
    expect(screen.getByText('Agent file scan')).toBeInTheDocument();
    expect(screen.getByText('Apply preset')).toBeInTheDocument();
    expect(screen.getByText('Write files')).toBeInTheDocument();
    expect(screen.getByText('Complete')).toBeInTheDocument();
    expect(apiMock.post).not.toHaveBeenCalledWith(expect.stringContaining('/scan'), expect.anything());
  });

  it('auto-generates and allows editing the project slug', async () => {
    render(<ProjectsPage />);

    const nameInput = await screen.findByLabelText('Name');
    const slugInput = screen.getByLabelText('Slug');

    fireEvent.change(nameInput, { target: { value: 'Demo' } });
    expect(slugInput).toHaveValue('demo');

    fireEvent.change(slugInput, { target: { value: 'demo-app' } });
    expect(slugInput).toHaveValue('demo-app');

    fireEvent.change(nameInput, { target: { value: 'Demo Renamed' } });
    expect(slugInput).toHaveValue('demo-app');
  });

  it('asks to create a missing repository path before creating the project', async () => {
    apiMock.post.mockImplementation((path: string, body?: unknown) => {
      if (path === '/api/projects' && !(body as { create_repo_path?: boolean }).create_repo_path) {
        return Promise.reject(new apiMock.ApiError('repository path does not exist', 400, 'repo_path_not_found'));
      }
      if (path === '/api/projects') {
        return Promise.resolve({
          id: 'p1',
          name: 'Demo',
          slug: 'demo',
          repo_path: '/repo/new-demo',
          default_branch: 'main',
          description: '',
          status: 'active'
        });
      }
      return Promise.resolve({});
    });

    render(<ProjectsPage />);
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Demo' } });
    fireEvent.change(screen.getByLabelText('Repository path'), { target: { value: '/repo/new-demo' } });
    fireEvent.click(screen.getByText('Validate and create'));

    expect(await screen.findByText('Create repository folder?')).toBeInTheDocument();
    expect(screen.getByText('/repo/new-demo')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Create folder'));

    await waitFor(() => expect(apiMock.post).toHaveBeenLastCalledWith('/api/projects', expect.objectContaining({
      repo_path: '/repo/new-demo',
      create_repo_path: true
    })));
    expect(await screen.findByText('Scan project repository')).toBeInTheDocument();
  });

  it('scans only after a project is created and renders grouped candidates', async () => {
    apiMock.post.mockImplementation((path: string) => {
      if (path === '/api/projects') {
        return Promise.resolve({
          id: 'p1',
          name: 'Project One',
          slug: 'project-one',
          repo_path: '/repo/project-one',
          default_branch: 'main',
          description: '',
          status: 'active'
        });
      }
      if (path === '/api/projects/p1/scan') {
        return Promise.resolve({
          project_id: 'p1',
          repo_path: '/repo/project-one',
          git_repo: true,
          readable: true,
          scanned_files: 8,
          candidates: [
            { path: 'AGENTS.md', kind: 'agents_file', reason: 'well-known agent instruction file', size_bytes: 12, matched_keywords: ['agent'] }
          ],
          ignored_dirs: ['node_modules'],
          truncated: false
        });
      }
      return Promise.resolve({});
    });

    render(<ProjectsPage />);
    fireEvent.change(await screen.findByLabelText('Name'), { target: { value: 'Project One' } });
    fireEvent.change(screen.getByLabelText('Slug'), { target: { value: 'project-one' } });
    fireEvent.change(screen.getByLabelText('Repository path'), { target: { value: '/repo/project-one' } });
    fireEvent.click(screen.getByText('Validate and create'));

    await waitFor(() => expect(apiMock.post).toHaveBeenCalledWith('/api/projects', expect.objectContaining({ repo_path: '/repo/project-one' })));
    expect(apiMock.post).not.toHaveBeenCalledWith('/api/projects/p1/scan');

    fireEvent.click(await screen.findByText('Scan project repository'));
    expect(await screen.findByText('Agents File')).toBeInTheDocument();
    expect(screen.getByText('AGENTS.md')).toBeInTheDocument();
    expect(screen.getByText('well-known agent instruction file')).toBeInTheDocument();
    expect(screen.queryByText(/"candidates"/)).not.toBeInTheDocument();
  });

  it('keeps the Projects route on the list and form when projects already exist', async () => {
    apiMock.get.mockResolvedValue([
      { id: 'p1', name: 'Existing Project', slug: 'existing-project', repo_path: '/repo/existing', default_branch: 'main', description: '', status: 'active' }
    ]);

    render(<ProjectsPage />);
    expect(await screen.findByText('Project Initialization Wizard')).toBeInTheDocument();
    expect(screen.getByText('Existing Project')).toBeInTheDocument();
    expect(screen.queryByText('Repository Tree')).not.toBeInTheDocument();
  });

  it('renders an empty task runs state when the API returns null items', async () => {
    apiMock.get.mockImplementation((path: string) => {
      if (path === '/api/projects') {
        return Promise.resolve([
          { id: 'p1', name: 'Demo Project', slug: 'demo-project', repo_path: '/repo/demo-project', default_branch: 'main', description: '', status: 'active' }
        ]);
      }
      if (path === '/api/projects/p1/task-runs?auto_import=true') {
        return Promise.resolve({
          auto_import: { scanned: 0, imported: 0, unchanged: 0, updated: 0, invalid: 0 },
          items: null
        });
      }
      return Promise.resolve([]);
    });

    render(<ProjectsPage projectId="p1" section="task-runs" />);

    expect(await screen.findByText('No task runs imported for this project yet.')).toBeInTheDocument();
    expect(screen.getByText(/Auto import:/)).toBeInTheDocument();
  });

  it('deletes a project from the workspace without deleting repository files', async () => {
    apiMock.get.mockResolvedValue([
      { id: 'p1', name: 'Demo Project', slug: 'demo-project', repo_path: '/repo/demo-project', default_branch: 'main', description: '', status: 'active' }
    ]);

    render(<ProjectsPage />);
    expect(await screen.findByText('Demo Project')).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Delete Demo Project'));
    expect(await screen.findByText('Delete project?')).toBeInTheDocument();
    expect(screen.getByText('This removes the project from the workspace only. Repository folders and files on disk are not deleted.')).toBeInTheDocument();

    fireEvent.click(screen.getByText('Delete project'));

    await waitFor(() => expect(apiMock.del).toHaveBeenCalledWith('/api/projects/p1'));
    await waitFor(() => expect(screen.queryByText('Demo Project')).not.toBeInTheDocument());
  });

  it('shows project-owned assets and preset preview statuses in the project assets view', async () => {
    apiMock.get.mockImplementation((path: string) => {
      if (path === '/api/projects') {
        return Promise.resolve([
          { id: 'p1', name: 'Project One', slug: 'project-one', repo_path: '/repo/project-one', default_branch: 'main', description: '', status: 'active' }
        ]);
      }
      if (path === '/api/projects/p1/assets') {
        return Promise.resolve([
          { id: 'pa1', type: 'skill_doc', slug: 'project-skill', name: 'Project custom skill', description: '', status: 'draft', tags: [] }
        ]);
      }
      if (path === '/api/presets') {
        return Promise.resolve([
          { id: 'preset1', slug: 'base', name: 'Base preset', description: 'Default project assets', status: 'active' }
        ]);
      }
      if (path === '/api/presets/preset1') {
        return Promise.resolve({
          id: 'preset1',
          slug: 'base',
          name: 'Base preset',
          description: 'Default project assets',
          status: 'active',
          items: [
            {
              template_asset: { id: 'ta1', type: 'agents_md', slug: 'agents', name: 'Agent entrypoint', description: '', status: 'published', tags: [] },
              template_asset_version: { id: 'tv1', asset_id: 'ta1', version: '1.0.0', content: '', content_format: 'markdown', checksum: 'abc', status: 'published' },
              target_path: 'AGENTS.md',
              required: true,
              sort_order: 1
            }
          ]
        });
      }
      if (path === '/api/projects/p1/presets/preset1/preview') {
        return Promise.resolve({
          project_id: 'p1',
          preset_id: 'preset1',
          items: [
            {
              template_asset: { id: 'ta1', type: 'agents_md', slug: 'agents', name: 'Agent entrypoint', description: '', status: 'published', tags: [] },
              template_asset_version: { id: 'tv1', asset_id: 'ta1', version: '1.0.0', content: '', content_format: 'markdown', checksum: 'abc', status: 'published' },
              target_path: 'AGENTS.md',
              required: true,
              sort_order: 1,
              status: 'already_exists'
            }
          ]
        });
      }
      return Promise.resolve([]);
    });

    render(<ProjectsPage projectId="p1" section="assets" />);

    expect(await screen.findByText('Project custom skill')).toBeInTheDocument();
    expect(screen.getByText('Project-owned assets')).toBeInTheDocument();
    fireEvent.click(await screen.findByText('Preview'));
    expect(await screen.findByText('Already exists')).toBeInTheDocument();
    expect(apiMock.get).toHaveBeenCalledWith('/api/projects/p1/assets');
    expect(apiMock.get).toHaveBeenCalledWith('/api/projects/p1/presets/preset1/preview');
  });
});
