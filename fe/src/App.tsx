import { useEffect, useMemo, useState } from 'react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import { Box, CssBaseline, Divider, List, ListItemButton, ListItemIcon, ListItemText, Typography } from '@mui/material';
import DashboardIcon from '@mui/icons-material/Dashboard';
import FolderIcon from '@mui/icons-material/Folder';
import Inventory2Icon from '@mui/icons-material/Inventory2';
import RateReviewIcon from '@mui/icons-material/RateReview';
import AccountTreeIcon from '@mui/icons-material/AccountTree';
import { DashboardPage } from './pages/DashboardPage';
import { ProjectsPage } from './pages/ProjectsPage';
import { PresetsPage } from './pages/PresetsPage';
import { ReviewsPage } from './pages/ReviewsPage';
import { WorkflowsPage } from './pages/WorkflowsPage';
import { get } from './api/client';
import type { Project } from './types';

const theme = createTheme({
  palette: {
    mode: 'light',
    primary: { main: '#1f6f78' },
    secondary: { main: '#7a4e1d' },
    background: { default: '#f7f8f5' }
  },
  shape: { borderRadius: 8 },
  typography: { fontFamily: 'Inter, system-ui, sans-serif' }
});

const workspaceTools = [
  { key: 'presets', label: 'Preset Library', icon: <Inventory2Icon fontSize="small" /> },
  { key: 'workflows', label: 'Workflow designer', icon: <AccountTreeIcon fontSize="small" /> },
  { key: 'reviews', label: 'Review Center', icon: <RateReviewIcon fontSize="small" /> }
];

export function App() {
  const [page, setPage] = useState(() => location.hash.replace('#/', '') || 'dashboard');
  const [projects, setProjects] = useState<Project[]>([]);

  useEffect(() => {
    const onHash = () => setPage(location.hash.replace('#/', '') || 'dashboard');
    window.addEventListener('hashchange', onHash);
    return () => window.removeEventListener('hashchange', onHash);
  }, []);

  useEffect(() => {
    const loadProjects = () => get<Project[] | null>('/api/projects').then((items) => {
      setProjects(Array.isArray(items) ? items : []);
    }).catch(() => setProjects([]));
    loadProjects();
    window.addEventListener('agentops:projects-changed', loadProjects);
    return () => window.removeEventListener('agentops:projects-changed', loadProjects);
  }, []);

  const route = useMemo(() => parseRoute(page), [page]);
  const selectedProjectId = route.type === 'project' ? route.projectId : '';

  const content = useMemo(() => {
    if (route.type === 'project') return <ProjectsPage projectId={route.projectId} section={route.section} />;
    if (route.type === 'playback') return <ReviewsPage taskRunId={route.taskRunId} />;
    switch (route.key) {
      case 'projects':
        return <ProjectsPage />;
      case 'presets':
        return <PresetsPage />;
      case 'workflows':
        return <WorkflowsPage />;
      case 'reviews':
        return <ReviewsPage />;
      default:
        return <DashboardPage />;
    }
  }, [route]);

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box className="app-shell">
        <Box component="aside" className="sidebar-shell">
          <Box className="sidebar-brand">
            <Typography variant="h6" sx={{ fontWeight: 800, lineHeight: 1.1 }}>
              AgentOps
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Workspace
            </Typography>
          </Box>
          <Divider />
          <List className="sidebar-nav" disablePadding>
            <ListItemButton selected={route.key === 'dashboard'} onClick={() => { location.hash = '#/dashboard'; }}>
              <ListItemIcon><DashboardIcon fontSize="small" /></ListItemIcon>
              <ListItemText primary="Workspace Overview" />
            </ListItemButton>
            <ListItemButton selected={route.key === 'projects' && route.type !== 'project'} onClick={() => { location.hash = '#/projects'; }}>
              <ListItemIcon><FolderIcon fontSize="small" /></ListItemIcon>
              <ListItemText primary="Projects" />
            </ListItemButton>
          </List>
          <List className="sidebar-nav project-list-nav" disablePadding>
            {projects.length === 0 && (
              <Box className="sidebar-empty">
                Create a project to add it here.
              </Box>
            )}
            {projects.map((p) => (
              <ListItemButton
                key={p.id}
                selected={selectedProjectId === p.id}
                onClick={() => { location.hash = `#/project/${p.id}/overview`; }}
              >
                <ListItemIcon><FolderIcon fontSize="small" /></ListItemIcon>
                <ListItemText primary={p.name} secondary={p.slug} />
              </ListItemButton>
            ))}
          </List>
          <Divider sx={{ mt: 'auto' }} />
          <Box className="sidebar-section-label">Workspace tools</Box>
          <List className="sidebar-nav workspace-tools-nav" disablePadding>
            {workspaceTools.map((tool) => (
              <ListItemButton
                key={tool.key}
                selected={route.type === 'workspace-tool' && route.key === tool.key}
                onClick={() => { location.hash = `#/${tool.key}`; }}
              >
                <ListItemIcon>{tool.icon}</ListItemIcon>
                <ListItemText primary={tool.label} />
              </ListItemButton>
            ))}
          </List>
        </Box>
        <Box className="body-shell">
          <Box component="main" className="content-shell">
            {content}
          </Box>
        </Box>
      </Box>
    </ThemeProvider>
  );
}

type Route =
  | { type: 'dashboard'; key: string }
  | { type: 'projects'; key: string }
  | { type: 'workspace-tool'; key: string }
  | { type: 'project'; key: string; projectId: string; section: string }
  | { type: 'playback'; key: string; taskRunId: string };

function parseRoute(page: string): Route {
  const parts = page.split('/').filter(Boolean);
  if (parts[0] === 'project') {
    return { type: 'project', key: 'project', projectId: parts[1] || '', section: parts[2] || 'overview' };
  }
  if (parts[0] === 'playback') return { type: 'playback', key: 'reviews', taskRunId: parts[1] || '' };
  if (parts[0] === 'projects') return { type: 'projects', key: 'projects' };
  if (['presets', 'workflows', 'reviews'].includes(parts[0])) return { type: 'workspace-tool', key: parts[0] };
  return { type: 'dashboard', key: 'dashboard' };
}
