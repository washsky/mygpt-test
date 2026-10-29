const test = require('node:test');
const assert = require('node:assert/strict');
const cleanup = require('../.github/scripts/cleanup.cjs');

const date = days => new Date(Date.now() - days * 86400000).toISOString();
const runs = [
  {id: 1, workflow_id: 10, name: 'CI', status: 'completed', created_at: date(0)},
  {id: 2, workflow_id: 10, name: 'CI', status: 'completed', created_at: date(2)},
  {id: 3, workflow_id: 10, name: 'CI', status: 'completed', created_at: date(40)},
  {id: 4, workflow_id: 20, name: 'Cleanup', status: 'in_progress', created_at: date(0)},
  {id: 5, workflow_id: 20, name: 'Cleanup', status: 'completed', created_at: date(1)},
];
const releases = [
  {id: 11, tag_name: 'v3', published_at: date(0), draft:false, prerelease:false},
  {id: 12, tag_name: 'v2', published_at: date(2), draft:false, prerelease:false},
  {id: 13, tag_name: 'v1', published_at: date(40), draft:false, prerelease:false},
  {id: 14, tag_name: 'draft', created_at: date(40), draft:true, prerelease:false},
  {id: 15, tag_name: 'beta', published_at: date(40), draft:false, prerelease:true},
];
async function run({scope='runs',days='0',keep='0',dry='true',runId=4}={}) {
  Object.assign(process.env,{CLEANUP_SCOPE:scope,CLEANUP_DAYS:days,CLEANUP_KEEP:keep,CLEANUP_DRY_RUN:dry});
  const deleted=[], logs=[], summary=[];
  const rest={actions:{listWorkflowRunsForRepo:()=>{},listArtifactsForRepo:()=>{},getWorkflowRun:async({run_id})=>({data:runs.find(x=>x.id===run_id)}),deleteWorkflowRun:async({run_id})=>deleted.push(run_id),deleteArtifact:async()=>{}},repos:{listReleases:()=>{},deleteRelease:async({release_id})=>deleted.push(release_id)}};
  const github={rest,paginate:async fn=>fn===rest.actions.listWorkflowRunsForRepo?runs:fn===rest.repos.listReleases?releases:[]};
  const core={summary:{addHeading:()=>{},addRaw:s=>summary.push(s),addTable:rows=>summary.push(rows),write:async()=>{}},info:s=>logs.push(s),warning:()=>{},setFailed:s=>{throw Error(s)}};
  await cleanup({github,context:{repo:{owner:'owner',repo:'repo'},runId},core});
  return {deleted,logs,summary};
}
const ids = result => result.logs.map(x=>Number.parseInt(x.split(' ')[1],10));

test('only quantity: zero days keeps exactly the newest completed run per workflow',async()=>{
  const result=await run({keep:'1'});assert.deepEqual(ids(result),[2,3]);assert.deepEqual(result.deleted,[]);assert.match(result.summary[0],/不限制时间/);
});
test('only age: zero retained allows old releases in each group but excludes drafts',async()=>{
  const result=await run({scope:'releases',days:'30'});assert.deepEqual(ids(result),[13,15]);
});
test('both conditions: young items outside retention are not selected',async()=>{
  const result=await run({days:'30',keep:'1'});assert.deepEqual(ids(result),[3]);
});
test('zero and zero includes the latest release and deletes every eligible item',async()=>{
  const result=await run({scope:'releases',dry:'false'});assert.deepEqual(result.deleted,[11,12,13,15]);
});
test('current and active runs remain protected even when no completed runs are retained',async()=>{
  const result=await run({runId:5,dry:'false'});assert.deepEqual(result.deleted,[1,2,3]);
});
test('missing, negative, non-integer and unsafe inputs cannot mean zero',async()=>{
  for(const opts of [{days:''},{keep:''},{days:'-1'},{keep:'1.5'},{days:'9007199254740992'}]) await assert.rejects(run(opts),/非负整数/);
});
