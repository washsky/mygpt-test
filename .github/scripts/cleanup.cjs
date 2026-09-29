module.exports = async ({github, context, core}) => {
  const scope = process.env.CLEANUP_SCOPE;
  const days = Number(process.env.CLEANUP_DAYS);
  const keep = Number(process.env.CLEANUP_KEEP);
  const dryRun = process.env.CLEANUP_DRY_RUN !== 'false';
  if (!['artifacts', 'runs', 'releases', 'all'].includes(scope) || !Number.isInteger(days) || days < 1 || !Number.isInteger(keep) || keep < 1) {
    throw new Error('清理范围无效，天数和保留数量必须为正整数。');
  }
  const {owner, repo} = context.repo;
  const cutoff = Date.now() - days * 86400000;
  const candidates = [];
  const select = (items, groupBy, dateOf) => {
    const groups = new Map();
    for (const item of items) {
      const key = groupBy(item);
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(item);
    }
    const selected = [];
    for (const group of groups.values()) {
      group.sort((a,b) => Date.parse(dateOf(b)) - Date.parse(dateOf(a)));
      selected.push(...group.slice(keep).filter(item => Date.parse(dateOf(item)) < cutoff));
    }
    return selected;
  };
  if (scope === 'artifacts' || scope === 'all') {
    const artifacts = await github.paginate(github.rest.actions.listArtifactsForRepo, {owner, repo, per_page:100});
    for (const item of select(artifacts, item => item.name, item => item.created_at)) {
      candidates.push({kind:'artifact', id:item.id, name:item.name, date:item.created_at,
        remove:()=>github.rest.actions.deleteArtifact({owner,repo,artifact_id:item.id})});
    }
  }
  if (scope === 'runs' || scope === 'all') {
    const runs = await github.paginate(github.rest.actions.listWorkflowRunsForRepo, {owner,repo,per_page:100});
    // Protect all active runs, this run, and recent completed runs in each workflow.
    for (const item of select(runs.filter(item=>item.status==='completed'), item=>item.workflow_id, item=>item.created_at)) {
      if (item.id === context.runId) continue;
      candidates.push({kind:'run',id:item.id,name:item.name||String(item.workflow_id),date:item.created_at,
        remove:async()=>{const current=await github.rest.actions.getWorkflowRun({owner,repo,run_id:item.id});if(current.data.status!=='completed')return false;await github.rest.actions.deleteWorkflowRun({owner,repo,run_id:item.id});return true;}});
    }
  }
  if (scope === 'releases' || scope === 'all') {
    const releases = await github.paginate(github.rest.repos.listReleases, {owner,repo,per_page:100});
    let latestID = null;
    try { latestID=(await github.rest.repos.getLatestRelease({owner,repo})).data.id; } catch(error) { if(error.status!==404)throw error; }
    for (const item of select(releases.filter(item=>!item.draft), item=>item.prerelease?'prerelease':'release', item=>item.published_at||item.created_at)) {
      if (item.id===latestID) continue;
      candidates.push({kind:'release',id:item.id,name:item.tag_name,date:item.published_at||item.created_at,
        remove:()=>github.rest.repos.deleteRelease({owner,repo,release_id:item.id})});
    }
  }
  core.summary.addHeading(dryRun?'清理预览':'清理执行结果');
  core.summary.addRaw(`范围：${scope}；早于 ${days} 天；每组保留最新 ${keep} 项。Release 清理保留 Git 标签。工作流运行删除也会删除其关联产物。\n\n`);
  const rows = [[{data:'类型',header:true},{data:'ID',header:true},{data:'名称',header:true},{data:'创建/发布时间',header:true},{data:'结果',header:true}]];
  let failed=0;
  for (const item of candidates) {
    let result='待删除（预览）';
    if (!dryRun) {
      try { result=await item.remove()===false?'已跳过（正在运行）':'已删除'; }
      catch(error) { if(error.status===404)result='已不存在';else{result='失败';failed++;core.warning(`${item.kind} ${item.id}: ${error.message}`);} }
    }
    const escape = value => String(value).replace(/[&<>"']/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
    rows.push([item.kind,String(item.id),escape(item.name),item.date,result]);
    core.info(`${item.kind} ${item.id}: ${result}`);
  }
  if(candidates.length)core.summary.addTable(rows);else core.summary.addRaw('没有符合条件的项目。\n');
  await core.summary.write();
  if(failed)core.setFailed(`${failed} 个项目未能删除，请查看上方结果。`);
};
