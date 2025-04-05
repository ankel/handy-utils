# Docker restart

This command listen to docker socket and restart containers that are not running.

## Why?

While `docker compose` can restart containers, it requires containers to have been running for over 10s before the container can be restarted. If that's not the case, the container will stay down. This util will help restart containers even if they have not run for 10s.
