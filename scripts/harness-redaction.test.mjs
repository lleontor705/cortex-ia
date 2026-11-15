import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const TELEMETRY = 'internal/assets/plugins/cortex-tool-telemetry.ts';
const BRIDGE = 'internal/assets/plugins/cortex-work.ts';

const root = path.resolve('/isolated/harness');
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');

function telemetryOptions(calls = []) {
  return {
    cwd: root,
    env: { USERPROFILE: home, HOME: home },
    fs: {
      existsSync: p => p === executable,
      realpathSync: p => p,
      lstatSync: () => ({ isSymbolicLink: () => false }),
      readFileSync: () => '',
      writeFileSync: () => {},
      mkdirSync: () => {},
    },
    childProcess: {
      execFileSync: () => executable,
      execFile: (_file, args, _opts, cb) => {
        calls.push(args);
        if (typeof cb === 'function') cb(null, '', '');
      },
    },
  };
}

async function redaction(calls = []) {
  const mod = await loadPluginFile(TELEMETRY, telemetryOptions(calls));
  return {
    calls,
    redactSecrets: mod.exports.redactSecrets,
    hasUnredactableSecret: mod.exports.hasUnredactableSecret,
    reportPromptRedactionDrop: mod.exports.reportPromptRedactionDrop,
  };
}

// Each case pairs a labeled secret with the typed placeholder the shared
// helper must emit; the label keeps the surrounding text intact so position
// context survives for a human reading the sanitized value.
const TYPED_CASES = [
  { kind: 'private', input: '<private>\n multi line demo body \n</private>', secret: 'multi line demo body' },
  { kind: 'connection-string', input: 'postgres://demo:DEMOPASS1@db.invalid:5432/app', secret: 'DEMOPASS1' },
  { kind: 'credential-url', input: 'https://demo:DEMOPASS2@example.invalid/path', secret: 'DEMOPASS2' },
  { kind: 'token', input: 'Authorization: Bearer DEMOBEARER1', secret: 'DEMOBEARER1' },
  { kind: 'token', input: 'Basic ZGVtb2Jhc2ljc2VjcmV0', secret: 'ZGVtb2Jhc2ljc2VjcmV0' },
  { kind: 'apikey', input: 'api_key = "DEMOAPIKEY1"', secret: 'DEMOAPIKEY1' },
  { kind: 'password', input: 'password: DEMOPASSWORD1', secret: 'DEMOPASSWORD1' },
  { kind: 'secret', input: 'client_secret=DEMOCLIENTSECRET1', secret: 'DEMOCLIENTSECRET1' },
  { kind: 'token', input: 'access_token: DEMOACCESSTOKEN1', secret: 'DEMOACCESSTOKEN1' },
];

test('typed redaction emits [REDACTED:<kind>] for each supported kind', async () => {
  const { redactSecrets, hasUnredactableSecret } = await redaction();

  for (const { kind, input, secret } of TYPED_CASES) {
    const redacted = redactSecrets(input);
    assert.ok(
      redacted.includes(`[REDACTED:${kind}]`),
      `expected a [REDACTED:${kind}] emission for input '${input}', got '${redacted}'`
    );
    assert.ok(!redacted.includes(secret), `secret literal '${secret}' must not survive redaction`);
    assert.equal(
      hasUnredactableSecret(redacted),
      false,
      'a fully typed redaction must remain persistable (no drop)'
    );
  }
});

test('bearer/basic redaction is case-insensitive across whitespace including newline', async () => {
  const { redactSecrets } = await redaction();

  const cases = [
    ['bEaReR\nDEMOBEARER2', 'DEMOBEARER2'],
    ['BASIC\tZGVtb2Jhc2ljMg==', 'ZGVtb2Jhc2ljMg=='],
    ['Api-Key:\nDEMODASHKEY1', 'DEMODASHKEY1'],
    ['PASSWORD\n=\nDEMONEWLINE1', 'DEMONEWLINE1'],
  ];

  for (const [input, secret] of cases) {
    const redacted = redactSecrets(input);
    assert.match(redacted, /\[REDACTED:(token|apikey|password)\]/, `no typed placeholder for '${input}'`);
    assert.ok(!redacted.includes(secret), `secret literal '${secret}' must not survive redaction`);
  }
});

test('<private> blocks are stripped as [REDACTED:private] and persisted', async () => {
  const { redactSecrets, hasUnredactableSecret } = await redaction();

  const redacted = redactSecrets('<private>demo private body</private>');
  assert.equal(redacted, '[REDACTED:private]');
  assert.equal(hasUnredactableSecret(redacted), false, 'a stripped private block must persist');
});

// Synthetic shapes only; every value is non-functional filler sized to the
// declared detector regexes, never a real credential.
const UNREDACTABLE_CASES = [
  { name: 'PEM private key', secret: '-----BEGIN RSA PRIVATE KEY-----' },
  { name: 'JWT', secret: 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJkZW1vIn0.c2lnbmF0dXJlLWRlbW8' },
  { name: 'AWS AKIA', secret: 'AKIAABCDEFGHIJKLMNOP' },
  { name: 'AWS ASIA', secret: 'ASIAABCDEFGHIJKLMNOP' },
  { name: 'Google AIza', secret: 'AIza' + 'A'.repeat(35) },
  { name: 'GitHub ghp_', secret: 'ghp_' + 'a'.repeat(24) },
  { name: 'GitHub github_pat_', secret: 'github_pat_' + 'b'.repeat(24) },
  { name: 'Slack xoxb', secret: 'xoxb-' + '1'.repeat(12) },
  { name: 'Anthropic sk-ant-', secret: 'sk-ant-' + 'c'.repeat(20) },
  { name: 'OpenAI sk-proj-', secret: 'sk-proj-' + 'd'.repeat(20) },
  { name: 'OpenAI sk-', secret: 'sk-' + 'e'.repeat(40) },
  { name: 'Stripe sk_live_', secret: 'sk_live_' + 'f'.repeat(24) },
  { name: 'npm_', secret: 'npm_' + 'g'.repeat(36) },
  { name: 'GitLab glpat-', secret: 'glpat-' + 'h'.repeat(24) },
];

test('unlabeled secret shapes fail the prompt drop gate and never reach persistence', async () => {
  const calls = [];
  const { redactSecrets, hasUnredactableSecret, reportPromptRedactionDrop } = await redaction(calls);

  for (const [index, { name, secret }] of UNREDACTABLE_CASES.entries()) {
    assert.equal(hasUnredactableSecret(secret), true, `${name}: raw shape must be detected`);

    const redacted = redactSecrets(secret);
    assert.equal(
      hasUnredactableSecret(redacted),
      true,
      `${name}: shape redaction could not rewrite must still trip the drop gate`
    );

    // Mirror cortex.ts prompt capture: a positive gate call reports the drop
    // and returns before any persistence request is issued.
    const sessionID = `ses-drop-${index}`;
    reportPromptRedactionDrop(root, sessionID, 'unredactable secret shape');
    const args = calls[calls.length - 1];
    assert.equal(args[args.indexOf('--code') + 1], 'ERR_PROMPT_UNREDACTABLE', `${name}: drop must ride ERR_PROMPT_UNREDACTABLE`);
    assert.equal(args[args.indexOf('--source') + 1], 'cortex-prompt-capture');
    assert.ok(args[args.indexOf('--message') + 1].startsWith('Prompt capture dropped'));
    assert.ok(!args.join(' ').includes(secret), `${name}: the report must never carry the raw secret`);
  }
});

test('labeled secrets are redacted and pass the drop gate', async () => {
  const { redactSecrets, hasUnredactableSecret } = await redaction();

  const labeled = 'password=DEMOPASSWORD2 api_key=DEMOAPIKEY2 token=DEMOTOKEN2';
  const redacted = redactSecrets(labeled);
  assert.ok(!redacted.includes('DEMOPASSWORD2') && !redacted.includes('DEMOAPIKEY2') && !redacted.includes('DEMOTOKEN2'));
  assert.equal(hasUnredactableSecret(redacted), false, 'a labeled secret is rewritten, not dropped');
});

async function compactReceipt(verdicts) {
  const bridge = await loadPluginFile(BRIDGE, { allowChildProcess: true });
  const structured_output = verdicts === undefined ? { summary: 'ok' } : { summary: 'ok', verdicts };
  const res = { status: 'succeeded', output: { structured_output } };
  return bridge.exports.extractCompactReceipt({ status: 'succeeded' }, res, 'job-redaction-1');
}

test('normalizeVerdicts drops malformed entries and trims accepted ones', async () => {
  const result = await compactReceipt([
    { req_id: 'REQ-1', verdict: 'PASS', evidence_ref: 'ev-1' },
    { req_id: ' REQ-2 ', verdict: ' BLOCKED ', evidence_ref: 'ev-2' },
    { req_id: 'REQ-3', verdict: 'MAYBE', evidence_ref: 'ev-3' },
    { req_id: '   ', verdict: 'PASS', evidence_ref: 'ev-4' },
    { req_id: 'REQ-5', verdict: 'PASS', evidence_ref: 42 },
    { req_id: 'REQ-6', verdict: 'fail', evidence_ref: 'ev-6' },
    null,
    'REQ-7',
  ]);

  // Sandbox-realm arrays have a foreign prototype, so compare structurally.
  assert.deepEqual(JSON.parse(JSON.stringify(result.output.verdicts)), [
    { req_id: 'REQ-1', verdict: 'PASS', evidence_ref: 'ev-1' },
    { req_id: 'REQ-2', verdict: 'BLOCKED', evidence_ref: 'ev-2' },
  ]);
});

test('normalizeVerdicts omits the field when absent or non-array and keeps an empty array', async () => {
  const absent = await compactReceipt(undefined);
  assert.ok(!('verdicts' in absent.output), 'an absent verdicts field must be omitted');
  assert.ok(!('verdicts' in absent), 'the compact result must not fabricate verdicts');

  for (const bad of ['PASS', {}, 42, null]) {
    const result = await compactReceipt(bad);
    assert.ok(!('verdicts' in result.output), `non-array verdicts ${JSON.stringify(bad)} must be omitted`);
  }

  const empty = await compactReceipt([]);
  assert.ok(Array.isArray(empty.output.verdicts), 'an explicitly empty array is retained');
  assert.equal(empty.output.verdicts.length, 0);
});
