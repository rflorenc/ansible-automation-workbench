# Ansible Automation Workbench

A web tool for managing AWX and AAP 2.x environments.  
Browse resources, populate sample data, export assets and migrate between automation platforms, from a single interface.  

## Screenshots

| Connections | Operations | Object Browser |
|:-----------:|:----------:|:--------------:|
| ![Connections](doc/Connections.png) | ![Operations](doc/awx_cleanup.png) | ![Object Browser](doc/Object%20browser%20aap.png) |

## Features

- **Object Browser** — Browse any resource type (organizations, credentials, job templates, schedules, execution environments, etc.) across connected AWX and AAP instances
- **Migrate** — API-driven migration from AWX/AAP to AAP: preview with conflict detection, then import in dependency order (no Ansible dependency)
- **Populate** — Create a full set of sample objects (orgs, teams, users, credentials, projects, inventories, job templates, workflows, schedules, surveys, RBAC) for testing and demos
- **Export** — Download assets in dependency order as structured JSON files
- **Cleanup** — Remove sample or non-default objects in reverse dependency order
- **Multi-connection** — Manage multiple AWX/AAP connections with source/destination roles

## Quick Start

```bash
# compiles frontend + backend into a go executable
make build

# Run
./autoworkbench --config config.yaml
```

Open `http://localhost:8080` in your browser.

## Configuration

Create a `config.yaml` file, or use the provided as a base:

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
go run ./cmd/workbench/ --dev --config config.yaml
```

## Build Requirements

- Go 1.23+
- Node.js 20+
