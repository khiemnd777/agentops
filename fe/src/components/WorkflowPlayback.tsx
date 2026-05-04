import { useEffect, useMemo, useState } from 'react';
import { ReactFlow, Background, Controls, Edge, MarkerType, Node } from '@xyflow/react';
import { Alert, Box, Button, Chip, Divider, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import PlayArrowIcon from '@mui/icons-material/PlayArrow';
import PauseIcon from '@mui/icons-material/Pause';
import RestartAltIcon from '@mui/icons-material/RestartAlt';
import SkipNextIcon from '@mui/icons-material/SkipNext';
import SkipPreviousIcon from '@mui/icons-material/SkipPrevious';
import { motion } from 'framer-motion';
import dagre from 'dagre';
import { get, post } from '../api/client';
import type { PlaybackNode, PlaybackResponse } from '../types';
import { usePlaybackStore } from '../store/playback';

export function WorkflowPlayback({ taskRunId }: { taskRunId: string }) {
  const [data, setData] = useState<PlaybackResponse | null>(null);
  const [selected, setSelected] = useState<PlaybackNode | null>(null);
  const [tab, setTab] = useState<'flow' | 'timeline' | 'compliance'>('flow');
  const [manualSummary, setManualSummary] = useState('');
  const [error, setError] = useState('');
  const { playing, index, speed, setPlaying, restart, step, setSpeed } = usePlaybackStore();

  useEffect(() => {
    get<PlaybackResponse>(`/api/task-runs/${taskRunId}/playback?auto_import=true`).then((r) => {
      setData(r);
      setSelected(r.nodes[0] || null);
    }).catch((e) => setError(e.message));
  }, [taskRunId]);

  useEffect(() => {
    if (!playing || !data) return;
    const timer = window.setInterval(() => {
      usePlaybackStore.setState((s) => ({ index: Math.min(data.events.length, s.index + 1), playing: s.index + 1 < data.events.length }));
    }, 900 / speed);
    return () => window.clearInterval(timer);
  }, [playing, speed, data]);

  const activeNode = data?.events[Math.max(0, index - 1)]?.node_key;
  const flow = useMemo(() => toFlow(data, activeNode), [data, activeNode]);
  const goBack = () => {
    if (window.history.length > 1) {
      window.history.back();
      return;
    }
    location.hash = '#/reviews';
  };

  if (error) return <Alert severity="warning">{error}</Alert>;
  if (!data) return <Alert severity="info">Loading playback...</Alert>;

  return (
    <Box className="playback-layout">
      <Box className="playback-header">
        <Box className="playback-title-group">
          <Button startIcon={<ArrowBackIcon />} variant="outlined" onClick={goBack} sx={{ alignSelf: 'flex-start' }}>
            Back
          </Button>
          <Box>
            <Typography variant="h4" fontWeight={700}>{data.external_run_id}</Typography>
            <Typography color="text.secondary">{data.workflow.id} {data.workflow.version}</Typography>
          </Box>
        </Box>
        <Chip label={data.review_status} color={data.review_status === 'pass' ? 'success' : data.review_status === 'failed' ? 'error' : 'warning'} />
        <Stack direction="row" spacing={1}>
          <Button startIcon={playing ? <PauseIcon /> : <PlayArrowIcon />} variant="contained" onClick={() => setPlaying(!playing)}>{playing ? 'Pause' : 'Play'}</Button>
          <Button startIcon={<RestartAltIcon />} onClick={restart}>Restart</Button>
          <Button startIcon={<SkipPreviousIcon />} onClick={() => step(-1, data.events.length)}>Previous</Button>
          <Button startIcon={<SkipNextIcon />} onClick={() => step(1, data.events.length)}>Step</Button>
          <TextField select size="small" label="Speed" value={speed} onChange={(e) => setSpeed(Number(e.target.value))} sx={{ width: 110 }}>
            {[0.5, 1, 2, 4].map((s) => <MenuItem key={s} value={s}>{s}x</MenuItem>)}
          </TextField>
        </Stack>
      </Box>
      {data.import_summary && <Alert severity="info">Auto import: {JSON.stringify(data.import_summary)}</Alert>}
      <Box className="review-grid">
        <Paper variant="outlined" className="left-panel">
          <Typography variant="subtitle2">Nodes</Typography>
          <Stack spacing={1}>
            {data.nodes.map((n, idx) => (
              <Button key={n.id} className={activeNode === n.id ? 'active-list-item' : ''} onClick={() => setSelected(n)}>{idx + 1}. {n.label || n.id}</Button>
            ))}
          </Stack>
          <Divider sx={{ my: 2 }} />
          <Typography variant="subtitle2">Legend</Typography>
          <Typography variant="body2">Blue pulse = running</Typography>
          <Typography variant="body2">Green = completed</Typography>
          <Typography variant="body2">Red = failed</Typography>
          <Typography variant="body2">Gray = pending</Typography>
          <Typography variant="body2">Purple badge = needs review</Typography>
        </Paper>
        <Paper variant="outlined" className="flow-panel">
          <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
            <Button variant={tab === 'flow' ? 'contained' : 'outlined'} onClick={() => setTab('flow')}>Flow View</Button>
            <Button variant={tab === 'timeline' ? 'contained' : 'outlined'} onClick={() => setTab('timeline')}>Timeline View</Button>
            <Button variant={tab === 'compliance' ? 'contained' : 'outlined'} onClick={() => setTab('compliance')}>Compliance View</Button>
          </Stack>
          {tab === 'flow' && (
            <ReactFlow nodes={flow.nodes} edges={flow.edges} fitView onNodeClick={(_, node: Node) => setSelected(data.nodes.find((n) => n.id === node.id) || null)}>
              <Background />
              <Controls />
            </ReactFlow>
          )}
          {tab === 'timeline' && <pre className="code-block">{JSON.stringify(data.events, null, 2)}</pre>}
          {tab === 'compliance' && <pre className="code-block">{JSON.stringify(data.review, null, 2)}</pre>}
        </Paper>
        <Paper variant="outlined" className="right-panel">
          <NodeDetail node={selected} events={data.events.filter((e) => e.node_key === selected?.id)} />
          <Divider sx={{ my: 2 }} />
          <Typography variant="subtitle2">Manual review</Typography>
          <TextField fullWidth size="small" label="Summary" value={manualSummary} onChange={(e) => setManualSummary(e.target.value)} sx={{ my: 1 }} />
          <Stack direction="row" spacing={1}>
            {['pass', 'warning', 'failed', 'needs_manual_review'].map((status) => (
              <Button key={status} size="small" variant="outlined" onClick={async () => {
                await post(`/api/task-runs/${taskRunId}/review`, { review_status: status, summary: manualSummary || `Manual review marked ${status}.` });
                const refreshed = await get<PlaybackResponse>(`/api/task-runs/${taskRunId}/playback?auto_import=false`);
                setData(refreshed);
              }}>{status}</Button>
            ))}
          </Stack>
        </Paper>
      </Box>
    </Box>
  );
}

function toFlow(data: PlaybackResponse | null, activeNode?: string): { nodes: Node[]; edges: Edge[] } {
  if (!data) return { nodes: [], edges: [] };
  const positions = dagreLayout(data.nodes, data.edges);
  return {
    nodes: data.nodes.map((n) => ({
      id: n.id,
      position: positions[n.id] || n.position || { x: 0, y: 0 },
      data: { label: <FlowNode node={n} active={activeNode === n.id} /> },
      type: 'default',
      style: { border: 'none', padding: 0, background: 'transparent', width: 250 }
    })),
    edges: data.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      animated: e.source === activeNode || e.target === activeNode,
      markerEnd: { type: MarkerType.ArrowClosed },
      style: { stroke: edgeColor(e.status), strokeWidth: 2, strokeDasharray: e.status === 'skipped' ? '6 6' : undefined }
    }))
  };
}

function dagreLayout(nodes: PlaybackNode[], edges: PlaybackResponse['edges']): Record<string, { x: number; y: number }> {
  const graph = new dagre.graphlib.Graph();
  graph.setDefaultEdgeLabel(() => ({}));
  graph.setGraph({ rankdir: 'LR', nodesep: 90, ranksep: 120 });
  nodes.forEach((node) => graph.setNode(node.id, { width: 260, height: 150 }));
  edges.forEach((edge) => graph.setEdge(edge.source, edge.target));
  dagre.layout(graph);
  return Object.fromEntries(nodes.map((node) => {
    const p = graph.node(node.id);
    return [node.id, { x: (p?.x || 0) - 130, y: (p?.y || 0) - 75 }];
  }));
}

function FlowNode({ node, active }: { node: PlaybackNode; active: boolean }) {
  return (
    <motion.div className={`flow-node ${node.status} ${active ? 'active' : ''}`} animate={active ? { scale: [1, 1.04, 1] } : { scale: 1 }} transition={{ repeat: active ? Infinity : 0, duration: 1 }}>
      <div className="node-title">{node.label || node.id}</div>
      <div className="node-meta">{node.type} {node.ref ? `/${node.ref}` : ''}</div>
      <div className="node-purpose">{node.purpose}</div>
      <div className="badge-row">
        {node.required && <span className="badge">Required</span>}
        {node.unexpected && <span className="badge purple">Unexpected</span>}
        <span className={`badge ${node.status}`}>{node.status}</span>
      </div>
    </motion.div>
  );
}

function NodeDetail({ node, events }: { node: PlaybackNode | null; events: Array<{ event_type: string; event_time: string; summary: string }> }) {
  if (!node) return <Typography>Select a node.</Typography>;
  return (
    <>
      <Typography variant="h6">{node.label}</Typography>
      <Typography color="text.secondary">{node.type} / {node.ref}</Typography>
      <Chip label={node.status} sx={{ my: 1 }} />
      <Typography variant="subtitle2">Purpose</Typography>
      <Typography variant="body2">{node.purpose || 'No purpose recorded.'}</Typography>
      <Typography variant="subtitle2" sx={{ mt: 2 }}>Input Summary</Typography>
      <Typography variant="body2">{node.input_summary || 'No input summary.'}</Typography>
      <Typography variant="subtitle2" sx={{ mt: 2 }}>What happened</Typography>
      <ul>{(node.action_summary || []).map((x) => <li key={x}>{x}</li>)}</ul>
      <Typography variant="subtitle2">Output Summary</Typography>
      <ul>{(node.output_summary || []).map((x) => <li key={x}>{x}</li>)}</ul>
      <Typography variant="subtitle2">Compliance</Typography>
      <pre className="code-block small">{JSON.stringify(node.compliance, null, 2)}</pre>
      <Typography variant="subtitle2">Mini Timeline</Typography>
      {events.map((e) => <Typography key={`${e.event_time}-${e.event_type}`} variant="body2">{e.event_type}: {e.summary}</Typography>)}
    </>
  );
}

function edgeColor(status: string) {
  if (status === 'completed') return '#2f8f46';
  if (status === 'failed') return '#b42318';
  if (status === 'blocked' || status === 'needs_review') return '#7c3aed';
  if (status === 'skipped') return '#7b7f87';
  return '#2f6fed';
}
