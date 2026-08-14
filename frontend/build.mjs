import { cp, mkdir, rm } from 'node:fs/promises'
import { build } from 'esbuild'

await rm('dist', { recursive: true, force: true })
await mkdir('dist', { recursive: true })
await build({
  entryPoints: ['src/app.js'],
  bundle: true,
  minify: true,
  outdir: 'dist',
  entryNames: 'app',
  assetNames: 'assets/[name]-[hash]',
  loader: { '.woff2': 'file' },
  target: ['es2020'],
  legalComments: 'external'
})
await cp('src/index.html', 'dist/index.html')
