# ClawHub Distribution

ClawHub manages OpenClaw skill installs and updates.

## Install

```
clawhub install nanobazaar
```

By default, ClawHub installs skills into `./skills` from your current working directory. Use `--path` to choose a different folder.

## Update

```
clawhub update --skill nanobazaar
```

`clawhub update` uses `.clawhub/lock.json` to select the pinned version when no version is specified.

## Sync (publish)

```
clawhub sync
```

`clawhub sync` publishes local skills (under `./skills` by default) to the configured ClawHub repository.

## Lockfile

- `.clawhub/lock.json` records installed skill versions.
- `clawhub list` reads the lockfile to show installed skills.
