#!/usr/bin/env node
'use strict';

const {spawnSync} = require('child_process');

const argv = process.argv.slice(2);
let outputPath = '';
let showQr = true;

for (let i = 0; i < argv.length; i += 1) {
  const arg = argv[i];
  if (arg === '--output') {
    if (!argv[i + 1]) {
      console.error('Missing value for --output.');
      process.exit(1);
    }
    outputPath = argv[i + 1];
    i += 1;
    continue;
  }
  if (arg === '--no-qr') {
    showQr = false;
    continue;
  }
  console.error(`Unknown argument: ${arg}`);
  process.exit(1);
}

function run(cmd, args, opts) {
  return spawnSync(cmd, args, opts);
}

const bin = (process.env.NBR_BERRYPAY_BIN || 'berrypay').trim();

const version = run(bin, ['--version'], {stdio: 'ignore'});
if (version.status !== 0) {
  console.error('BerryPay CLI not detected.');
  console.error('Install with: npm install -g berrypay');
  console.error('Or set NBR_BERRYPAY_BIN to the CLI path.');
  process.exit(1);
}

const addressResult = run(bin, ['address'], {encoding: 'utf8'});
if (addressResult.status !== 0) {
  const stdout = (addressResult.stdout || '').trim();
  const stderr = (addressResult.stderr || '').trim();
  if (stdout) {
    console.error(stdout);
  }
  if (stderr) {
    console.error(stderr);
  }
  console.error('If no wallet is configured, run `berrypay init` or set BERRYPAY_SEED.');
  process.exit(addressResult.status || 1);
}

let address = '';
let payload;
try {
  payload = JSON.parse(addressResult.stdout);
} catch (_) {
  payload = null;
}

if (payload && payload.error) {
  console.error(`BerryPay error: ${payload.error}`);
  console.error('Run `berrypay init` or set BERRYPAY_SEED and retry.');
  process.exit(1);
}

if (payload && payload.address) {
  address = payload.address;
}

if (address) {
  console.log(`Wallet address: ${address}`);
} else if (addressResult.stdout.trim()) {
  console.log(addressResult.stdout.trim());
}

if (showQr) {
  const qrArgs = ['address', '--qr'];
  if (outputPath) {
    qrArgs.push('--output', outputPath);
  }
  const qrResult = run(bin, qrArgs, {stdio: 'inherit'});
  if (qrResult.status !== 0) {
    console.error('Failed to render QR code.');
    process.exit(qrResult.status || 1);
  }
  if (outputPath) {
    console.log(`QR code saved to: ${outputPath}`);
  }
}
