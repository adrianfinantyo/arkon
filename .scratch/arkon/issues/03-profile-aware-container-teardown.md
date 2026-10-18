# [03] Profile-Aware Container Teardown Engine

Type: grilling
Status: open
Blocked by: 01

## Question

How should Arkon execute the phased teardown of Docker Compose container profiles (`apps` -> `infra` -> `*`) to guarantee clean database WAL checkpoints and buffer flushes, should it invoke the Docker CLI (`docker compose`) or speak directly to the Docker Engine Unix socket (`/var/run/docker.sock`), and how should per-stage timeouts be managed?
