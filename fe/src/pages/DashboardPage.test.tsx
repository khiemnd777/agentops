import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { DashboardPage } from './DashboardPage';

vi.mock('../api/client', () => ({
  get: () => Promise.resolve({
    project_count: 1,
    asset_count: 10,
    task_run_count: 2,
    drifted_bindings: 0,
    open_reviews: 1,
    import_error_count: 0,
    passed_reviews: 1,
    total_reviews: 2,
    recent_reviews: [{ id: 'tr1', title: 'Review me', project_name: 'Demo', external_run_id: 'run_1', review_status: 'warning' }],
    recent_import_errors: []
  })
}));

describe('DashboardPage', () => {
  it('renders metrics and recent reviews', async () => {
    render(<DashboardPage />);
    expect(await screen.findByText('Review me')).toBeInTheDocument();
    expect(screen.getByText('Compliance pass rate')).toBeInTheDocument();
    expect(screen.getByText('Recent import errors')).toBeInTheDocument();
  });
});

