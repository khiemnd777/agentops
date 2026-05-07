# Repo-Local Codex Kit

This directory contains Codex project configuration, custom agents, supporting guidance, policies, workflows, and review checklists for coding agents working on this AgentOps Workspace repository.

These files are not AgentOps product assets. Do not sync them into mounted project repositories and do not treat them as database seed content.

## Layout

- `config.toml`: project-scoped Codex settings.
- `agents/`: project-scoped custom agent definitions.
- `skills/`: repo-local Codex skill source for this workspace.
- `docs/`: runtime, stack, commands, container, config, and module inventory.
- `policies/`: non-negotiable repo constraints.
- `workflows/`: standard coding workflow for this repo.
- `checklists/`: final review checklist before handoff.

## Core Rule

Keep the implementation aligned with the MVP boundary: manage assets, sync Codex-native project files, import reports, audit runs, and render playback. Do not add direct agent execution inside the web app/API.

For the remake direction, see `docs/agent-first-codex-remake.md`.
