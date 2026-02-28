'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const {spawnSync} = require('node:child_process');

function buildKeyEnv() {
  const signing = crypto.generateKeyPairSync('ed25519');
  const signingPrivateJwk = signing.privateKey.export({format: 'jwk'});
  const signingPublicJwk = signing.publicKey.export({format: 'jwk'});

  const encryption = crypto.generateKeyPairSync('x25519');
  const encryptionPrivateJwk = encryption.privateKey.export({format: 'jwk'});
  const encryptionPublicJwk = encryption.publicKey.export({format: 'jwk'});

  return {
    NBR_SIGNING_PRIVATE_KEY_B64URL: signingPrivateJwk.d,
    NBR_SIGNING_PUBLIC_KEY_B64URL: signingPublicJwk.x,
    NBR_ENCRYPTION_PRIVATE_KEY_B64URL: encryptionPrivateJwk.d,
    NBR_ENCRYPTION_PUBLIC_KEY_B64URL: encryptionPublicJwk.x,
  };
}

test('watch initializes without undefined variable crashes', () => {
  const cliPath = path.resolve(__dirname, '..', 'bin', 'nanobazaar');
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nanobazaar-cli-watch-'));
  const statePath = path.join(tempDir, 'state.json');

  try {
    const result = spawnSync(process.execPath, [cliPath, 'watch', '--no-openclaw'], {
      encoding: 'utf8',
      timeout: 1200,
      killSignal: 'SIGTERM',
      env: {
        ...process.env,
        ...buildKeyEnv(),
        NBR_RELAY_URL: 'http://127.0.0.1:1',
        NBR_STATE_PATH: statePath,
      },
    });

    if (result.error && result.error.code !== 'ETIMEDOUT') {
      throw result.error;
    }

    const stderr = String(result.stderr || '');
    assert.match(stderr, /\[watch\] stream_path=\/v0\/stream/);
    assert.match(stderr, /\[watch\] poll_on_wake=true/);
    assert.match(stderr, /\[watch\] max_wake_poll_cycles=5/);
    assert.doesNotMatch(stderr, /streams is not defined/i);
    assert.doesNotMatch(stderr, /safetyIntervalSeconds is not defined/i);
    assert.doesNotMatch(stderr, /runPollLoop is not defined/i);
  } finally {
    fs.rmSync(tempDir, {recursive: true, force: true});
  }
});
