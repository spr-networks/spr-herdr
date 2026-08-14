import { cp, mkdir, rm } from 'node:fs/promises'
import { build } from 'esbuild'

await rm('dist', { recursive: true, force: true })
await mkdir('dist', { recursive: true })
await build({
  entryPoints: ['src/app.js'],
  bundle: true,
  // @xterm/xterm 6.0.0 is already identifier-minified. Re-mangling it can
  // corrupt closures in mode-query handlers used by full-screen TUIs.
  minifyIdentifiers: false,
  minifySyntax: true,
  minifyWhitespace: true,
  outdir: 'dist',
  entryNames: 'app',
  assetNames: 'assets/[name]-[hash]',
  loader: { '.woff2': 'file' },
  target: ['es2020'],
  legalComments: 'external'
})
await cp('src/index.html', 'dist/index.html')
