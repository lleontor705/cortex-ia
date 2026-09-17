import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { loadPluginFile } from './harness-plugin-loader.mjs';

const schema = new Proxy(() => schema, { get: () => schema });
const content = 'line\r\n雪😀\n';
const sha256 = createHash('sha256').update(content).digest('hex');
const receipt = { transport:'local_cortex_cli',project:'p',observation_id:7,content,sha256,byte_length:Buffer.byteLength(content) };
async function snapshot(producer) {
  const calls=[];
  const mod=await loadPluginFile('internal/assets/plugins/cortex-snapshot.ts',{
    mockPluginSDK:{tool:Object.assign(x=>x,{schema})},
    childProcess:{execFileSync:(file,args,opts)=>{calls.push({file,args:Array.from(args),opts});return producer();}}
  });
  const plugin=await mod.instantiate({});
  return {tool:plugin.tool.cortex_ia_snapshot_read,calls};
}

test('snapshot uses one bounded Cortex-IA stream request and preserves exact bytes',async()=>{
  const h=await snapshot(()=>JSON.stringify(receipt));
  const result=JSON.parse(await h.tool.execute({project:'p',observation_id:7,expected_sha256:sha256}));
  assert.deepEqual(result,receipt);
  assert.equal(h.calls.length,1);
  assert.equal(h.calls[0].file,'cortex-ia');
  assert.deepEqual(h.calls[0].args,['snapshot','read','--project','p','--id','7','--expected-sha256',sha256]);
  assert.equal(h.calls[0].opts.timeout,35000);
});

test('snapshot rejects failed producer and malformed or drifted receipts without retries',async()=>{
  for(const producer of [()=>{throw Error('private producer diagnostics')},()=>'{',
    ...[{...receipt,project:'other'},{...receipt,observation_id:8},{...receipt,content:'changed'},
      {...receipt,byte_length:1},{...receipt,transport:'remote'}].map(value=>()=>JSON.stringify(value))]) {
    const h=await snapshot(producer);
    await assert.rejects(h.tool.execute({project:'p',observation_id:7,expected_sha256:sha256}));
    assert.equal(h.calls.length,1);
  }
});

test('snapshot rejects invalid intent before starting producer',async()=>{
  const h=await snapshot(()=>JSON.stringify(receipt));
  for(const args of [{project:'p',observation_id:0},{project:'',observation_id:7},
    {project:'p',observation_id:7,expected_sha256:'bad'}]) await assert.rejects(h.tool.execute(args));
  assert.equal(h.calls.length,0);
});
