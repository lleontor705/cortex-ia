import test from 'node:test';
import assert from 'node:assert/strict';
import path from 'node:path';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
const home = path.resolve('/isolated/home');
const executable = path.join(home, 'go', 'bin', 'cortex-ia.exe');
const context = { agent: 'implement', sessionID: 'reservation-test', directory: '/isolated/work' };

async function harness(external = false) {
  const calls = [];
  let failClaim = false;
  const mod = await loadPluginFile('internal/assets/plugins/herdr-bridge.ts', {
    env: { HOME: home, USERPROFILE: home },
    mockPluginSDK: { tool: Object.assign(x => x, { schema }) },
    fs: { existsSync: p => p === executable, readFileSync: () => '', mkdirSync: () => {}, writeFileSync: () => {} },
    childProcess: { execFileSync: (_file, args, opts) => {
      calls.push({ args: Array.from(args), input: opts?.input });
      if (args[0] === 'delegate') return JSON.stringify({ schema_version: 1, role: 'implement', external_enabled: external,
        reason: external ? 'external_enabled' : 'role_native' });
      if (args[1] === 'status') return JSON.stringify({ status: 'in_progress', claim: {
        owner: 'opencode-session:reservation-test', expires_at: new Date(Date.now() + 60000).toISOString() } });
      if (args[1] === 'claim' && failClaim) throw new Error('batch conflict');
      const paths = args.flatMap((x, i) => x === '--path' ? [args[i + 1]] : []).sort();
      const leases = paths.map(p => ({ path: p, lease_token: `secret-${p}` }));
      if (args[1] === 'claim') return JSON.stringify({ claim_token: 'secret-claim', reserved_files: leases });
      if (args[1] === 'reserve') return JSON.stringify(leases.length === 1 ? leases[0] : { reserved: leases });
      return '{}';
    } },
  });
  const plugin = await mod.instantiate({ client: {} });
  return { tools: plugin.tool, calls, failClaim: value => { failClaim = value; } };
}

test('claim and reserve send one batch each and never expose authority tokens', async () => {
  const h = await harness();
  const claimed = await h.tools.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['b.go', 'a.go'] }, context);
  const claims = h.calls.filter(x => x.args[1] === 'claim');
  assert.equal(claims.length, 1);
  assert.deepEqual(claims[0].args.slice(-4), ['--path', 'b.go', '--path', 'a.go']);
  assert.equal(claimed.includes('secret-'), false);
  assert.equal(JSON.parse(claimed).maintenance.active, false);
  assert.equal(JSON.parse(claimed).maintenance.reason, 'host_status_unavailable_manual_renewal_required');
  assert.deepEqual(JSON.parse(claimed).reserved_files.map(x => x.path), ['a.go', 'b.go']);
  const reserved = await h.tools.cortex_ia_file_reserve.execute({ task_id: 'task', paths: ['d.go', 'c.go'] }, context);
  const acquisitions = h.calls.filter(x => x.args[1] === 'reserve');
  assert.equal(acquisitions.length, 1);
  assert.deepEqual(acquisitions[0].args.slice(-4), ['--path', 'd.go', '--path', 'c.go']);
  assert.equal(reserved.includes('secret-'), false);
  assert.equal(acquisitions[0].args.includes('secret-claim'), false);
  assert.equal(JSON.parse(reserved).count, 2);
});

test('failed atomic claim does not retain a phantom claim in the bridge', async () => {
  const h = await harness();
  h.failClaim(true);
  await assert.rejects(h.tools.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context), /batch conflict/);
  h.failClaim(false);
  await h.tools.cortex_ia_work_claim.execute({ task_id: 'task', paths: ['a.go'] }, context);
  assert.equal(h.calls.filter(x => x.args[1] === 'claim').length, 2);
});

test('native preference consults policy and cannot override external execution', async () => {
  for (const external of [false, true]) {
    const h = await harness(external);
    const result = JSON.parse(await h.tools.cortex_ia_delegate_start.execute({
      role: 'implement', objective: 'bounded objective', prefer_native: true,
    }, context));
    assert.ok(h.calls.some(x => x.args[0] === 'delegate' && x.args[1] === 'policy'));
    if (external) {
      assert.equal(result.status, 'blocked');
      assert.equal(result.error.code, 'DELEGATION_POLICY_CONFLICT');
      assert.equal(result.execution_mode, undefined);
    } else {
      assert.equal(result.execution_mode, 'native');
      assert.equal(result.reason, 'role_native');
    }
  }
});
