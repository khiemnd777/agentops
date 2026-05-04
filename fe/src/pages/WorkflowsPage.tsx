import { useEffect, useMemo, useState } from 'react';
import Editor from '@monaco-editor/react';
import { ReactFlow, Background, Controls, Edge, MarkerType, Node } from '@xyflow/react';
import { Alert, Box, Button, Chip, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import SaveIcon from '@mui/icons-material/Save';
import AccountTreeIcon from '@mui/icons-material/AccountTree';
import { get, post, put } from '../api/client';
import type { WorkflowTemplate } from '../types';

const starterYaml = `id: custom-workflow
name: Custom Workflow
version: 0.1.0
nodes:
  - id: task_start
    type: task_start
    label: Task Start
    purpose: Mark the beginning of the workflow.
    required: true
`;

export function WorkflowsPage() {
  const [workflows, setWorkflows] = useState<WorkflowTemplate[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [selected, setSelected] = useState<WorkflowTemplate | null>(null);
  const [yaml, setYaml] = useState(starterYaml);
  const [graph, setGraph] = useState<{ nodes: any[]; edges: any[] } | null>(null);
  const [error, setError] = useState('');

  const load = () => get<WorkflowTemplate[]>('/api/workflows').then(setWorkflows).catch((e) => setError(e.message));
  useEffect(() => { load(); }, []);
  useEffect(() => {
    if (!selectedId) return;
    get<WorkflowTemplate>(`/api/workflows/${selectedId}`).then((wf) => {
      setSelected(wf);
      setYaml(wf.definition_yaml);
      setGraph(null);
    }).catch((e) => setError(e.message));
  }, [selectedId]);

  const flow = useMemo(() => toFlow(graph), [graph]);
  const preview = async () => {
    const res = await post<{ nodes: any[]; edges: any[] }>(`/api/workflows/${selectedId || 'preview'}/preview-graph`, { definition_yaml: yaml });
    setGraph(res);
  };
  const save = async () => {
    if (selected) {
      await put(`/api/workflows/${selected.id}`, { name: selected.name, definition_yaml: yaml, status: selected.status });
    } else {
      await post('/api/workflows', { definition_yaml: yaml, status: 'draft' });
    }
    await load();
  };

  return (
    <Stack spacing={2}>
      <Box className="page-header">
        <Box>
          <Typography variant="h4" fontWeight={700}>Workflow Designer</Typography>
          <Typography color="text.secondary">Edit YAML and preview the expected workflow graph.</Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button startIcon={<AccountTreeIcon />} variant="outlined" onClick={preview}>Preview graph</Button>
          <Button startIcon={<SaveIcon />} variant="contained" onClick={save}>Save</Button>
        </Stack>
      </Box>
      {error && <Alert severity="warning">{error}</Alert>}
      <TextField select label="Workflow" value={selectedId} onChange={(e) => setSelectedId(e.target.value)}>
        <MenuItem value="">New workflow</MenuItem>
        {workflows.map((wf) => <MenuItem key={wf.id} value={wf.id}>{wf.slug} {wf.version}</MenuItem>)}
      </TextField>
      <Box className="editor-grid">
        <Paper variant="outlined" className="editor-box">
          <Editor height="640px" language="yaml" value={yaml} onChange={(v) => setYaml(v || '')} options={{ minimap: { enabled: false }, wordWrap: 'on' }} />
        </Paper>
        <Paper variant="outlined" className="flow-panel">
          {graph ? (
            <ReactFlow nodes={flow.nodes} edges={flow.edges} fitView>
              <Background />
              <Controls />
            </ReactFlow>
          ) : (
            <Alert severity="info">Preview renders expected nodes and dependencies from YAML.</Alert>
          )}
          {selected && <Chip label={selected.status} sx={{ mt: 1 }} />}
        </Paper>
      </Box>
    </Stack>
  );
}

function toFlow(graph: { nodes: any[]; edges: any[] } | null): { nodes: Node[]; edges: Edge[] } {
  if (!graph) return { nodes: [], edges: [] };
  return {
    nodes: graph.nodes.map((n) => ({
      id: n.id,
      position: n.position || { x: 0, y: 0 },
      data: { label: <div className="workflow-preview-node"><strong>{n.label}</strong><br /><span>{n.type}</span></div> }
    })),
    edges: graph.edges.map((e) => ({ id: e.id, source: e.source, target: e.target, markerEnd: { type: MarkerType.ArrowClosed } }))
  };
}

