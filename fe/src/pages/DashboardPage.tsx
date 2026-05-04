import { useEffect, useState } from 'react';
import { Alert, Box, Chip, Paper, Stack, Typography } from '@mui/material';
import { get } from '../api/client';

export function DashboardPage() {
  const [data, setData] = useState<Record<string, any>>({});
  const [error, setError] = useState('');

  useEffect(() => {
    get<Record<string, number>>('/api/dashboard').then(setData).catch((e) => setError(e.message));
  }, []);

  return (
    <Stack spacing={2}>
      <Box>
        <Typography variant="h4" fontWeight={700}>Workspace Dashboard</Typography>
        <Typography color="text.secondary">Project health, asset drift, imports, and workflow review status.</Typography>
      </Box>
      {error && <Alert severity="warning">{error}</Alert>}
      <Box className="metric-grid">
        <Metric title="Projects" value={data.project_count ?? 0} />
        <Metric title="Assets" value={data.asset_count ?? 0} />
        <Metric title="Task runs" value={data.task_run_count ?? 0} />
        <Metric title="Drifted bindings" value={data.drifted_bindings ?? 0} />
        <Metric title="Open reviews" value={data.open_reviews ?? 0} />
        <Metric title="Import errors" value={data.import_error_count ?? 0} />
        <Metric title="Compliance pass rate" value={passRate(data)} />
      </Box>
      <Box className="dashboard-columns">
        <Paper variant="outlined" className="panel">
          <Typography variant="h6">Recent task reviews</Typography>
          {(data.recent_reviews || []).map((item: any) => (
            <Box key={item.id} className="compact-row">
              <Box>
                <Typography fontWeight={700}>{item.title}</Typography>
                <Typography variant="body2" color="text.secondary">{item.project_name} / {item.external_run_id}</Typography>
              </Box>
              <Chip label={item.review_status} size="small" />
            </Box>
          ))}
        </Paper>
        <Paper variant="outlined" className="panel">
          <Typography variant="h6">Recent import errors</Typography>
          {(data.recent_import_errors || []).length === 0 && <Typography color="text.secondary">No recent import errors.</Typography>}
          {(data.recent_import_errors || []).map((item: any) => (
            <Box key={`${item.run_id}-${item.created_at}`} className="compact-row">
              <Box>
                <Typography fontWeight={700}>{item.run_id}</Typography>
                <Typography variant="body2" color="text.secondary">{item.project_name} / {item.error_message}</Typography>
              </Box>
              <Chip label={item.import_status} size="small" color="error" />
            </Box>
          ))}
        </Paper>
      </Box>
    </Stack>
  );
}

function passRate(data: Record<string, any>) {
  const total = data.total_reviews ?? 0;
  if (!total) return '0%';
  return `${Math.round(((data.passed_reviews ?? 0) / total) * 100)}%`;
}

function Metric({ title, value }: { title: string; value: number | string }) {
  return (
    <Paper className="metric-card" variant="outlined">
      <Typography variant="overline">{title}</Typography>
      <Typography variant="h3" fontWeight={700}>{value}</Typography>
    </Paper>
  );
}
