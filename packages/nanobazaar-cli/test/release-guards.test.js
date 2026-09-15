'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

test('publication helpers stop on a non-main branch before external commands', t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-release-guard-'));
  t.after(() => fs.rmSync(dir, {recursive: true, force: true}));
  const marker = path.join(dir, 'external-command-ran');
  const bin = path.join(dir, 'bin');
  fs.mkdirSync(bin);
  fs.writeFileSync(path.join(bin, 'git'), `#!/bin/sh
if [ "$3" = "branch" ] && [ "$4" = "--show-current" ]; then
  echo "\${NBR_FAKE_BRANCH:-feature/release-test}"
  exit 0
fi
if [ "$3" = "status" ]; then
  printf '%s' "\${NBR_FAKE_STATUS:-}"
  exit 0
fi
exit 92
`, {mode: 0o700});
  for (const name of ['npm', 'gh', 'clawhub']) {
    fs.writeFileSync(path.join(bin, name), `#!/bin/sh
echo ${name} >> ${JSON.stringify(marker)}
exit 93
`, {mode: 0o700});
  }
  const root = path.resolve(__dirname, '../../..');
  const commands = [
    ['release_nanobazaar_cli.sh', ['3.0.0', 'npm', '--confirm-publish']],
    ['release_nanobazaar_cli.sh', ['3.0.0', 'github', '--confirm-release']],
    ['publish_nanobazaar_skill.sh', ['3.0.0', 'publish', '--confirm-publish']],
  ];
  for (const scenario of [
    {env: {NBR_FAKE_BRANCH: 'feature/release-test'}, error: /main branch/},
    {env: {NBR_FAKE_BRANCH: 'main', NBR_FAKE_STATUS: ' M release-file'}, error: /clean working tree/},
  ]) {
    for (const [script, args] of commands) {
      const env = {...process.env, ...scenario.env, PATH: `${bin}:${process.env.PATH}`};
      const result = spawnSync('bash', [path.join(root, 'scripts', script), ...args], {encoding: 'utf8', env});
      assert.equal(result.status, 1, result.stderr);
      assert.match(result.stderr, scenario.error);
    }
  }
  assert.equal(fs.existsSync(marker), false);
});
