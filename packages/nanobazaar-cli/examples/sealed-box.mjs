// Standalone ESM round trip with throwaway keys; no relay or wallet access.
import { webcrypto } from 'node:crypto';

// Node 18 does not expose Web Crypto globally when running an ESM file.
globalThis.crypto ??= webcrypto;
const { default: sodium } = await import('libsodium-wrappers');

await sodium.ready;
const recipient = sodium.crypto_box_keypair();
const ciphertext = sodium.crypto_box_seal('NanoBazaar encrypted delivery', recipient.publicKey);
const message = sodium.crypto_box_seal_open(ciphertext, recipient.publicKey, recipient.privateKey, 'text');
console.log(message);
