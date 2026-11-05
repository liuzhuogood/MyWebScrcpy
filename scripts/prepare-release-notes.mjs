import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, writeFileSync, existsSync } from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

function git(root, args) {
  return execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim()
}
export function releaseNotes({ root, tag, target = 'HEAD', workingTree = false, baseTag }) {
  if (!/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(tag)) throw new Error('发布 tag 必须是带 v 的 SemVer')
  const commit = git(root, ['rev-parse', '--verify', `${target}^{commit}`])
  const candidates = ['fix', 'feature'].map(kind => `docs/versions/${tag}-${kind}/version.md`)
  const available = candidates.flatMap(file => {
    if (workingTree) return existsSync(path.join(root, file)) ? [{ file, text: readFileSync(path.join(root, file), 'utf8') }] : []
    try { return [{ file, text: git(root, ['show', `${commit}:${file}`]) }] }
    catch (error) { if (error.status === 128) return []; throw error }
  })
  if (available.length > 1) throw new Error('当前发布版本存在多个 version.md，请先合并为唯一说明')
  if (available.length) {
    const { file, text } = available[0]
    if (!text.trim() || !text.includes(tag) || !text.split('\n').some(line => line.trim() && !line.startsWith('#'))) throw new Error(`${file} 说明为空、过短或版本不匹配`)
    return { text: text.trim() + '\n', source: file, commit }
  }
  if (!baseTag) {
    const releases = JSON.parse(execFileSync('gh', ['release', 'list', '--limit', '100', '--json', 'tagName,isDraft'], { cwd: root, encoding: 'utf8' }))
    for (const release of releases) {
      if (release.isDraft || release.tagName === tag) continue
      try { git(root, ['merge-base', '--is-ancestor', `${release.tagName}^{commit}`, commit]); baseTag = release.tagName; break }
      catch { /* A release outside this target's history cannot be its baseline. */ }
    }
    if (!baseTag) throw new Error('未找到可用的最近已发布 tag；请核对历史或显式传入 --base-tag')
  }
  git(root, ['merge-base', '--is-ancestor', `${baseTag}^{commit}`, commit])
  const subjects = git(root, ['log', '--no-merges', '--format=%s', `${baseTag}..${commit}`]).split('\n').filter(Boolean)
  const files = git(root, ['diff', '--name-only', baseTag, commit]).split('\n').filter(Boolean)
  if (!subjects.length || !files.length) throw new Error('没有可生成说明的真实变更')
  const sections = [['新增', subjects.filter(s => /^feat(?:\(|:|：)/.test(s))], ['修复', subjects.filter(s => /^fix(?:\(|:|：)/.test(s))], ['其他变更', subjects.filter(s => !/^(feat|fix)(?:\(|:|：)/.test(s))]]
  const body = sections.filter(([, items]) => items.length).map(([title, items]) => `## ${title}\n\n${items.map(s => `- ${s}`).join('\n')}`).join('\n\n')
  const modules = [...new Set(files.map(f => f.includes('/') ? f.split('/').slice(0, 2).join('/') : f))]
  return { text: `# ${tag} 版本说明\n\n变更基线：${baseTag}。\n\n${body}\n\n## 涉及范围\n\n${modules.map(f => `- ${f}`).join('\n')}\n\n## 验证与限制\n\n本说明根据真实提交和文件变更生成；测试、部署和真机验收状态需另行核对，不能从提交记录推断。\n`, source: `commits:${baseTag}..${commit}`, commit }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  try {
    const args = process.argv.slice(2), tag = args.shift(), options = { root: process.cwd(), tag }
    let output
    while (args.length) {
      const key = args.shift()
      if (key === '--working-tree') options.workingTree = true
      else if (['--target', '--output', '--base-tag'].includes(key) && args.length) {
        const value = args.shift()
        if (key === '--target') options.target = value
        else if (key === '--base-tag') options.baseTag = value
        else output = value
      } else throw new Error('用法: node scripts/prepare-release-notes.mjs <tag> --output <file> [--target <commit>] [--working-tree] [--base-tag <tag>]')
    }
    if (!output) throw new Error('必须指定 --output')
    const result = releaseNotes(options)
    mkdirSync(path.dirname(path.resolve(output)), { recursive: true })
    writeFileSync(output, result.text)
    console.log(`发布说明已准备: ${output}（来源 ${result.source}，目标 ${result.commit}）`)
  } catch (error) { console.error(`[ReleaseNotes] ${error.message}`); process.exitCode = 1 }
}
