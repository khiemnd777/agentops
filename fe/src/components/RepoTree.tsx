import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import Editor from '@monaco-editor/react';
import { marked } from 'marked';
import { Alert, Box, Button, Chip, CircularProgress, Divider, Paper, Tooltip, Typography } from '@mui/material';
import ArticleOutlinedIcon from '@mui/icons-material/ArticleOutlined';
import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import FolderOutlinedIcon from '@mui/icons-material/FolderOutlined';
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown';
import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight';
import SaveIcon from '@mui/icons-material/Save';
import { get, put } from '../api/client';

type TreeNode = { name: string; path: string; type: 'directory' | 'managed_file'; children?: TreeNode[] };
type RepoFile = { name: string; path: string; size_bytes: number; content: string; modified_at: string; can_write?: boolean };

const repoTreeMaxDepth = 8;

export function RepoTree({ projectId }: { projectId: string }) {
  const [root, setRoot] = useState<TreeNode | null>(null);
  const [selected, setSelected] = useState<TreeNode | null>(null);
  const [file, setFile] = useState<RepoFile | null>(null);
  const [fileLoading, setFileLoading] = useState(false);
  const [fileError, setFileError] = useState('');
  const [error, setError] = useState('');
  const [expandedPaths, setExpandedPaths] = useState<Set<string>>(new Set());
  const [openTooltipPath, setOpenTooltipPath] = useState<string | null>(null);
  const [pendingScrollPath, setPendingScrollPath] = useState<string | null>(null);
  const treeRowRefs = useRef(new Map<string, HTMLButtonElement>());

  useEffect(() => {
    get<{ root: TreeNode }>(`/api/projects/${projectId}/repo-tree?max_depth=${repoTreeMaxDepth}&show_managed_files=true`).then((r) => {
      setRoot(r.root);
      setExpandedPaths(collectDirectoryPaths(r.root));
    }).catch((e) => setError(e.message));
  }, [projectId]);

  useEffect(() => {
    if (!pendingScrollPath) return;
    const row = treeRowRefs.current.get(pendingScrollPath);
    if (!row) return;
    row.scrollIntoView?.({ behavior: 'smooth', block: 'center', inline: 'nearest' });
    setPendingScrollPath(null);
  }, [expandedPaths, pendingScrollPath, selected?.path]);

  const selectNode = (node: TreeNode, reveal = false) => {
    setOpenTooltipPath(null);
    setSelected(node);
    setFile(null);
    setFileError('');
    if (reveal) {
      setExpandedPaths((paths) => {
        const next = new Set(paths);
        ancestorDirectoryPaths(node).forEach((path) => next.add(path));
        return next;
      });
    }
    if (node.type !== 'managed_file') {
      setFileLoading(false);
      return;
    }
    setFileLoading(true);
    get<RepoFile>(`/api/projects/${projectId}/repo-file?path=${encodeURIComponent(node.path)}`)
      .then(setFile)
      .catch((e) => setFileError(e.message))
      .finally(() => setFileLoading(false));
  };
  const toggleNode = (node: TreeNode) => {
    if (node.type !== 'directory') return;
    setOpenTooltipPath(null);
    setExpandedPaths((paths) => {
      const next = new Set(paths);
      if (next.has(node.path)) next.delete(node.path);
      else next.add(node.path);
      return next;
    });
  };
  const openNode = (node: TreeNode) => {
    setPendingScrollPath(node.path);
    selectNode(node, true);
    if (node.type === 'directory') {
      setExpandedPaths((paths) => {
        const next = new Set(paths);
        expandDirectoryPaths(node).forEach((path) => next.add(path));
        return next;
      });
    }
  };
  const registerTreeRow = (path: string, row: HTMLButtonElement | null) => {
    if (row) treeRowRefs.current.set(path, row);
    else treeRowRefs.current.delete(path);
  };

  if (error) return <Alert severity="warning">{error}</Alert>;
  return (
    <Box className="repo-tree-workspace">
      <Paper variant="outlined" className="repo-tree-panel">
        {root ? (
          <Box className="repo-tree" role="tree" aria-label="Repository tree">
            <Tree
              node={root}
              depth={0}
              root
              last
              selectedPath={selected?.path || ''}
              expandedPaths={expandedPaths}
              openTooltipPath={openTooltipPath}
              setOpenTooltipPath={setOpenTooltipPath}
              onSelect={selectNode}
              onToggle={toggleNode}
              registerRow={registerTreeRow}
            />
          </Box>
        ) : (
          <Typography className="repo-tree-loading">Loading tree...</Typography>
        )}
      </Paper>
      <RepoFilePanel projectId={projectId} selected={selected} file={file} loading={fileLoading} error={fileError} onOpenNode={openNode} />
    </Box>
  );
}

function Tree({
  node,
  depth,
  selectedPath,
  expandedPaths,
  openTooltipPath,
  setOpenTooltipPath,
  onSelect,
  onToggle,
  registerRow,
  root = false,
  last = false
}: {
  node: TreeNode;
  depth: number;
  selectedPath: string;
  expandedPaths: Set<string>;
  openTooltipPath: string | null;
  setOpenTooltipPath: (path: string | null) => void;
  onSelect: (node: TreeNode) => void;
  onToggle: (node: TreeNode) => void;
  registerRow: (path: string, row: HTMLButtonElement | null) => void;
  root?: boolean;
  last?: boolean;
}) {
  const children = node.children || [];
  const isDirectory = node.type === 'directory';
  const expandable = isDirectory && children.length > 0;
  const expanded = expandable && expandedPaths.has(node.path);
  const selected = selectedPath === node.path;
  const tooltipOpen = openTooltipPath === node.path;
  const style = {
    '--tree-indent': `${depth * 24}px`,
    '--tree-branch-left': `${Math.max(0, (depth - 1) * 24 + 10)}px`
  } as CSSProperties;

  const closeTooltip = () => {
    setOpenTooltipPath(null);
  };

  return (
    <Box
      className={`repo-tree-item ${isDirectory ? 'directory' : 'managed'} ${root ? 'root' : ''} ${last ? 'last' : ''}`}
      data-testid="tree-node"
      role="treeitem"
      aria-expanded={expandable ? expanded : undefined}
      style={style}
    >
      <Box
        component="button"
        type="button"
        ref={(row: HTMLButtonElement | null) => registerRow(node.path, row)}
        className={`repo-tree-row ${selected ? 'selected' : ''}`}
        aria-expanded={expandable ? expanded : undefined}
        onBlur={closeTooltip}
        onClick={() => {
          closeTooltip();
          onSelect(node);
        }}
        onMouseLeave={closeTooltip}
      >
        <Box
          component="span"
          className="repo-tree-toggle"
          aria-hidden
          onClick={(event) => {
            event.stopPropagation();
            closeTooltip();
            if (expandable) onToggle(node);
          }}
        >
          {expandable ? (expanded ? <KeyboardArrowDownIcon fontSize="small" /> : <KeyboardArrowRightIcon fontSize="small" />) : null}
        </Box>
        <Box className="repo-tree-icon" aria-hidden>
          {isDirectory ? <FolderOutlinedIcon fontSize="small" /> : fileIcon(node.name)}
        </Box>
        <Tooltip
          title={node.path === '.' ? node.name : node.path}
          enterDelay={500}
          leaveDelay={0}
          open={tooltipOpen}
          onOpen={() => setOpenTooltipPath(node.path)}
          onClose={closeTooltip}
          disableFocusListener
          disableInteractive
          disableTouchListener
        >
          <Typography component="span" className="repo-tree-name">
            {node.name}
          </Typography>
        </Tooltip>
        {isDirectory && children.length > 0 && (
          <Chip size="small" label={children.length} className="repo-tree-count" />
        )}
      </Box>
      {expanded && children.length > 0 && (
        <Box className="repo-tree-children" role="group">
          {children.map((child, index) => (
            <Tree
              key={child.path}
              node={child}
              depth={depth + 1}
              selectedPath={selectedPath}
              expandedPaths={expandedPaths}
              openTooltipPath={openTooltipPath}
              setOpenTooltipPath={setOpenTooltipPath}
              onSelect={onSelect}
              onToggle={onToggle}
              registerRow={registerRow}
              last={index === children.length - 1}
            />
          ))}
        </Box>
      )}
    </Box>
  );
}

function collectDirectoryPaths(root: TreeNode) {
  const paths = new Set<string>();
  const visit = (node: TreeNode) => {
    if (node.type !== 'directory') return;
    paths.add(node.path);
    node.children?.forEach(visit);
  };
  visit(root);
  return paths;
}

function ancestorDirectoryPaths(node: TreeNode) {
  if (node.path === '.') return ['.'];
  const parts = node.path.split('/').filter(Boolean);
  const directoryPartCount = node.type === 'directory' ? parts.length - 1 : parts.length - 1;
  const paths = ['.'];
  for (let index = 1; index <= directoryPartCount; index += 1) {
    paths.push(parts.slice(0, index).join('/'));
  }
  return paths;
}

function expandDirectoryPaths(node: TreeNode) {
  if (node.type !== 'directory') return ancestorDirectoryPaths(node);
  if (node.path === '.') return ['.'];
  const paths = ancestorDirectoryPaths(node);
  paths.push(node.path);
  return paths;
}

function RepoFilePanel({
  projectId,
  selected,
  file,
  loading,
  error,
  onOpenNode
}: {
  projectId: string;
  selected: TreeNode | null;
  file: RepoFile | null;
  loading: boolean;
  error: string;
  onOpenNode: (node: TreeNode) => void;
}) {
  const [content, setContent] = useState('');
  const [savedFile, setSavedFile] = useState<RepoFile | null>(null);
  const [saveError, setSaveError] = useState('');
  const [saving, setSaving] = useState(false);
  const currentFile = savedFile || file;
  const canWrite = Boolean(currentFile?.can_write);
  const markdown = isMarkdown(currentFile?.name || selected?.name || '');
  const dirty = Boolean(currentFile && content !== currentFile.content);
  const preview = useMemo(() => ({ __html: marked.parse(content || '') as string }), [content]);

  useEffect(() => {
    setContent(file?.content || '');
    setSavedFile(null);
    setSaveError('');
  }, [file?.path, file?.modified_at]);

  const save = async () => {
    if (!currentFile) return;
    setSaving(true);
    setSaveError('');
    try {
      const updated = await put<RepoFile>(`/api/projects/${projectId}/repo-file`, { path: currentFile.path, content });
      setSavedFile(updated);
      setContent(updated.content);
    } catch (e) {
      setSaveError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Paper variant="outlined" className="repo-file-panel">
      {!selected && (
        <Box className="repo-file-empty">
          <Typography variant="subtitle1" fontWeight={800}>No file selected</Typography>
          <Typography variant="body2" color="text.secondary">Select a managed file in the repository tree to preview it.</Typography>
        </Box>
      )}
      {selected && selected.type === 'directory' && (
        <Box className="repo-folder-preview">
          <Box>
            <Typography variant="subtitle1" fontWeight={800}>{selected.name}</Typography>
            <Typography variant="body2" color="text.secondary">{selected.path}</Typography>
          </Box>
          <Divider />
          <Box className="repo-folder-grid">
            {(selected.children || []).length === 0 && (
              <Typography variant="body2" color="text.secondary">No items in this folder.</Typography>
            )}
            {(selected.children || []).map((child) => (
              <Box
                key={child.path}
                component="button"
                type="button"
                className={`repo-folder-thumb ${child.type === 'directory' ? 'directory' : 'file'}`}
                onClick={() => onOpenNode(child)}
              >
                <Box className="repo-folder-thumb-icon" aria-hidden>
                  {child.type === 'directory' ? <FolderOutlinedIcon fontSize="large" /> : fileIcon(child.name, 'large')}
                  {child.type === 'directory' && directItemCount(child) > 0 && (
                    <Box component="span" className="repo-folder-thumb-count" data-testid={`folder-thumb-count-${child.path}`}>
                      {directItemCount(child)}
                    </Box>
                  )}
                </Box>
                <Box className="repo-folder-thumb-text">
                  <Tooltip title={child.name} enterDelay={500} leaveDelay={0} disableInteractive>
                    <Typography className="repo-folder-thumb-name">{child.name}</Typography>
                  </Tooltip>
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      )}
      {selected && selected.type === 'managed_file' && (
        <Box className="repo-file-preview">
          <Box>
            <Typography variant="subtitle1" fontWeight={800}>{selected.name}</Typography>
            <Typography variant="body2" color="text.secondary">{selected.path}</Typography>
          </Box>
          <Divider />
          {loading && (
            <Box className="repo-file-loading">
              <CircularProgress size={18} />
              <Typography variant="body2" color="text.secondary">Loading preview...</Typography>
            </Box>
          )}
          {error && <Alert severity="warning">{error}</Alert>}
          {currentFile && (
            <>
              <Box className="repo-file-meta">
                <Chip size="small" label={formatBytes(currentFile.size_bytes)} />
                <Chip size="small" label={formatDate(currentFile.modified_at)} variant="outlined" />
                {markdown && <Chip size="small" label="Markdown preview" color="primary" variant="outlined" />}
                {dirty && canWrite && <Chip size="small" label="Unsaved" color="warning" />}
                {!canWrite && <Chip size="small" label="Read only" variant="outlined" />}
                {canWrite && (
                  <Button startIcon={<SaveIcon />} variant="contained" size="small" onClick={save} disabled={!dirty || saving} sx={{ ml: 'auto' }}>
                    {saving ? 'Saving...' : 'Save'}
                  </Button>
                )}
              </Box>
              {saveError && <Alert severity="warning">{saveError}</Alert>}
              <Box className={markdown ? 'repo-file-editor-grid markdown' : 'repo-file-editor-grid'}>
                <Paper variant="outlined" className="repo-file-editor-box">
                  <Editor
                    height="560px"
                    language={editorLanguage(currentFile.name)}
                    value={content}
                    onChange={(value) => {
                      if (canWrite) setContent(value || '');
                    }}
                    options={{ minimap: { enabled: false }, wordWrap: 'on', fontSize: 13, scrollBeyondLastLine: false, readOnly: !canWrite }}
                  />
                </Paper>
                {markdown && (
                  <Paper variant="outlined" className="repo-file-markdown-preview">
                    <div dangerouslySetInnerHTML={preview} />
                  </Paper>
                )}
              </Box>
            </>
          )}
        </Box>
      )}
    </Paper>
  );
}

function fileIcon(name: string, fontSize: 'small' | 'large' = 'small') {
  if (/\.(md|mdx|txt)$/i.test(name)) return <ArticleOutlinedIcon fontSize={fontSize} />;
  return <DescriptionOutlinedIcon fontSize={fontSize} />;
}

function directItemCount(node: TreeNode) {
  return (node.children || []).length;
}

function isMarkdown(name: string) {
  return /\.(md|mdx|markdown)$/i.test(name);
}

function editorLanguage(name: string) {
  if (isMarkdown(name)) return 'markdown';
  if (/\.(ya?ml)$/i.test(name)) return 'yaml';
  if (/\.json$/i.test(name)) return 'json';
  return 'plaintext';
}

function formatBytes(value: number) {
  if (!value) return '0 B';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatDate(value: string) {
  if (!value) return 'Unknown modified time';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'Unknown modified time';
  return date.toLocaleString();
}
