import { useEffect, useState } from 'react';
import { Alert, Box, Button, Chip, Paper, Stack, Typography } from '@mui/material';
import PlayCircleIcon from '@mui/icons-material/PlayCircle';
import { get } from '../api/client';
import { WorkflowPlayback } from '../components/WorkflowPlayback';

type ReviewRow = { task_run_id: string; external_run_id: string; title: string; review_status: string; project_name: string; summary: string };

export function ReviewsPage({ taskRunId }: { taskRunId?: string }) {
  const [rows, setRows] = useState<ReviewRow[]>([]);
  const [error, setError] = useState('');
  useEffect(() => {
    if (!taskRunId) get<ReviewRow[]>('/api/reviews').then(setRows).catch((e) => setError(e.message));
  }, [taskRunId]);

  if (taskRunId) return <WorkflowPlayback taskRunId={taskRunId} />;

  return (
    <Stack spacing={2}>
      <Typography variant="h4" fontWeight={700}>Review Center</Typography>
      {error && <Alert severity="warning">{error}</Alert>}
      {rows.map((row) => (
        <Paper key={row.task_run_id} variant="outlined" className="run-row">
          <Box>
            <Typography fontWeight={700}>{row.title}</Typography>
            <Typography variant="body2" color="text.secondary">{row.project_name} / {row.external_run_id}</Typography>
            <Typography variant="body2">{row.summary}</Typography>
          </Box>
          <Chip label={row.review_status} color={row.review_status === 'pass' ? 'success' : row.review_status === 'failed' ? 'error' : 'warning'} />
          <Button startIcon={<PlayCircleIcon />} onClick={() => { location.hash = `#/playback/${row.task_run_id}`; }}>Open</Button>
        </Paper>
      ))}
    </Stack>
  );
}

