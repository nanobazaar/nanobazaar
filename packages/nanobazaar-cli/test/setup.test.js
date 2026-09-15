'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

test('setup creates identity without probing or installing wallet software', t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-setup-'));
  t.after(() => fs.rmSync(dir, {recursive: true, force: true}));
  const marker = path.join(dir, 'external-command-ran');
  const bin = path.join(dir, 'bin');
  fs.mkdirSync(bin);
  for (const name of ['npm', 'pnpm', 'wallet-cli']) {
    fs.writeFileSync(path.join(bin, name), `#!/bin/sh\necho ${name} >> ${JSON.stringify(marker)}\nexit 91\n`, {mode: 0o700});
  }
  const state = path.join(dir, 'identity.json');
  const cli = path.resolve(__dirname, '../bin/nanobazaar');
  const result = spawnSync(process.execPath, [cli, 'setup', '--skip-register'], {
    encoding: 'utf8',
    env: {...process.env, PATH: bin, NBR_STATE_PATH: state, NBR_RELAY_URL: 'https://relay.example'},
  });
  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /setup complete/);
  assert.equal(fs.existsSync(marker), false);
  assert.equal(JSON.parse(fs.readFileSync(state)).relay_url, 'https://relay.example');
});
