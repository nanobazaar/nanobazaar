'use strict';

const REQUIRED_RUNTIME = 'Node.js 24.22.1 or newer within 24.x';

function isSupportedRuntime(version) {
  if (typeof version !== 'string') return false;
  const match = /^v?24\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.exec(version);
  if (!match) return false;
  const minor = Number(match[1]);
  const patch = Number(match[2]);
  return Number.isSafeInteger(minor) && Number.isSafeInteger(patch) &&
    (minor > 22 || (minor === 22 && patch >= 1));
}

function enforceRuntime() {
  if (!isSupportedRuntime(process.version)) {
    console.error(`${REQUIRED_RUNTIME} is required; detected ${process.version || 'unknown'}.`);
    process.exit(1);
  }
}

if (require.main === module) enforceRuntime();

module.exports = {isSupportedRuntime, enforceRuntime};
