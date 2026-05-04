import { useEffect, useState } from 'react';
import { Alert, Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle, IconButton, MenuItem, Paper, Stack, Step, StepLabel, Stepper, Tab, Tabs, TextField, Tooltip, Typography } from '@mui/material';
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline';
import SyncIcon from '@mui/icons-material/Sync';
import TravelExploreIcon from '@mui/icons-material/TravelExplore';
import { ApiError, del, get, post } from '../api/client';
import type { Project, ProjectScanCandidate, ProjectScanResponse } from '../types';
import { RepoTree } from '../components/RepoTree';
import { TaskRunsPanel } from '../components/TaskRunsPanel';
import { ApplyPresetPanel, ProjectComposer } from '../components/ProjectComposer';
import { WorkflowsPage } from './WorkflowsPage';

const projectContextSections = [
  { key: 'overview', label: 'Overview' },
  { key: 'repo-tree', label: 'Repository Tree' },
  { key: 'assets', label: 'Agentic Assets' },
  { key: 'sync', label: 'Sync Status' },
  { key: 'workflows', label: 'Workflows' },
  { key: 'task-runs', label: 'Task Runs' },
  { key: 'reviews', label: 'Reviews' }
];

const sectionTitles = Object.fromEntries(projectContextSections.map((item) => [item.key, item.label])) as Record<string, string>;

export function ProjectsPage({ projectId, section = 'overview' }: { projectId?: string; section?: string }) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [selected, setSelected] = useState<Project | null>(null);
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null);

  const load = () => get<Project[] | null>('/api/projects').then((items) => {
    const projectItems = Array.isArray(items) ? items : [];
    setProjects(projectItems);
    setSelected(projectId ? projectItems.find((p) => p.id === projectId) || null : null);
  }).catch((e) => setError(e.message));

  useEffect(() => { load(); }, [projectId]);

  const deleteProject = async () => {
    if (!deleteTarget) return;
    setError('');
    try {
      await del(`/api/projects/${deleteTarget.id}`);
      setProjects((items) => items.filter((p) => p.id !== deleteTarget.id));
      if (selected?.id === deleteTarget.id) {
        setSelected(null);
        location.hash = '#/projects';
      }
      setDeleteTarget(null);
      window.dispatchEvent(new CustomEvent('agentops:projects-changed'));
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const deleteDialog = (
    <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
      <DialogTitle>Delete project?</DialogTitle>
      <DialogContent>
        <DialogContentText>
          This removes the project from the workspace only. Repository folders and files on disk are not deleted.
        </DialogContentText>
        {deleteTarget ? (
          <Typography component="pre" sx={{ mt: 2, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{deleteTarget.repo_path}</Typography>
        ) : null}
      </DialogContent>
      <DialogActions>
        <Button onClick={() => setDeleteTarget(null)}>Cancel</Button>
        <Button color="error" variant="contained" onClick={deleteProject}>Delete project</Button>
      </DialogActions>
    </Dialog>
  );

  if (projectId) {
    const project = selected;
    if (!project) return <Alert severity="info">Project not found.</Alert>;
    const activeSection = sectionTitles[section] ? section : 'overview';
    return (
      <Stack spacing={2}>
        <Box className="page-header">
          <Box>
            <Typography variant="h4" fontWeight={700}>{project.name}</Typography>
            <Typography color="text.secondary">{project.repo_path}</Typography>
          </Box>
          <Stack direction="row" spacing={1} alignItems="center">
            <Chip label={project.status} color="success" variant="outlined" />
            <Tooltip title="Delete project">
              <IconButton color="error" onClick={() => setDeleteTarget(project)} aria-label={`Delete ${project.name}`}>
                <DeleteOutlineIcon />
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>
        {deleteDialog}
        <ProjectContextTabs projectId={project.id} activeSection={activeSection} />
        {activeSection === 'overview' && <ProjectOverview project={project} />}
        {activeSection === 'repo-tree' && <RepoTree projectId={project.id} />}
        {activeSection === 'assets' && <ProjectComposer projectId={project.id} />}
        {activeSection === 'sync' && <SyncPanel project={project} />}
        {activeSection === 'task-runs' && <TaskRunsPanel projectId={project.id} />}
        {activeSection === 'reviews' && <TaskRunsPanel projectId={project.id} reviews />}
        {activeSection === 'workflows' && (
          <Stack spacing={2}>
            <Alert severity="info">Workflow templates are workspace-level assets, but this view is opened in the context of {project.name}. Synced workflow docs are materialized into this project repo.</Alert>
            <WorkflowsPage />
          </Stack>
        )}
      </Stack>
    );
  }

  return (
    <Stack spacing={2}>
      <Typography variant="h4" fontWeight={700}>Projects</Typography>
      {error && <Alert severity="warning">{error}</Alert>}
      <ProjectForm onCreated={load} />
      {deleteDialog}
      <Box className="list-grid">
        {projects.map((p) => (
          <Paper key={p.id} variant="outlined" className="list-card" onClick={() => { location.hash = `#/project/${p.id}`; }}>
            <Stack direction="row" spacing={1} justifyContent="space-between" alignItems="flex-start">
              <Typography variant="h6">{p.name}</Typography>
              <Tooltip title="Delete project">
                <IconButton
                  size="small"
                  color="error"
                  onClick={(event) => {
                    event.stopPropagation();
                    setDeleteTarget(p);
                  }}
                  aria-label={`Delete ${p.name}`}
                >
                  <DeleteOutlineIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </Stack>
            <Typography color="text.secondary">{p.slug}</Typography>
            <Typography variant="body2">{p.repo_path}</Typography>
            <Chip size="small" label={p.status} sx={{ mt: 1 }} />
          </Paper>
        ))}
      </Box>
    </Stack>
  );
}

function ProjectContextTabs({ projectId, activeSection }: { projectId: string; activeSection: string }) {
  return (
    <Box className="project-context-tabs">
      <Tabs
        value={activeSection}
        onChange={(_, value) => { location.hash = `#/project/${projectId}/${value}`; }}
        variant="scrollable"
        scrollButtons="auto"
        aria-label="Project context actions"
      >
        {projectContextSections.map((item) => (
          <Tab key={item.key} value={item.key} label={item.label} />
        ))}
      </Tabs>
    </Box>
  );
}

function ProjectForm({ onCreated }: { onCreated: () => void }) {
  const [form, setForm] = useState({ name: '', slug: '', repo_path: '', description: '', default_branch: 'main' });
  const [error, setError] = useState('');
  const [activeStep, setActiveStep] = useState(0);
  const [created, setCreated] = useState<Project | null>(null);
  const [scan, setScan] = useState<ProjectScanResponse | null>(null);
  const [preview, setPreview] = useState<unknown>(null);
  const [missingRepoPath, setMissingRepoPath] = useState('');
  const steps = ['Basic info', 'Repository location', 'Agent file scan', 'Apply preset', 'Write files', 'Complete'];

  const update = (key: keyof typeof form, value: string) => setForm((f) => {
    if (key === 'name') {
      const currentAutoSlug = slugifyProjectName(f.name);
      const shouldSyncSlug = !f.slug || f.slug === currentAutoSlug;
      return { ...f, name: value, slug: shouldSyncSlug ? slugifyProjectName(value) : f.slug };
    }
    if (key === 'slug') {
      return { ...f, slug: slugifyProjectName(value) };
    }
    return { ...f, [key]: value };
  });
  const create = async (createRepoPath = false) => {
    setError('');
    try {
      const project = await post<Project>('/api/projects', { ...form, create_repo_path: createRepoPath });
      setMissingRepoPath('');
      setCreated(project);
      setActiveStep(2);
      onCreated();
      window.dispatchEvent(new CustomEvent('agentops:projects-changed'));
    } catch (e) {
      if (e instanceof ApiError && e.code === 'repo_path_not_found') {
        setMissingRepoPath(form.repo_path);
        return;
      }
      setError((e as Error).message);
    }
  };
  const scanRepo = async () => {
    if (!created) return;
    setScan(await post<ProjectScanResponse>(`/api/projects/${created.id}/scan`));
    setActiveStep(3);
  };
  const applySync = async () => {
    if (!created) return;
    setPreview(await post(`/api/projects/${created.id}/sync/apply`, { mode: 'DB_TO_REPO' }));
    setForm({ name: '', slug: '', repo_path: '', description: '', default_branch: 'main' });
    setCreated(null);
    setActiveStep(5);
    onCreated();
    window.dispatchEvent(new CustomEvent('agentops:projects-changed'));
  };

  return (
    <Paper variant="outlined" className="form-panel">
      <Typography variant="h6">Project Initialization Wizard</Typography>
      <Stepper activeStep={activeStep} alternativeLabel>
        {steps.map((step) => <Step key={step}><StepLabel>{step}</StepLabel></Step>)}
      </Stepper>
      {error && <Alert severity="error">{error}</Alert>}
      {activeStep <= 1 && (
        <>
          <Box className="form-grid">
            <TextField label="Name" value={form.name} onChange={(e) => update('name', e.target.value)} />
            <TextField label="Slug" value={form.slug} onChange={(e) => update('slug', e.target.value)} />
            <TextField label="Repository path" value={form.repo_path} onChange={(e) => update('repo_path', e.target.value)} />
            <TextField label="Default branch" value={form.default_branch} onChange={(e) => update('default_branch', e.target.value)} />
          </Box>
          <TextField fullWidth multiline minRows={2} label="Description" value={form.description} onChange={(e) => update('description', e.target.value)} />
          <Stack direction="row" spacing={1}>
            <Button variant="outlined" onClick={() => setActiveStep(Math.min(1, activeStep + 1))}>Next</Button>
            <Button variant="contained" onClick={() => create()}>Validate and create</Button>
          </Stack>
        </>
      )}
      <Dialog open={Boolean(missingRepoPath)} onClose={() => setMissingRepoPath('')}>
        <DialogTitle>Create repository folder?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            The repository path does not exist. Create this folder and continue?
          </DialogContentText>
          <Typography component="pre" sx={{ mt: 2, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{missingRepoPath}</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setMissingRepoPath('')}>Cancel</Button>
          <Button variant="contained" onClick={() => create(true)}>Create folder</Button>
        </DialogActions>
      </Dialog>
      {activeStep === 2 && (
        <Stack spacing={1}>
          <Typography variant="subtitle1" fontWeight={700}>Agent file scan</Typography>
          <Typography variant="body2" color="text.secondary">Review detected agent-related files and folders before choosing a profile.</Typography>
          <Button variant="contained" onClick={scanRepo} sx={{ alignSelf: 'flex-start' }}>Scan project repository</Button>
          {scan && <ScanResultsPanel scan={scan} />}
        </Stack>
      )}
      {activeStep === 3 && (
        <Stack spacing={1}>
          <ScanResultsPanel scan={scan} />
          {created ? <ApplyPresetPanel projectId={created.id} onApplied={() => setActiveStep(4)} /> : null}
        </Stack>
      )}
      {activeStep === 4 && <WizardPanel title="Write project files to repo" action="Write project files to repo" onAction={applySync} data={preview} />}
      {activeStep === 5 && <Alert severity="success">Project initialized and synced.</Alert>}
    </Paper>
  );
}

function slugifyProjectName(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

function WizardPanel({ title, action, onAction, data }: { title: string; action: string; onAction: () => void; data: unknown }) {
  return (
    <Stack spacing={1}>
      <Typography variant="subtitle1" fontWeight={700}>{title}</Typography>
      <Button variant="contained" onClick={onAction} sx={{ alignSelf: 'flex-start' }}>{action}</Button>
      {data ? <Alert severity="info">Preview is ready in the Sync Status panel.</Alert> : null}
    </Stack>
  );
}

function ProjectOverview({ project }: { project: Project }) {
  const [scan, setScan] = useState<ProjectScanResponse | null>(null);
  return (
    <Paper variant="outlined" className="panel">
      <Stack direction="row" spacing={1}>
        <Button startIcon={<TravelExploreIcon />} variant="outlined" onClick={() => post<ProjectScanResponse>(`/api/projects/${project.id}/scan`).then(setScan)}>Scan project repository</Button>
        <Button startIcon={<SyncIcon />} variant="contained" onClick={() => post(`/api/projects/${project.id}/sync/apply`, { mode: 'DB_TO_REPO' })}>Write project files to repo</Button>
      </Stack>
      {scan && <ScanResultsPanel scan={scan} />}
    </Paper>
  );
}

function ScanResultsPanel({ scan }: { scan: ProjectScanResponse | null }) {
  if (!scan) return null;
  const groups = groupScanCandidates(scan.candidates);
  return (
    <Stack spacing={1} className="scan-results">
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap">
        <Chip size="small" label={`${scan.scanned_files} files scanned`} />
        <Chip size="small" label={`${scan.candidates.length} candidates`} color={scan.candidates.length ? 'primary' : 'default'} variant="outlined" />
        {scan.git_repo && <Chip size="small" label="git repo" variant="outlined" />}
        {scan.truncated && <Chip size="small" label="truncated" color="warning" />}
      </Stack>
      {groups.length === 0 ? (
        <Alert severity="info">No agent-related files or folders found.</Alert>
      ) : (
        groups.map((group) => (
          <Paper key={group.key} variant="outlined" className="scan-group">
            <Stack spacing={1}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="subtitle2" fontWeight={800}>{group.label}</Typography>
                <Chip size="small" label={group.items.length} />
              </Stack>
              {group.items.map((candidate) => (
                <Box key={`${candidate.kind}:${candidate.path}`} className="scan-candidate">
                  <Box>
                    <Typography fontWeight={700}>{candidate.path}</Typography>
                    <Typography variant="body2" color="text.secondary">{candidate.reason}</Typography>
                    {candidate.matched_keywords?.length ? (
                      <Typography variant="caption" color="text.secondary">Keywords: {candidate.matched_keywords.join(', ')}</Typography>
                    ) : null}
                  </Box>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Chip size="small" label={candidate.kind} variant="outlined" />
                    <Typography variant="caption" color="text.secondary">{formatBytes(candidate.size_bytes)}</Typography>
                  </Stack>
                </Box>
              ))}
            </Stack>
          </Paper>
        ))
      )}
    </Stack>
  );
}

function groupScanCandidates(candidates: ProjectScanCandidate[]) {
  const groups = new Map<string, { key: string; label: string; items: ProjectScanCandidate[] }>();
  for (const candidate of candidates) {
    const key = candidate.kind;
    const existing = groups.get(key) || { key, label: labelForScanKind(key), items: [] };
    existing.items.push(candidate);
    groups.set(key, existing);
  }
  return Array.from(groups.values());
}

function labelForScanKind(kind: string) {
  return kind.split('_').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
}

function formatBytes(value: number) {
  if (!value) return '0 B';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function SyncPanel({ project }: { project: Project }) {
  const [items, setItems] = useState<{ mode: string; items: any[] } | null>(null);
  const [mode, setMode] = useState('DB_TO_REPO');
  const [actions, setActions] = useState<Record<string, string>>({});
  const run = (apply: boolean) => post<{ mode: string; items: any[] }>(`/api/projects/${project.id}/sync/${apply ? 'apply' : 'preview'}`, {
    mode,
    actions: Object.entries(actions).map(([target_path, action]) => ({ target_path, action }))
  }).then(setItems);
  return (
    <Paper variant="outlined" className="panel">
      <Stack spacing={0.5} sx={{ mb: 2 }}>
        <Typography variant="h6" fontWeight={700}>Write project files to repo</Typography>
        <Typography variant="body2" color="text.secondary">
          Sync uses project-owned assets only. Preset files are copied into a project before they are written to the repo.
        </Typography>
      </Stack>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
        <TextField select label="Sync operation" value={mode} onChange={(e) => setMode(e.target.value)} sx={{ minWidth: 240 }} SelectProps={{ native: true }}>
          <option value="DB_TO_REPO">Write project files to repo</option>
          <option value="REPO_TO_DB">Import repo files into project assets</option>
          <option value="COMPARE_ONLY">Compare only</option>
        </TextField>
        <Button variant="outlined" onClick={() => run(false)}>Preview</Button>
        <Button variant="contained" onClick={() => run(true)}>Write project files to repo</Button>
      </Stack>
      {items ? (
        <Stack spacing={1} sx={{ mt: 2 }}>
          {items.items.map((item) => (
            <Paper key={item.target_path} variant="outlined" className="run-row">
              <Box>
                <Typography fontWeight={700}>{item.target_path}</Typography>
                <Typography variant="body2" color="text.secondary">{item.status} / {item.action}</Typography>
                {item.diff_summary && <Typography variant="body2">{item.diff_summary}</Typography>}
                {item.message && <Typography variant="body2">{item.message}</Typography>}
              </Box>
              <Chip label={item.status} />
              <TextField select size="small" label="Action" value={actions[item.target_path] || item.action || 'noop'} onChange={(e) => setActions((a) => ({ ...a, [item.target_path]: e.target.value }))} sx={{ width: 220 }}>
                {(item.allowed_actions || [item.action || 'noop']).map((action: string) => <MenuItem key={action} value={action}>{action}</MenuItem>)}
              </TextField>
            </Paper>
          ))}
        </Stack>
      ) : null}
    </Paper>
  );
}
