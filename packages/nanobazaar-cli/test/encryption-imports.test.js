'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const sodium=require('libsodium-wrappers');
const { webcrypto }=require('node:crypto');
const { execFileSync }=require('node:child_process');
const path=require('node:path');
globalThis.crypto ??= webcrypto;
const legacy=require('./fixtures/sealed-box-0.7.16.json');

test('CommonJS and direct ESM imports exchange sealed boxes', async()=>{
 const esm=(await import('libsodium-wrappers')).default;
 await Promise.all([sodium.ready,esm.ready]);
 const keys=sodium.crypto_box_keypair();
 for (const [encryptor,decryptor] of [[esm,sodium],[sodium,esm]]) {
  const ciphertext=encryptor.crypto_box_seal('NanoBazaar encrypted delivery',keys.publicKey);
  assert.equal(decryptor.crypto_box_seal_open(ciphertext,keys.publicKey,keys.privateKey,'text'),'NanoBazaar encrypted delivery');
 }
});

test('previous 0.7.16 ciphertext remains readable from both module entry points',async()=>{
 const esm=(await import('libsodium-wrappers')).default;
 await Promise.all([sodium.ready,esm.ready]);
 for(const module of [sodium,esm]) {
  assert.equal(module.crypto_box_seal_open(Buffer.from(legacy.ciphertext,'hex'),Buffer.from(legacy.publicKey,'hex'),Buffer.from(legacy.privateKey,'hex'),'text'),legacy.message);
 }
});

test('standalone ESM file runs on the supported Node version', () => {
 const output=execFileSync(process.execPath,[path.resolve(__dirname,'../examples/sealed-box.mjs')],{encoding:'utf8'});
 assert.equal(output.trim(),'NanoBazaar encrypted delivery');
});
