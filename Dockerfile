# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.23-alpine AS backend
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /autoworkbench ./cmd/workbench/

# Stage 3: Minimal runtime
FROM scratch
LABEL org.opencontainers.image.title="Ansible Automation Workbench" \
      org.opencontainers.image.description="Go single-binary with embedded React UI for managing AWX/AAP automation platforms" \
      org.opencontainers.image.source="https://github.com/rflorenc/ansible-automation-workbench"
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=backend /autoworkbench /autoworkbench
COPY config.yaml.example /config/config.yaml
EXPOSE 8080
ENTRYPOINT ["/autoworkbench", "--config", "/config/config.yaml"]
