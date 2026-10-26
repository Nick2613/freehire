# Free Render hosting with Neon

This deployment puts the Go API, SvelteKit frontend, Meilisearch, Redis and nginx
inside one Render Free web service. PostgreSQL lives in Neon. It is intended for
the small local catalog imported for this installation.

**Status on 26 October 2026:** the local ARM64 image builds and passes job,
company, source and health checks under a 512 MiB memory limit. Ten parallel
search requests passed without an OOM termination. The official Blueprint schema,
ShellCheck, Dockerfile lint, Go vet, all Go unit tests and integration-tagged Go vet
also pass. A private backup contains 1,781 jobs and 91 source entries (about 31 MB
of database storage).

The GitHub hosting fork/branch and the user's free Neon PostgreSQL 18 project exist.
Database migration and public Render deployment are still pending. Production
AMD64 behavior must be verified on Render; the local memory test is ARM64.

## Storage and restarts

- Neon stores the jobs, companies, sources and account data.
- Meilisearch uses temporary disk. Startup rebuilds its job and company indexes
  sequentially from PostgreSQL. Node starts after the API is ready to reduce peak
  memory during indexing.
- Redis contains temporary rate-limit counters. It binds to loopback and has a
  16 MB data limit. Counters reset after the container is replaced.
- Render supplies the public HTTPS URL. Only nginx's public port is routed to
  visitors. JWT and search keys are generated as private service variables.
- The Dockerfile-specific ignore file excludes local secrets, caches and backups
  from the build context. Do not upload the local `.env` or database backup to Git.

Render Free sleeps after 15 minutes without visitors and has monthly quotas.
Neon Free also has storage and compute quotas. These services do not guarantee
continuous uptime or perpetual service. No keep-awake traffic is configured.

## Migration order

1. Finish Render and Neon sign-in using their free plans.
2. Start Docker Desktop and export the existing local database from the checkout:

   ```sh
   ./hosting/render/export-local.sh
   ```

   The private dump stays in `.cache/cloud-export/`. The script reports database
   size and job/source counts. Confirm the database fits Neon's current free quota.

3. Build and test the image locally before provisioning the web service:

   ```sh
   docker build -f hosting/render/Dockerfile -t freehire-render .
   ```

   Test cold startup, job search, companies, and a container replacement under a
   512 MB memory limit. The memory settings are tuning knobs, not proof that the
   application fits. Do not upgrade to a paid plan if the free test fails without
   the user's approval.

4. Create an empty Neon PostgreSQL **18** database. The original database and dump
   use PostgreSQL 18. Set `NEON_DATABASE_URL` to its direct SSL connection string
   privately in the terminal, then restore:

   ```sh
   ./hosting/render/restore-neon.sh /absolute/path/to/freehire-backup.dump
   ```

   Restore refuses a target with public tables or views. It uses a single
   transaction and does not delete existing data. Compare job/source counts with
   the export before deploying the application.

5. Place the current source and deployment files in a Git repository accessible
   to Render. Include the existing F1 adapter changes. Keep secrets and database
   files out of that repository.
6. Create the service using the root [render.yaml](../../render.yaml). It declares
   exactly one web service with `plan: free`, no paid disks, and no Render database.
   Set `DATABASE_URL` to Neon's SSL connection string. Render generates the JWT and
   search keys. The application derives its HTTPS origin from `RENDER_EXTERNAL_URL`.
7. Verify `/health`, `/jobs`, `/companies`, `/sources` and a Data Engineer search
   on the public URL. Verify job counts after a fresh container starts and confirm
   the service is still on the free plan.

Database migrations run at application startup. Restore the existing database
**before** first starting the service; a schema initialized by the app is no
longer an empty restore target.

## Features requiring separate configuration

The job catalog and search are the target of this deployment. Email sending,
OAuth sign-in, AI features and persistent resume uploads need their own provider
credentials/storage and are not provisioned by this Blueprint. No recurring source
refresh has been configured. The existing local refresh tools remain available.

## Official provider limits

- [Render Free](https://render.com/docs/free)
- [Render Blueprint specification](https://render.com/docs/blueprint-spec)
- [Neon Free pricing and quotas](https://neon.com/pricing)
