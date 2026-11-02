# Publish queue

AnyProfile can post a video by itself at a chosen time, with the profile's own
browser: no agent session has to be open then, and nothing depends on Meta
Business Suite.

## Where it lives

- **UI**: Settings sidebar → *Scheduled posts* (`frontend/src/pages/publish-page.tsx`).
- **REST**: `GET/POST /v1/publish-tasks`, `POST /v1/publish-tasks/{id}/cancel`,
  `POST /v1/publish-tasks/{id}/retry` (same token as the local API).
- **MCP**: `publish_schedule`, `publish_list`, `publish_cancel`, `publish_retry`.
- **Storage**: table `publish_tasks` in the app database (migration 0027).

## How a task runs

1. The queue loop ticks every 20 s and claims every pending task whose time has
   come, one UPDATE with RETURNING, so a task can never run twice.
2. The app starts the profile if it is closed (DevTools on), waits for its pipe,
   and replays the task's steps: the same act steps `/v1/run` takes.
3. The steps are built per platform in `internal/publish/steps.go`
   (Instagram: Create → Post → Select from computer → OK notice → Next (crop) →
   Next (edit) → caption → Share, wait for "Shared reel"; Facebook:
   `/reels/create` → Add video → caption → Next → Post, wait for the share
   confirmation).
4. On success the task is `done` and, when the page shows it, the post URL is
   stored. On failure the failing step's screenshot goes to
   `logs/publish/<task>/step-N.jpg`, the error is kept, and the task is retried
   after 5, 15 and 60 minutes; the fourth failure marks it `failed`.
5. A task left `running` by a crash is marked failed when the app starts, so it
   is never silently half-posted.

## Rules and limits

- The file must already be in the `uploads` folder; the queue never moves or
  deletes it.
- Times are stored in UTC; the UI shows the local zone. `scheduleAt` must be in
  the future.
- Instagram and Facebook web post publicly: there is no private option and no
  native schedule on those composers, which is why this queue exists.
- YouTube is not in the queue: use Studio's own Schedule (the `youtube-upload`
  recipe).
- The queue needs the local API: the REST routes and MCP tools are served by it,
  and the runner is wired when the API starts. With the API off, due tasks stay
  pending and say so in their error (they are retried once a minute).

## Platform steps drift

The step templates use accessible names measured on 2026-11-02. When a platform
redesigns a dialog a step fails; the screenshot and the error name the step.
Update `internal/publish/steps.go`, raise `TemplateVersion`, and retry the task.
