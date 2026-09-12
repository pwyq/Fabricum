import { spawnSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..')
mkdirSync(resolve(root,'bin'),{recursive:true})
const output=process.platform==='win32'?'bin/fabricum.exe':'bin/fabricum'
const result=spawnSync('go',['build','-trimpath','-o',output,'./back-end/cmd/fabricum'],{cwd:root,stdio:'inherit',windowsHide:true})
if(result.error)throw result.error
process.exitCode=result.status??1
