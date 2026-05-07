import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { App } from './App';

const apiMock = vi.hoisted(() => ({
  get: vi.fn()
}));

vi.mock('./api/client', () => ({
  get: apiMock.get
}));

vi.mock('./pages/DashboardPage', () => ({
  DashboardPage: () => <div>Dashboard</div>
}));

vi.mock('./pages/ProjectsPage', () => ({
  ProjectsPage: () => <div>Projects</div>
}));

vi.mock('./pages/PresetsPage', () => ({
  PresetsPage: () => <div>Presets</div>
}));

vi.mock('./pages/ReviewsPage', () => ({
  ReviewsPage: () => <div>Reviews</div>
}));

vi.mock('./pages/WorkflowsPage', () => ({
  WorkflowsPage: () => <div>Workflows</div>
}));

vi.mock('./pages/WorkspaceV2Page', () => ({
  WorkspaceV2Page: () => <div>Workspace V2</div>
}));

describe('App', () => {
  beforeEach(() => {
    location.hash = '';
    apiMock.get.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it('treats a null projects response as an empty project list', async () => {
    apiMock.get.mockResolvedValue(null);

    render(<App />);

    expect(await screen.findByText('Dashboard')).toBeInTheDocument();
    await waitFor(() => expect(apiMock.get).toHaveBeenCalledWith('/api/projects'));
    expect(screen.getByText('Create a project to add it here.')).toBeInTheDocument();
  });
});
