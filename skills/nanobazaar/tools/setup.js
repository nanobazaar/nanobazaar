#!/usr/bin/env node
'use strict';

const crypto = require('crypto');
const fs = require('fs');
const http = require('http');
const https = require('https');
const os = require('os');
const path = require('path');
const {spawnSync} = require('child_process');

const DEFAULT_RELAY_URL = 'https://relay.nanobazaar.ai';
const XDG_CONFIG_HOME = (process.env.XDG_CONFIG_HOME || '').trim();
const CONFIG_BASE_DIR = XDG_CONFIG_HOME || path.join(os.homedir(), '.config');
const STATE_DEFAULT = path.join(CONFIG_BASE_DIR, 'nanobazaar', 'nanobazaar.json');

const args = new Set(process.argv.slice(2));
const installBerryPay = !args.has('--no-install-berrypay');
const skipRegister = args.has('--skip-register');

const env = process.env;
const relayUrl = (env.NBR_RELAY_URL || DEFAULT_RELAY_URL).trim();
const statePath = (env.NBR_STATE_PATH || STATE_DEFAULT).trim();
const berrypayBin = (env.NBR_BERRYPAY_BIN || 'berrypay').trim();

function base32Encode(buffer) {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
  let bits = 0;
  let value = 0;
  let output = '';

  for (const byte of buffer) {
    value = (value << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      output += alphabet[(value >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }

  if (bits > 0) {
    output += alphabet[(value << (5 - bits)) & 31];
  }

  return output;
}

function multibaseBase32(buffer) {
  return 'b' + base32Encode(buffer);
}

function sha256(buffer) {
  return crypto.createHash('sha256').update(buffer).digest();
}

function sha256Hex(buffer) {
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

function loadState(filePath) {
  try {
    const raw = fs.readFileSync(filePath, 'utf8');
    return JSON.parse(raw);
  } catch (err) {
    return {};
  }
}

function saveState(filePath, state) {
  fs.mkdirSync(path.dirname(filePath), {recursive: true});
  fs.writeFileSync(filePath, JSON.stringify(state, null, 2));
  try {
    fs.chmodSync(filePath, 0o600);
  } catch (_) {
    // ignore chmod errors on unsupported platforms
  }
}

function getEnvValue(name) {
  const value = env[name];
  return value && value.trim() ? value.trim() : '';
}

function loadKeysFromEnv() {
  const signingPrivate = getEnvValue('NBR_SIGNING_PRIVATE_KEY_B64URL');
  const signingPublic = getEnvValue('NBR_SIGNING_PUBLIC_KEY_B64URL');
  const encryptionPrivate = getEnvValue('NBR_ENCRYPTION_PRIVATE_KEY_B64URL');
  const encryptionPublic = getEnvValue('NBR_ENCRYPTION_PUBLIC_KEY_B64URL');

  if (!signingPrivate || !signingPublic || !encryptionPrivate || !encryptionPublic) {
    return null;
  }

  return {
    signing_private_key_b64url: signingPrivate,
    signing_public_key_b64url: signingPublic,
    encryption_private_key_b64url: encryptionPrivate,
    encryption_public_key_b64url: encryptionPublic,
  };
}

function generateKeyPair(kind) {
  const {publicKey, privateKey} = crypto.generateKeyPairSync(kind);
  const publicJwk = publicKey.export({format: 'jwk'});
  const privateJwk = privateKey.export({format: 'jwk'});
  return {publicJwk, privateJwk};
}

function resolveKeys(state) {
  const envKeys = loadKeysFromEnv();
  if (envKeys) {
    return {keys: envKeys, source: 'env'};
  }

  if (state.keys &&
      state.keys.signing_private_key_b64url &&
      state.keys.signing_public_key_b64url &&
      state.keys.encryption_private_key_b64url &&
      state.keys.encryption_public_key_b64url) {
    return {keys: state.keys, source: 'state'};
  }

  const signingPair = generateKeyPair('ed25519');
  const encryptionPair = generateKeyPair('x25519');

  return {
    source: 'generated',
    keys: {
      signing_private_key_b64url: signingPair.privateJwk.d,
      signing_public_key_b64url: signingPair.publicJwk.x,
      encryption_private_key_b64url: encryptionPair.privateJwk.d,
      encryption_public_key_b64url: encryptionPair.publicJwk.x,
    },
  };
}

function ensureBerryPay() {
  const result = spawnSync(berrypayBin, ['--version'], {stdio: 'ignore'});
  if (result.status === 0) {
    return true;
  }
  if (!installBerryPay) {
    return false;
  }

  const npmCheck = spawnSync('npm', ['--version'], {stdio: 'ignore'});
  if (npmCheck.status !== 0) {
    return false;
  }

  const npmResult = spawnSync('npm', ['install', '-g', 'berrypay'], {stdio: 'inherit'});
  if (npmResult.status !== 0) {
    return false;
  }
  const retry = spawnSync(berrypayBin, ['--version'], {stdio: 'ignore'});
  return retry.status === 0;
}

function request(url, body, headers) {
  return new Promise((resolve, reject) => {
    const client = url.protocol === 'https:' ? https : http;
    const req = client.request(
      {
        method: 'POST',
        hostname: url.hostname,
        port: url.port,
        path: url.pathname + url.search,
        headers,
      },
      (res) => {
        let data = '';
        res.on('data', (chunk) => { data += chunk; });
        res.on('end', () => {
          resolve({status: res.statusCode || 0, body: data});
        });
      }
    );
    req.on('error', reject);
    req.write(body);
    req.end();
  });
}

async function main() {
  const state = loadState(statePath);
  const resolved = resolveKeys(state);

  const keys = resolved.keys;
  const signingPubBytes = Buffer.from(keys.signing_public_key_b64url, 'base64url');
  const encryptionPubBytes = Buffer.from(keys.encryption_public_key_b64url, 'base64url');

  const signingKid = multibaseBase32(sha256(signingPubBytes).subarray(0, 16));
  const encryptionKid = multibaseBase32(sha256(encryptionPubBytes).subarray(0, 16));
  const botId = multibaseBase32(sha256(signingPubBytes));

  const payload = {
    signing_pubkey_ed25519: keys.signing_public_key_b64url,
    encryption_pubkey_x25519: keys.encryption_public_key_b64url,
    signing_kid: signingKid,
    encryption_kid: encryptionKid,
  };

  const body = JSON.stringify(payload);
  const bodyHash = sha256Hex(body);
  const timestamp = new Date().toISOString();
  const nonce = crypto.randomBytes(8).toString('hex');

  const base = new URL(relayUrl);
  const basePath = base.pathname.endsWith('/') ? base.pathname.slice(0, -1) : base.pathname;
  const url = new URL(basePath + '/v0/bots', base);
  const pathAndQuery = url.pathname + url.search;

  const canonical = `POST\n${pathAndQuery}\n${timestamp}\n${nonce}\n${bodyHash}`;
  const signingKey = crypto.createPrivateKey({
    format: 'jwk',
    key: {
      kty: 'OKP',
      crv: 'Ed25519',
      x: keys.signing_public_key_b64url,
      d: keys.signing_private_key_b64url,
    },
  });
  const signature = crypto.sign(null, Buffer.from(canonical, 'utf8'), signingKey).toString('base64url');

  const headers = {
    'Content-Type': 'application/json',
    'Content-Length': Buffer.byteLength(body),
    'X-NBR-Bot-Id': botId,
    'X-NBR-Timestamp': timestamp,
    'X-NBR-Nonce': nonce,
    'X-NBR-Body-SHA256': bodyHash,
    'X-NBR-Signature': signature,
    'X-Idempotency-Key': crypto.randomBytes(16).toString('hex'),
  };

  if (!skipRegister) {
    const response = await request(url, body, headers);
    if (response.status !== 200) {
      console.error(`Registration failed (${response.status}): ${response.body}`);
      process.exit(1);
    }
  }

  state.relay_url = relayUrl;
  state.bot_id = botId;
  state.signing_kid = signingKid;
  state.encryption_kid = encryptionKid;
  state.keys = keys;
  if (typeof state.last_acked_event_id !== 'number') {
    state.last_acked_event_id = 0;
  }

  saveState(statePath, state);

  const berrypayInstalled = ensureBerryPay();

  console.log('NanoBazaar setup complete.');
  console.log(`State path: ${statePath}`);
  console.log(`Relay URL: ${relayUrl}`);
  console.log(`Bot ID: ${botId}`);
  console.log(`Keys source: ${resolved.source}`);
  if (!berrypayInstalled) {
    console.log('BerryPay CLI not detected. Install it for automated payments.');
  } else {
    if (!process.env.BERRYPAY_SEED) {
      console.log('BerryPay CLI installed but BERRYPAY_SEED is not set.');
      console.log('Run `berrypay init` or set BERRYPAY_SEED to configure a wallet.');
    }
    console.log('Top up your BerryPay wallet with: /nanobazaar wallet');
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
