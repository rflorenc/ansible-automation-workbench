# Ansible Automation Workbench

A single-binary web tool for managing AWX and AAP 2.x environments. Browse resources, populate sample data, export assets, and clean up — all from one interface.

## Features

- **Object Browser** — Browse any resource type (organizations, credentials, job templates, schedules, execution environments, etc.) across connected AWX and AAP instances
- **Populate** — Create a full set of sample objects (orgs, teams, users, credentials, projects, inventories, job templates, workflows, schedules, surveys, RBAC) for testing and demos
- **Export** — Download assets in dependency order as structured JSON files
- **Cleanup** — Remove sample or non-default objects in reverse dependency order
- **Multi-connection** — Manage multiple AWX/AAP connections with source/destination roles

## Quick Start

```bash
# Build (compiles frontend + backend into a single binary)
make build

# Run
./migration-tool --config migration-tool.yaml
```

Open `http://localhost:8080` in your browser.

## Configuration

Create a `migration-tool.yaml` file:

```yaml
listen: ":8080"

connections:
  - name: My AWX
    type: awx
    role: source
    scheme: http
    host: awx.example.com
    port: 80
    username: admin
    password: secret
    insecure: false

  - name: My AAP
    type: aap
    role: destination
    scheme: https
    host: aap.example.com
    port: 443
    username: admin
    password: secret
    insecure: true
```

Connections can also be created at runtime through the UI.

## Development

```bash
# Terminal 1 — frontend (Vite dev server with hot reload)
cd web && npm run dev

# Terminal 2 — backend (proxies frontend from Vite)
go run ./cmd/migration-tool/ --dev --config migration-tool.yaml
```

## Project Structure

```
cmd/migration-tool/    Entry point
internal/
  api/                 HTTP handlers and router
  config/              CLI flags and YAML config
  models/              Data types (connections, jobs, resources)
  platform/            AWX and AAP platform implementations
web/src/
  pages/               Dashboard, ObjectBrowser, Jobs
  components/          ResourceTable, ConnectionForm, LogViewer
```

## Build Requirements

- Go 1.21+
- Node.js 18+
