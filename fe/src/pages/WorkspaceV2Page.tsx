import { useEffect, useMemo, useState } from 'react';
import { Alert, Box, Button, Chip, Collapse, Divider, List, ListItemButton, ListItemIcon, ListItemText, MenuItem, Paper, Stack, Tab, Tabs, TextField, Typography } from '@mui/material';
import AccountTreeIcon from '@mui/icons-material/AccountTree';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import ArticleIcon from '@mui/icons-material/Article';
import FolderIcon from '@mui/icons-material/Folder';
import HubIcon from '@mui/icons-material/Hub';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight';
import LanIcon from '@mui/icons-material/Lan';
import SyncIcon from '@mui/icons-material/Sync';
import { get, post } from '../api/client';
import { RepoTree } from '../components/RepoTree';
import type { Project, SyncResponse, SyncRun } from '../types';

type WorkspaceV2PageProps = {
  projectId?: string;
  section?: string;
};

const sections = [
  { key: 'repository', label: 'Repository', icon: <FolderIcon fontSize="small" /> },
  { key: 'agent-setup', label: 'Agent Setup', icon: <LanIcon fontSize="small" /> },
  { key: 'manifest', label: 'Manifest', icon: <ArticleIcon fontSize="small" /> },
  { key: 'sync', label: 'Codex Sync', icon: <SyncIcon fontSize="small" /> }
];

export function WorkspaceV2Page({ projectId = '', section = 'repository' }: WorkspaceV2PageProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [error, setError] = useState('');
  const [expandedProjects, setExpandedProjects] = useState<Set<string>>(new Set());

  useEffect(() => {
    get<Project[] | null>('/api/projects')
      .then((items) => {
        const list = Array.isArray(items) ? items : [];
        setProjects(list);
        if (!projectId && list[0]) {
          location.hash = `#/workspace-v2/project/${list[0].id}/repository`;
        }
      })
      .catch((e) => setError(e.message));
  }, [projectId]);

  const project = useMemo(() => projects.find((item) => item.id === projectId) || projects[0] || null, [projectId, projects]);
  const activeSection = sections.some((item) => item.key === section) ? section : 'repository';

  useEffect(() => {
    if (!project?.id) return;
    setExpandedProjects((current) => new Set(current).add(project.id));
  }, [project?.id]);

  const toggleProject = (id: string) => {
    setExpandedProjects((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <Box className="workspace-v2-shell">
      <Box component="aside" className="workspace-v2-sidebar">
        <Box className="workspace-v2-brand">
          <Stack direction="row" spacing={1.2} alignItems="center">
            <Box className="workspace-v2-brand-icon"><HubIcon fontSize="small" /></Box>
            <Box>
              <Typography variant="h6" fontWeight={900} lineHeight={1.1}>Workspace V2</Typography>
              <Typography variant="body2" color="text.secondary">Codex control plane</Typography>
            </Box>
          </Stack>
        </Box>
        <Divider />
        <Box className="sidebar-section-label">Projects</Box>
        <List className="sidebar-nav workspace-v2-projects" disablePadding>
          {projects.length === 0 && <Box className="sidebar-empty">No projects found.</Box>}
          {projects.map((item) => {
            const selected = project?.id === item.id;
            const expanded = selected || expandedProjects.has(item.id);
            return (
              <Box key={item.id} className="workspace-v2-project-node">
                <ListItemButton
                  selected={selected}
                  onClick={() => {
                    toggleProject(item.id);
                    location.hash = `#/workspace-v2/project/${item.id}/${activeSection}`;
                  }}
                >
                  <ListItemIcon><FolderIcon fontSize="small" /></ListItemIcon>
                  <ListItemText primary={item.name} secondary={item.slug} />
                  {expanded ? <KeyboardArrowDownIcon fontSize="small" /> : <KeyboardArrowRightIcon fontSize="small" />}
                </ListItemButton>
                <Collapse in={expanded} timeout="auto" unmountOnExit>
                  <List className="workspace-v2-project-sections" disablePadding>
                    {sections.map((sectionItem) => (
                      <ListItemButton
                        key={sectionItem.key}
                        selected={selected && activeSection === sectionItem.key}
                        onClick={() => { location.hash = `#/workspace-v2/project/${item.id}/${sectionItem.key}`; }}
                      >
                        <ListItemIcon>{sectionItem.icon}</ListItemIcon>
                        <ListItemText primary={sectionItem.label} />
                      </ListItemButton>
                    ))}
                  </List>
                </Collapse>
              </Box>
            );
          })}
        </List>
        <Divider sx={{ mt: 'auto' }} />
        <Box sx={{ p: 1.5 }}>
          <Button fullWidth startIcon={<ArrowBackIcon />} variant="outlined" onClick={() => { location.hash = '#/dashboard'; }}>
            Classic workspace
          </Button>
        </Box>
      </Box>
      <Box component="main" className="workspace-v2-content">
        {error && <Alert severity="warning">{error}</Alert>}
        {!project && !error && <Alert severity="info">Create a project in the classic workspace before opening Workspace V2.</Alert>}
        {project && (
          <Stack spacing={2}>
            <Box className="workspace-v2-header">
              <Box>
                <Typography variant="h4" fontWeight={900}>{project.name}</Typography>
                <Typography color="text.secondary">{project.repo_path}</Typography>
              </Box>
              <Stack direction="row" spacing={1} flexWrap="wrap" justifyContent="flex-end">
                <Chip label={project.slug} variant="outlined" />
                <Chip label={project.status} color={project.status === 'active' ? 'success' : 'default'} variant="outlined" />
              </Stack>
            </Box>
            <Box className="project-context-tabs">
              <Tabs
                value={activeSection}
                onChange={(_, nextSection) => { location.hash = `#/workspace-v2/project/${project.id}/${nextSection}`; }}
                variant="scrollable"
                scrollButtons="auto"
              >
                {sections.map((sectionItem) => (
                  <Tab
                    key={sectionItem.key}
                    value={sectionItem.key}
                    icon={sectionItem.icon}
                    iconPosition="start"
                    label={sectionItem.label}
                  />
                ))}
              </Tabs>
            </Box>
            {activeSection === 'repository' && <ProjectRepositoryPanel project={project} />}
            {activeSection === 'agent-setup' && <ProjectAgentSetupPanel project={project} />}
            {activeSection === 'manifest' && <ProjectManifestPanel project={project} />}
            {activeSection === 'sync' && <ProjectSyncPanel project={project} />}
          </Stack>
        )}
      </Box>
    </Box>
  );
}

function ProjectRepositoryPanel({ project }: { project: Project }) {
  return (
    <Stack spacing={1.5}>
      <SectionTitle title="Project Repository" subtitle="Codex-managed repository view. This panel reuses the existing repository tree only." />
      <RepoTree projectId={project.id} />
    </Stack>
  );
}

function ProjectAgentSetupPanel({ project }: { project: Project }) {
  const mcpEndpoint = `${window.location.origin}/mcp`;
  return (
    <Paper variant="outlined" className="workspace-v2-panel">
      <SectionTitle title="Project Agent Setup" subtitle="Non-secret connection details for project repo agents." />
      <Box className="workspace-v2-kv-grid">
        <InfoTile label="Project" value={project.slug} />
        <InfoTile label="MCP endpoint" value={mcpEndpoint} />
        <InfoTile label="Token env" value="AGENTOPS_MCP_TOKEN" />
        <InfoTile label="Report path" value=".codex/reports/runs/{run_id}/run.report.json" />
        <InfoTile label="Manifest" value=".codex/project.json" />
        <InfoTile label="Sync lock" value=".codex/sync/lock.json" />
      </Box>
      <Divider sx={{ my: 2 }} />
      <Stack direction="row" spacing={1} flexWrap="wrap">
        {['AGENTS.md', '.codex/**'].map((item) => <Chip key={item} label={item} />)}
      </Stack>
      <Box className="workspace-v2-flow">
        <Typography variant="subtitle2" fontWeight={900}>Expected project agent flow</Typography>
        <Typography variant="body2">{'Read `.codex/project.json`, inspect `.codex/sync/lock.json`, edit managed files, preview `files -> DB`, sync drafts, then submit a run report.'}</Typography>
      </Box>
    </Paper>
  );
}

function ProjectManifestPanel({ project }: { project: Project }) {
  return (
    <Stack spacing={1.5}>
      <SectionTitle title="Codex Manifest" subtitle="Read-only view of generated project manifest and lock files when they exist in the repo." />
      <Box className="workspace-v2-two-col">
        <RepoManagedFile projectId={project.id} path=".codex/project.json" title="project.json" />
        <RepoManagedFile projectId={project.id} path=".codex/sync/lock.json" title="lock.json" />
      </Box>
    </Stack>
  );
}

function ProjectSyncPanel({ project }: { project: Project }) {
  const [mode, setMode] = useState('REPO_TO_DB');
  const [response, setResponse] = useState<SyncResponse | null>(null);
  const [runs, setRuns] = useState<SyncRun[]>([]);
  const [error, setError] = useState('');

  const loadRuns = () => get<SyncRun[]>(`/api/projects/${project.id}/sync-runs`).then(setRuns).catch((e) => setError(e.message));
  useEffect(() => { loadRuns(); }, [project.id]);

  const run = async (apply: boolean) => {
    setError('');
    try {
      const result = await post<SyncResponse>(`/api/projects/${project.id}/sync/${apply ? 'apply' : 'preview'}`, { mode });
      setResponse(result);
      if (apply) await loadRuns();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <Stack spacing={1.5}>
      <Paper variant="outlined" className="workspace-v2-panel">
        <SectionTitle title="Codex Sync" subtitle="Preview or run Codex-native sync operations without using old composer or preset UI." />
        {error && <Alert severity="warning" sx={{ mb: 1.5 }}>{error}</Alert>}
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
          <TextField select label="Operation" value={mode} onChange={(e) => setMode(e.target.value)} sx={{ minWidth: 250 }}>
            <MenuItem value="REPO_TO_DB">Files to DB drafts</MenuItem>
            <MenuItem value="DB_TO_REPO">DB to files</MenuItem>
            <MenuItem value="COMPARE_ONLY">Compare only</MenuItem>
          </TextField>
          <Button variant="outlined" onClick={() => run(false)}>Preview</Button>
          <Button variant="contained" onClick={() => run(true)}>Apply</Button>
        </Stack>
        {response && (
          <Stack spacing={1} sx={{ mt: 2 }}>
            {response.items.map((item) => (
              <Paper key={item.target_path} variant="outlined" className="workspace-v2-sync-item">
                <Box minWidth={0}>
                  <Typography fontWeight={800} noWrap>{item.target_path}</Typography>
                  <Typography variant="body2" color="text.secondary">{item.status} / {item.action}</Typography>
                  {item.diff_summary && <Typography variant="body2">{item.diff_summary}</Typography>}
                  {item.message && <Typography variant="body2" color="text.secondary">{item.message}</Typography>}
                </Box>
                <Chip label={item.status} />
              </Paper>
            ))}
          </Stack>
        )}
      </Paper>
      <SyncHistoryTable runs={runs} />
    </Stack>
  );
}

function SyncHistoryTable({ runs }: { runs: SyncRun[] }) {
  return (
    <Paper variant="outlined" className="workspace-v2-panel">
      <SectionTitle title="Sync History" subtitle="Actor metadata shows whether sync came from API, CLI, or MCP." />
      <Stack spacing={1}>
        {runs.length === 0 && <Typography color="text.secondary">No sync runs yet.</Typography>}
        {runs.map((run) => (
          <Box key={run.id} className="workspace-v2-history-row">
            <Box minWidth={0}>
              <Typography fontWeight={800}>{run.direction}</Typography>
              <Typography variant="body2" color="text.secondary" noWrap>{run.summary || run.id}</Typography>
            </Box>
            <Stack direction="row" spacing={1} flexWrap="wrap" justifyContent="flex-end">
              <Chip size="small" label={run.status} />
              <Chip size="small" label={`${run.actor_type || 'unknown'} via ${run.transport || 'unknown'}`} variant="outlined" />
              {run.actor_name && <Chip size="small" label={run.actor_name} variant="outlined" />}
            </Stack>
          </Box>
        ))}
      </Stack>
    </Paper>
  );
}

function RepoManagedFile({ projectId, path, title }: { projectId: string; path: string; title: string }) {
  const [content, setContent] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    setContent('');
    setError('');
    get<{ content: string }>(`/api/projects/${projectId}/repo-file?path=${encodeURIComponent(path)}`)
      .then((file) => setContent(file.content))
      .catch((e) => setError(e.message));
  }, [projectId, path]);

  return (
    <Paper variant="outlined" className="workspace-v2-code-panel">
      <Typography variant="subtitle1" fontWeight={900}>{title}</Typography>
      <Typography variant="body2" color="text.secondary">{path}</Typography>
      {error ? <Alert severity="info" sx={{ mt: 1.5 }}>{error}</Alert> : <pre className="code-block workspace-v2-code">{content || 'Loading...'}</pre>}
    </Paper>
  );
}

function SectionTitle({ title, subtitle }: { title: string; subtitle: string }) {
  return (
    <Box>
      <Typography variant="h6" fontWeight={900}>{title}</Typography>
      <Typography variant="body2" color="text.secondary">{subtitle}</Typography>
    </Box>
  );
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <Box className="workspace-v2-info-tile">
      <Typography variant="caption" color="text.secondary" fontWeight={800}>{label}</Typography>
      <Typography variant="body2" fontWeight={700}>{value}</Typography>
    </Box>
  );
}
