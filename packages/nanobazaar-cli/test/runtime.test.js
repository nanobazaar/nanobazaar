'use strict';
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');
const {isSupportedRuntime} = require('../lib/runtime');

const root = path.resolve(__dirname, '../../..');
const packageRoot = path.resolve(__dirname, '..');

test('runtime policy accepts only stable patched Node 24 releases', () => {
  for (const version of ['v24.22.1', '24.22.1', 'v24.22.2', 'v24.23.0']) {
    assert.equal(isSupportedRuntime(version), true, version);
  }
  for (const version of [
    undefined, null, '', '24', '24.22', 'v24.22.0', 'v24.21.99', 'v22.99.0',
    'v25.0.0', 'v26.0.0', 'v24.22.1-rc.1', 'v24.22.1+build', 'v24.022.1',
    'v24.99999999999999999999.1', 'v24.22.1\n',
  ]) {
    assert.equal(isSupportedRuntime(version), false, String(version));
  }
});

function fixture(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'nbr-runtime-'));
  t.after(() => fs.rmSync(directory, {recursive: true, force: true}));
  const marker = path.join(directory, 'side-effect');
  const preload = path.join(directory, 'old-node.cjs');
  fs.writeFileSync(preload, `
Object.defineProperty(process, 'version', {value: 'v24.22.0'});
Object.defineProperty(process.versions, 'node', {value: '24.22.0'});
const fs = require('node:fs');
function networkAttempt() { fs.writeFileSync(${JSON.stringify(marker)}, 'network'); process.exit(91); }
global.fetch = networkAttempt;
for (const name of ['node:http', 'node:https']) {
  const transport = require(name);
  transport.request = transport.get = networkAttempt;
}
require('node:net').connect = require('node:net').createConnection = networkAttempt;
`);
  return {
    directory, marker, preload,
    env: {
      ...process.env,
      NBR_STATE_PATH: path.join(directory, 'state', 'identity.json'),
      XDG_CONFIG_HOME: path.join(directory, 'config'),
    },
  };
}

function assertRuntimeRejection(result) {
  assert.equal(result.status, 1, result.stderr);
  assert.equal(result.stdout, '');
  assert.match(result.stderr, /Node\.js 24\.22\.1 or newer within 24\.x is required; detected v24\.22\.0/);
}

test('CLI aliases and direct setup reject old Node before state or network activity', t => {
  const {directory, preload, env} = fixture(t);
  const commands = [
    ...[[], ['--help'], ['-h'], ['--version'], ['-v'], ['version'],
      ['setup', '--skip-register'], ['market', 'offers']].map(args => [
      path.join(packageRoot, 'bin/nanobazaar'), ...args,
    ]),
    [path.join(packageRoot, 'tools/setup.js')],
    [path.join(packageRoot, 'tools/setup.js'), '--skip-register'],
  ];
  for (const args of commands) {
    assertRuntimeRejection(spawnSync(process.execPath, ['--require', preload, ...args], {
      encoding: 'utf8', env,
    }));
  }
  assert.deepEqual(fs.readdirSync(directory), ['old-node.cjs']);
});

test('release helpers reject old host Node before tools, artifacts, or publication', t => {
  const {directory, preload, marker, env} = fixture(t);
  const bin = path.join(directory, 'bin');
  fs.mkdirSync(bin);
  const quote = value => "'" + value.replaceAll("'", "'\\''") + "'";
  fs.writeFileSync(path.join(bin, 'node'),
    '#!/bin/sh\nexec ' + quote(process.execPath) + ' --require ' + quote(preload) + ' "$@"\n',
    {mode: 0o700});
  for (const name of ['npm', 'go', 'curl', 'docker', 'git', 'gh', 'clawhub']) {
    fs.writeFileSync(path.join(bin, name), '#!/bin/sh\ntouch ' + quote(marker) + '\nexit 92\n', {mode: 0o700});
  }
  const commands = [
    ['verify_nanobazaar_release.sh', '3.0.0'],
    ['verify_nanobazaar_release.sh', '3.0.0', '--skip-docker'],
    ['release_nanobazaar_cli.sh', '3.0.0', 'check'],
    ['release_nanobazaar_cli.sh', '3.0.0', 'npm', '--confirm-publish'],
    ['release_nanobazaar_cli.sh', '3.0.0', 'github', '--confirm-release'],
    ['publish_nanobazaar_skill.sh', '3.0.0', 'check'],
    ['publish_nanobazaar_skill.sh', '3.0.0', 'publish', '--confirm-publish'],
  ];
  for (const [script, ...args] of commands) {
    assertRuntimeRejection(spawnSync('bash', [path.join(root, 'scripts', script), ...args], {
      encoding: 'utf8',
      env: {...env, PATH: bin + path.delimiter + env.PATH, NBR_RELEASE_ARTIFACT_DIR: path.join(directory, 'artifacts')},
    }));
  }
  assert.deepEqual(fs.readdirSync(directory).sort(), ['bin', 'old-node.cjs']);
});

test('supported runtime preserves CLI version and help commands', () => {
  const version = spawnSync(process.execPath, [path.join(packageRoot, 'bin/nanobazaar'), '--version'], {encoding: 'utf8'});
  assert.equal(version.status, 0, version.stderr);
  assert.equal(version.stdout.trim(), require('../package.json').version);
  const help = spawnSync(process.execPath, [path.join(packageRoot, 'bin/nanobazaar'), '--help'], {encoding: 'utf8'});
  assert.equal(help.status, 0, help.stderr);
  assert.match(help.stdout, /nanobazaar/i);
});
