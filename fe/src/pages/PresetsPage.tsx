import { useEffect, useMemo, useState } from 'react';
import Editor from '@monaco-editor/react';
import { marked } from 'marked';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/Delete';
import SaveIcon from '@mui/icons-material/Save';
import { del, get, post, put } from '../api/client';
import type { Asset, AssetPreset, AssetPresetDetail, AssetPresetItem, AssetVersion } from '../types';

type PresetForm = {
  slug: string;
  name: string;
  description: string;
  status: string;
  current_version: number;
};

type AssetWriteResponse = {
  asset: Asset;
  version: AssetVersion;
};

const emptyPresetForm: PresetForm = {
  slug: '',
  name: '',
  description: '',
  status: 'active',
  current_version: 1
};

const assetTypes = ['agents_md', 'readme', 'skill_doc', 'subagent_doc', 'policy_doc', 'workflow_doc', 'prompt_template', 'checklist', 'run_report_contract'];

export function PresetsPage() {
  const [presets, setPresets] = useState<AssetPreset[]>([]);
  const [selected, setSelected] = useState<AssetPreset | null>(null);
  const [detail, setDetail] = useState<AssetPresetDetail | null>(null);
  const [templates, setTemplates] = useState<Asset[]>([]);
  const [createForm, setCreateForm] = useState<PresetForm>(emptyPresetForm);
  const [createOpen, setCreateOpen] = useState(false);
  const [editForm, setEditForm] = useState<PresetForm>(emptyPresetForm);
  const [error, setError] = useState('');

  const loadPresets = () => get<AssetPreset[]>('/api/presets')
    .then((items) => {
      setPresets(items);
      setSelected((current) => {
        if (current) return items.find((item) => item.id === current.id) || items[0] || null;
        return items[0] || null;
      });
    })
    .catch((e) => setError(e.message));

  const loadTemplates = () => get<Asset[]>('/api/assets?scope=template')
    .then(setTemplates)
    .catch((e) => setError(e.message));

  useEffect(() => {
    loadPresets();
    loadTemplates();
  }, []);

  useEffect(() => {
    if (!selected) {
      setDetail(null);
      setEditForm(emptyPresetForm);
      return;
    }
    get<AssetPresetDetail>(`/api/presets/${selected.id}`)
      .then((next) => {
        setDetail(next);
        setEditForm(toPresetForm(next));
      })
      .catch((e) => setError(e.message));
  }, [selected]);

  const createPreset = async () => {
    setError('');
    try {
      const created = await post<AssetPresetDetail>('/api/presets', createForm);
      setCreateForm(emptyPresetForm);
      setCreateOpen(false);
      await loadPresets();
      setSelected(created);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const savePreset = async () => {
    if (!detail) return;
    setError('');
    try {
      const saved = await put<AssetPresetDetail>(`/api/presets/${detail.id}`, editForm);
      await loadPresets();
      setSelected(saved);
      setDetail(saved);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const refreshDetail = (next: AssetPresetDetail) => {
    setDetail(next);
    setEditForm(toPresetForm(next));
    loadPresets();
    loadTemplates();
  };

  return (
    <Box className="split-layout">
      <Stack spacing={2} className="asset-list">
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="h4" fontWeight={700}>Preset Library</Typography>
          <Tooltip title="Create preset">
            <IconButton color="primary" onClick={() => setCreateOpen(true)} aria-label="Create preset">
              <AddIcon />
            </IconButton>
          </Tooltip>
        </Stack>
        {error && <Alert severity="warning">{error}</Alert>}
        {presets.length === 0 ? <Alert severity="info">No presets available.</Alert> : null}
        {presets.map((preset) => (
          <Paper key={preset.id} variant="outlined" className="asset-card" onClick={() => setSelected(preset)}>
            <Typography fontWeight={700}>{preset.name}</Typography>
            <Typography variant="body2" color="text.secondary">{preset.description}</Typography>
            <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1 }}>
              <Chip size="small" label={preset.slug} variant="outlined" />
              <Chip size="small" label={`v${preset.current_version || 1}`} variant="outlined" />
              <Chip size="small" label={`${preset.item_count || 0} files`} />
              <Chip size="small" label={preset.status} />
            </Stack>
          </Paper>
        ))}
      </Stack>
      <Box className="editor-pane">
        {detail ? (
          <Stack spacing={2}>
            <PresetEditPanel form={editForm} onChange={setEditForm} onSave={savePreset} />
            <PresetFileManager detail={detail} templates={templates} onChanged={refreshDetail} />
          </Stack>
        ) : (
          <Alert severity="info">Select a preset to inspect or edit its files.</Alert>
        )}
      </Box>
      <CreatePresetDialog
        open={createOpen}
        form={createForm}
        onChange={setCreateForm}
        onClose={() => setCreateOpen(false)}
        onCreate={createPreset}
      />
    </Box>
  );
}

function CreatePresetDialog({ open, form, onChange, onClose, onCreate }: { open: boolean; form: PresetForm; onChange: (form: PresetForm) => void; onClose: () => void; onCreate: () => void }) {
  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Create preset</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <TextField label="Name" value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value, slug: form.slug || slugify(e.target.value) })} />
          <TextField label="Slug" value={form.slug} onChange={(e) => onChange({ ...form, slug: e.target.value })} />
          <TextField multiline minRows={3} label="Description" value={form.description} onChange={(e) => onChange({ ...form, description: e.target.value })} />
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button startIcon={<AddIcon />} variant="contained" onClick={onCreate} disabled={!form.name || !form.slug}>Create</Button>
      </DialogActions>
    </Dialog>
  );
}

function PresetEditPanel({ form, onChange, onSave }: { form: PresetForm; onChange: (form: PresetForm) => void; onSave: () => void }) {
  return (
    <Paper variant="outlined" className="panel">
      <Stack spacing={2}>
        <Box className="page-header">
          <Box>
            <Typography variant="h5" fontWeight={700}>{form.name}</Typography>
            <Typography color="text.secondary">{form.slug}</Typography>
          </Box>
          <Button startIcon={<SaveIcon />} variant="contained" onClick={onSave}>Save preset</Button>
        </Box>
        <Box className="form-grid">
          <TextField label="Name" value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} />
          <TextField label="Slug" value={form.slug} onChange={(e) => onChange({ ...form, slug: e.target.value })} />
          <TextField select label="Status" value={form.status} onChange={(e) => onChange({ ...form, status: e.target.value })}>
            <MenuItem value="active">active</MenuItem>
            <MenuItem value="draft">draft</MenuItem>
            <MenuItem value="archived">archived</MenuItem>
          </TextField>
          <TextField type="number" label="Preset version" value={form.current_version} onChange={(e) => onChange({ ...form, current_version: Number(e.target.value) || 1 })} />
        </Box>
        <TextField multiline minRows={2} label="Description" value={form.description} onChange={(e) => onChange({ ...form, description: e.target.value })} />
      </Stack>
    </Paper>
  );
}

function PresetFileManager({ detail, templates, onChanged }: { detail: AssetPresetDetail; templates: Asset[]; onChanged: (detail: AssetPresetDetail) => void }) {
  const [createFileOpen, setCreateFileOpen] = useState(false);
  const [editItemID, setEditItemID] = useState('');
  const [error, setError] = useState('');
  const editingItem = detail.items.find((item) => item.id === editItemID) || null;

  const removeItem = async (itemID?: string) => {
    if (!itemID) return;
    setError('');
    try {
      onChanged(await del<AssetPresetDetail>(`/api/presets/${detail.id}/items/${itemID}`));
      setEditItemID('');
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <Stack spacing={2}>
      <Paper variant="outlined" className="panel">
        <Stack spacing={2}>
          <Box className="page-header">
            <Box>
              <Typography variant="h6" fontWeight={700}>Preset files</Typography>
              <Typography color="text.secondary">{detail.items.length} pinned files</Typography>
            </Box>
            <Stack direction="row" spacing={1} alignItems="center">
              <Chip label={`v${detail.current_version || 1}`} color="primary" variant="outlined" />
              <Tooltip title="Create preset file">
                <IconButton color="primary" onClick={() => setCreateFileOpen(true)} aria-label="Create preset file">
                  <AddIcon />
                </IconButton>
              </Tooltip>
            </Stack>
          </Box>
          {error && <Alert severity="warning">{error}</Alert>}
          <AddExistingFileForm detail={detail} templates={templates} onChanged={onChanged} />
        </Stack>
      </Paper>
      <Box className="list-grid">
        {detail.items
          .slice()
          .sort((a, b) => a.sort_order - b.sort_order)
          .map((item) => (
            <Paper key={`${item.id}:${item.target_path}`} variant="outlined" className="list-card" onClick={() => setEditItemID(item.id || '')}>
              <Typography fontWeight={700}>{item.template_asset.name}</Typography>
              <Typography variant="body2" color="text.secondary">{item.template_asset.type}/{item.template_asset.slug}</Typography>
              <Typography variant="caption" color="text.secondary">{item.target_path}</Typography>
              <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1 }}>
                <Chip size="small" label={`v${item.template_asset_version.version}`} />
                <Chip size="small" label={item.required ? 'Required' : 'Optional'} variant="outlined" />
                <Chip size="small" label={`Order ${item.sort_order}`} variant="outlined" />
              </Stack>
            </Paper>
          ))}
      </Box>
      {detail.items.length === 0 ? <Alert severity="info">Create or add a file to this preset.</Alert> : null}
      <CreatePresetFileDialog
        open={createFileOpen}
        detail={detail}
        onChanged={(next) => {
          setCreateFileOpen(false);
          onChanged(next);
        }}
        onClose={() => setCreateFileOpen(false)}
      />
      {editingItem ? (
        <PresetFileEditorDialog
          open={Boolean(editingItem)}
          detail={detail}
          item={editingItem}
          onChanged={(next) => {
            setEditItemID('');
            onChanged(next);
          }}
          onRemove={() => removeItem(editingItem.id)}
          onClose={() => setEditItemID('')}
        />
      ) : null}
    </Stack>
  );
}

function CreatePresetFileDialog({ open, detail, onChanged, onClose }: { open: boolean; detail: AssetPresetDetail; onChanged: (detail: AssetPresetDetail) => void; onClose: () => void }) {
  const [form, setForm] = useState({
    type: 'skill_doc',
    name: '',
    slug: '',
    version: '1.0.0',
    content_format: 'markdown',
    content: '# New File\n',
    target_path: '',
    required: true,
    sort_order: detail.items.length + 1
  });
  const [error, setError] = useState('');

  useEffect(() => {
    if (open) {
      setForm({
        type: 'skill_doc',
        name: '',
        slug: '',
        version: '1.0.0',
        content_format: 'markdown',
        content: '# New File\n',
        target_path: '',
        required: true,
        sort_order: detail.items.length + 1
      });
      setError('');
    }
  }, [open, detail.items.length]);

  const create = async () => {
    setError('');
    try {
      const saved = await post<AssetWriteResponse>('/api/assets', { ...form, status: 'published' });
      const next = await post<AssetPresetDetail>(`/api/presets/${detail.id}/items`, {
        template_asset_id: saved.asset.id,
        template_asset_version_id: saved.version.id,
        target_path: form.target_path || targetPathForAsset(saved.asset),
        required: form.required,
        sort_order: form.sort_order
      });
      onChanged(next);
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const preview = useMemo(() => ({ __html: marked.parse(form.content || '') as string }), [form.content]);
  const language = form.content_format === 'yaml' ? 'yaml' : 'markdown';

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xl">
      <DialogTitle>Create preset file</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          {error && <Alert severity="warning">{error}</Alert>}
          <Box className="form-grid">
            <TextField select label="Type" value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value, content_format: e.target.value === 'workflow_doc' ? 'yaml' : 'markdown', target_path: targetPathForType(e.target.value, form.slug) })}>
              {assetTypes.map((type) => <MenuItem key={type} value={type}>{type}</MenuItem>)}
            </TextField>
            <TextField label="Name" value={form.name} onChange={(e) => {
              const slug = form.slug || slugify(e.target.value);
              setForm({ ...form, name: e.target.value, slug, target_path: form.target_path || targetPathForType(form.type, slug) });
            }} />
            <TextField label="Slug" value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value, target_path: targetPathForType(form.type, e.target.value) })} />
            <TextField label="Version" value={form.version} onChange={(e) => setForm({ ...form, version: e.target.value })} />
            <TextField label="Target path" value={form.target_path} onChange={(e) => setForm({ ...form, target_path: e.target.value })} />
            <TextField type="number" label="Sort order" value={form.sort_order} onChange={(e) => setForm({ ...form, sort_order: Number(e.target.value) || 1 })} />
          </Box>
          <FormControlLabel control={<Checkbox checked={form.required} onChange={(e) => setForm({ ...form, required: e.target.checked })} />} label="Required" />
          <Box className="editor-grid">
            <Paper variant="outlined" className="editor-box">
              <Editor height="520px" language={language} value={form.content} onChange={(value) => setForm({ ...form, content: value || '' })} options={{ minimap: { enabled: false }, wordWrap: 'on' }} />
            </Paper>
            <Paper variant="outlined" className="preview-box">
              <div dangerouslySetInnerHTML={preview} />
            </Paper>
          </Box>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button startIcon={<AddIcon />} variant="contained" onClick={create} disabled={!form.name || !form.slug || !form.version}>Create and pin</Button>
      </DialogActions>
    </Dialog>
  );
}

function AddExistingFileForm({ detail, templates, onChanged }: { detail: AssetPresetDetail; templates: Asset[]; onChanged: (detail: AssetPresetDetail) => void }) {
  const [assetID, setAssetID] = useState('');
  const [versions, setVersions] = useState<AssetVersion[]>([]);
  const [versionID, setVersionID] = useState('');
  const [targetPath, setTargetPath] = useState('');
  const [required, setRequired] = useState(true);
  const [sortOrder, setSortOrder] = useState(detail.items.length + 1);
  const [error, setError] = useState('');
  const selectedAsset = useMemo(() => templates.find((asset) => asset.id === assetID), [assetID, templates]);

  useEffect(() => {
    setSortOrder(detail.items.length + 1);
  }, [detail.items.length]);

  useEffect(() => {
    if (!assetID) {
      setVersions([]);
      setVersionID('');
      setTargetPath('');
      return;
    }
    get<AssetVersion[]>(`/api/assets/${assetID}/versions`)
      .then((items) => {
        setVersions(items);
        setVersionID(items[0]?.id || '');
      })
      .catch((e) => setError(e.message));
    if (selectedAsset) setTargetPath(targetPathForAsset(selectedAsset));
  }, [assetID, selectedAsset]);

  const add = async () => {
    setError('');
    try {
      onChanged(await post<AssetPresetDetail>(`/api/presets/${detail.id}/items`, {
        template_asset_id: assetID,
        template_asset_version_id: versionID,
        target_path: targetPath,
        required,
        sort_order: sortOrder
      }));
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <Paper variant="outlined" className="mini-form">
      <Typography variant="subtitle2" fontWeight={800}>Add existing preset file</Typography>
      {error && <Alert severity="warning">{error}</Alert>}
      <Box className="form-grid">
        <TextField select label="Existing file" value={assetID} onChange={(e) => setAssetID(e.target.value)}>
          {templates.map((asset) => <MenuItem key={asset.id} value={asset.id}>{asset.type}/{asset.slug}</MenuItem>)}
        </TextField>
        <TextField select label="Pinned version" value={versionID} onChange={(e) => setVersionID(e.target.value)} disabled={!versions.length}>
          {versions.map((version) => <MenuItem key={version.id} value={version.id}>{version.version} {version.status}</MenuItem>)}
        </TextField>
        <TextField label="Target path" value={targetPath} onChange={(e) => setTargetPath(e.target.value)} />
        <TextField type="number" label="Sort order" value={sortOrder} onChange={(e) => setSortOrder(Number(e.target.value) || 1)} />
      </Box>
      <Stack direction="row" spacing={1} alignItems="center">
        <FormControlLabel control={<Checkbox checked={required} onChange={(e) => setRequired(e.target.checked)} />} label="Required" />
        <Button startIcon={<AddIcon />} variant="outlined" onClick={add} disabled={!assetID || !versionID}>Add to preset</Button>
      </Stack>
    </Paper>
  );
}

function PresetFileEditorDialog({ open, detail, item, onChanged, onRemove, onClose }: { open: boolean; detail: AssetPresetDetail; item: AssetPresetItem; onChanged: (detail: AssetPresetDetail) => void; onRemove: () => void; onClose: () => void }) {
  const [content, setContent] = useState(item.template_asset_version.content || '');
  const [version, setVersion] = useState(nextDraftVersion(item.template_asset_version.version));
  const [required, setRequired] = useState(item.required);
  const [targetPath, setTargetPath] = useState(item.target_path);
  const [sortOrder, setSortOrder] = useState(item.sort_order);
  const [error, setError] = useState('');

  useEffect(() => {
    setContent(item.template_asset_version.content || '');
    setVersion(nextDraftVersion(item.template_asset_version.version));
    setRequired(item.required);
    setTargetPath(item.target_path);
    setSortOrder(item.sort_order);
  }, [item.id, item.template_asset_version.id]);

  const saveAndPin = async (status: 'draft' | 'published') => {
    setError('');
    try {
      const saved = await post<AssetWriteResponse>('/api/assets', {
        type: item.template_asset.type,
        slug: item.template_asset.slug,
        name: item.template_asset.name,
        description: item.template_asset.description,
        version,
        content,
        content_format: item.template_asset_version.content_format || 'markdown',
        status
      });
      onChanged(await post<AssetPresetDetail>(`/api/presets/${detail.id}/items`, {
        template_asset_id: saved.asset.id,
        template_asset_version_id: saved.version.id,
        target_path: targetPath,
        required,
        sort_order: sortOrder
      }));
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const preview = useMemo(() => ({ __html: marked.parse(content || '') as string }), [content]);
  const language = item.template_asset_version.content_format === 'yaml' ? 'yaml' : 'markdown';

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="xl">
      <DialogTitle>{item.template_asset.name}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{ pt: 1 }}>
          <Typography color="text.secondary">{item.template_asset.type}/{item.template_asset.slug}</Typography>
          {error && <Alert severity="warning">{error}</Alert>}
          <Box className="form-grid">
            <TextField label="New version" value={version} onChange={(e) => setVersion(e.target.value)} />
            <TextField label="Target path" value={targetPath} onChange={(e) => setTargetPath(e.target.value)} />
            <TextField type="number" label="Sort order" value={sortOrder} onChange={(e) => setSortOrder(Number(e.target.value) || 1)} />
            <FormControlLabel control={<Checkbox checked={required} onChange={(e) => setRequired(e.target.checked)} />} label="Required" />
          </Box>
          <Box className="editor-grid">
            <Paper variant="outlined" className="editor-box">
              <Editor height="520px" language={language} value={content} onChange={(value) => setContent(value || '')} options={{ minimap: { enabled: false }, wordWrap: 'on' }} />
            </Paper>
            <Paper variant="outlined" className="preview-box">
              <div dangerouslySetInnerHTML={preview} />
            </Paper>
          </Box>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button startIcon={<DeleteIcon />} color="error" onClick={onRemove}>Remove</Button>
        <Box sx={{ flex: 1 }} />
        <Button onClick={onClose}>Close</Button>
        <Button startIcon={<SaveIcon />} variant="outlined" onClick={() => saveAndPin('draft')}>Save draft and pin</Button>
        <Button startIcon={<SaveIcon />} variant="contained" onClick={() => saveAndPin('published')}>Publish and pin</Button>
      </DialogActions>
    </Dialog>
  );
}

function toPresetForm(preset: AssetPresetDetail): PresetForm {
  return {
    slug: preset.slug,
    name: preset.name,
    description: preset.description,
    status: preset.status || 'active',
    current_version: preset.current_version || 1
  };
}

function slugify(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

function targetPathForAsset(asset: Asset) {
  return targetPathForType(asset.type, asset.slug);
}

function targetPathForType(type: string, slug: string) {
  switch (type) {
    case 'agents_md':
      return 'AGENTS.md';
    case 'readme':
      return 'README.md';
    case 'skill_doc':
      return `docs/agentic/skills/${slug}.md`;
    case 'subagent_doc':
      return `docs/agentic/subagents/${slug}.md`;
    case 'policy_doc':
      return `docs/agentic/policies/${slug}.md`;
    case 'workflow_doc':
      return `docs/agentic/workflows/${slug}.workflow.yaml`;
    case 'prompt_template':
      return `docs/agentic/prompts/${slug}.md`;
    case 'checklist':
      return `docs/agentic/checklists/${slug}.md`;
    case 'run_report_contract':
      return 'docs/agentic/contracts/run-report-contract.md';
    default:
      return '';
  }
}

function nextDraftVersion(current: string) {
  if (!current) return 'draft-1';
  return `${current}-draft`;
}
