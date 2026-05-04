import { useEffect, useState } from 'react';
import { Alert, Button, Chip, Paper, Stack, Typography } from '@mui/material';
import PlayCircleIcon from '@mui/icons-material/PlayCircle';
import { get } from '../api/client';
import type { TaskRun } from '../types';

type TaskRunListItem = TaskRun & { task_run_id?: string };
type TaskRunsResponse = {
  auto_import?: unknown;
  items?: TaskRunListItem[] | null;
};

export function TaskRunsPanel({ projectId, reviews = false }: { projectId: string; reviews?: boolean }) {
  const [items, setItems] = useState<TaskRunListItem[]>([]);
  const [summary, setSummary] = useState<unknown>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError('');
    get<TaskRunsResponse>(`/api/projects/${projectId}/${reviews ? 'reviews' : 'task-runs'}?auto_import=true`)
      .then((r) => {
        setItems(Array.isArray(r.items) ? r.items : []);
        setSummary(r.auto_import ?? null);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [projectId, reviews]);

  if (error) return <Alert severity="warning">{error}</Alert>;
  if (loading) return <Alert severity="info">Loading {reviews ? 'reviews' : 'task runs'}...</Alert>;

  return (
    <Stack spacing={1}>
      {summary ? <Alert severity="info">Auto import: {JSON.stringify(summary)}</Alert> : null}
      {items.length === 0 ? (
        <Alert severity="info">{reviews ? 'No reviews found for this project yet.' : 'No task runs imported for this project yet.'}</Alert>
      ) : null}
      {items.map((run) => (
        <Paper key={run.id || run.task_run_id} variant="outlined" className="run-row">
          <div>
            <Typography fontWeight={700}>{run.title}</Typography>
            <Typography variant="body2" color="text.secondary">{run.external_run_id}</Typography>
          </div>
          <Chip label={run.review_status} color={run.review_status === 'pass' ? 'success' : 'warning'} />
          <Button startIcon={<PlayCircleIcon />} onClick={() => { location.hash = `#/playback/${run.id || run.task_run_id}`; }}>Playback</Button>
        </Paper>
      ))}
    </Stack>
  );
}
