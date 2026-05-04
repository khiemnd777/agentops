import { useEffect, useMemo, useState } from 'react';
import { Alert, Box, Button, Chip, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import PreviewIcon from '@mui/icons-material/Preview';
import { get, post } from '../api/client';
import type { ApplyPresetItem, ApplyPresetResponse, Asset, AssetPreset, AssetPresetDetail } from '../types';

export function ProjectComposer({ projectId }: { projectId: string }) {
  const [assets, setAssets] = useState<Asset[]>([]);
  const [error, setError] = useState('');

  const loadAssets = () => {
    get<Asset[]>(`/api/projects/${projectId}/assets`).then(setAssets).catch((e) => setError(e.message));
  };

  useEffect(() => { loadAssets(); }, [projectId]);

  return (
    <Stack spacing={2}>
      {error && <Alert severity="warning">{error}</Alert>}
      <ApplyPresetPanel projectId={projectId} onApplied={loadAssets} />
      <Stack spacing={0.5}>
        <Typography variant="h6" fontWeight={700}>Project-owned assets</Typography>
        <Typography variant="body2" color="text.secondary">
          These assets belong to this project. Presets define the shared file set they were copied from.
        </Typography>
      </Stack>
      {assets.length === 0 ? (
        <Alert severity="info">No project-owned assets yet. Apply a preset to copy missing files into this project.</Alert>
      ) : (
        <Box className="list-grid">
          {assets.map((asset) => (
            <Paper key={asset.id} variant="outlined" className="list-card">
              <Typography fontWeight={700}>{asset.name}</Typography>
              <Typography variant="body2" color="text.secondary">{asset.type}/{asset.slug}</Typography>
              {asset.description ? <Typography variant="body2" sx={{ mt: 1 }}>{asset.description}</Typography> : null}
              <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1 }}>
                <Chip size="small" label={asset.status} />
                {asset.current_version_id ? <Chip size="small" label="versioned" variant="outlined" /> : null}
              </Stack>
            </Paper>
          ))}
        </Box>
      )}
    </Stack>
  );
}

export function ApplyPresetPanel({ projectId, onApplied }: { projectId: string; onApplied?: () => void }) {
  const [presets, setPresets] = useState<AssetPreset[]>([]);
  const [presetId, setPresetId] = useState('');
  const [detail, setDetail] = useState<AssetPresetDetail | null>(null);
  const [preview, setPreview] = useState<ApplyPresetResponse | null>(null);
  const [error, setError] = useState('');
  const [applied, setApplied] = useState(false);

  useEffect(() => {
    setError('');
    get<AssetPreset[]>('/api/presets')
      .then((items) => {
        setPresets(items);
        setPresetId((current) => current || items[0]?.id || '');
      })
      .catch((e) => setError(e.message));
  }, []);

  useEffect(() => {
    setPreview(null);
    setApplied(false);
    if (!presetId) {
      setDetail(null);
      return;
    }
    get<AssetPresetDetail>(`/api/presets/${presetId}`).then(setDetail).catch((e) => setError(e.message));
  }, [presetId]);

  const runPreview = async () => {
    if (!presetId) return;
    setError('');
    setApplied(false);
    try {
      setPreview(await get<ApplyPresetResponse>(`/api/projects/${projectId}/presets/${presetId}/preview`));
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const applyPreset = async () => {
    if (!presetId) return;
    setError('');
    try {
      setPreview(await post<ApplyPresetResponse>(`/api/projects/${projectId}/presets/${presetId}/apply`));
      setApplied(true);
      onApplied?.();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const items = preview?.items ?? detail?.items.map((item) => ({
    template_asset: item.template_asset,
    template_asset_version: item.template_asset_version,
    target_path: item.target_path,
    required: item.required,
    sort_order: item.sort_order,
    status: 'not_previewed'
  })) ?? [];
  const missingCount = items.filter((item) => !isAlreadyExists(item)).length;

  return (
    <Paper variant="outlined" className="composer-profile-panel">
      <Stack spacing={0.5}>
        <Typography variant="h6" fontWeight={700}>Apply Preset</Typography>
        <Typography variant="body2" color="text.secondary">
          Preview a preset, then copy only missing template assets into this project.
        </Typography>
      </Stack>
      {error && <Alert severity="warning">{error}</Alert>}
      {applied && <Alert severity="success">Preset applied. Missing assets were copied into the project.</Alert>}
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ xs: 'stretch', sm: 'center' }}>
        <TextField
          select
          label="Preset"
          value={presetId}
          onChange={(e) => setPresetId(e.target.value)}
          sx={{ minWidth: 260 }}
        >
          {presets.map((preset) => <MenuItem key={preset.id} value={preset.id}>{preset.name}</MenuItem>)}
        </TextField>
        <Button startIcon={<PreviewIcon />} variant="outlined" onClick={runPreview} disabled={!presetId}>Preview</Button>
        <Button startIcon={<ContentCopyIcon />} variant="contained" onClick={applyPreset} disabled={!presetId || missingCount === 0}>
          Copy missing assets
        </Button>
      </Stack>
      {presetId && items.length === 0 ? <Alert severity="info">This preset has no assets.</Alert> : null}
      {items.length ? <PresetItemList items={items} /> : null}
    </Paper>
  );
}

function PresetItemList({ items }: { items: ApplyPresetItem[] }) {
  const sorted = useMemo(() => items.slice().sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0)), [items]);
  return (
    <Stack spacing={1}>
      {sorted.map((item) => {
        const asset = item.template_asset ?? item.project_asset;
        return (
          <Paper key={`${asset?.id ?? item.target_path}:${item.target_path}`} variant="outlined" className="run-row">
            <Box>
              <Typography fontWeight={700}>{asset?.name ?? item.target_path}</Typography>
              <Typography variant="body2" color="text.secondary">{item.target_path}</Typography>
              {item.message ? <Typography variant="body2">{item.message}</Typography> : null}
            </Box>
            <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" justifyContent="flex-end">
              {item.required ? <Chip size="small" label="Required" variant="outlined" /> : null}
              {item.template_asset_version?.version ? <Chip size="small" label={`v${item.template_asset_version.version}`} /> : null}
              <Chip size="small" label={presetItemLabel(item)} color={isAlreadyExists(item) ? 'default' : 'primary'} variant={isAlreadyExists(item) ? 'outlined' : 'filled'} />
            </Stack>
          </Paper>
        );
      })}
    </Stack>
  );
}

function isAlreadyExists(item: ApplyPresetItem) {
  return item.already_exists || item.status === 'already_exists' || item.action === 'already_exists' || item.action === 'noop';
}

function presetItemLabel(item: ApplyPresetItem) {
  if (isAlreadyExists(item)) return 'Already exists';
  if (item.status === 'created') return 'Copied';
  if (item.status === 'not_previewed') return 'Preview pending';
  return 'Missing';
}
