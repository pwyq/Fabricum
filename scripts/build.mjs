import { spawnSync } from 'node:child_process'
import { mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..')
const args=process.argv.slice(2)
const outputIndex=args.indexOf('--output')
const configuredOutput=outputIndex>=0?args[outputIndex+1]:''
if(outputIndex>=0&&!configuredOutput)throw new Error('--output requires a path')
const output=configuredOutput?resolve(root,configuredOutput):resolve(root,process.platform==='win32'?'bin/fabricum.exe':'bin/fabricum')
mkdirSync(dirname(output),{recursive:true})
const result=spawnSync('go',['build','-trimpath','-ldflags=-s -w','-o',output,'./back-end/cmd/fabricum'],{cwd:root,stdio:'inherit',windowsHide:true})
if(result.error)throw result.error
process.exitCode=result.status??1
