#!/usr/bin/env node
//
// Sincroniza la fuente de verdad de i18n (frontend/public/i18n) hacia el
// bundle (frontend/src/i18n) antes del build. Evita que las traducciones
// servidas y las embebidas en el bundle se desincronicen (fallo recurrente
// documentado en AGENTS.md / SPEC-032-033).
//
const fs = require('fs')
const path = require('path')

const srcDir = path.join(__dirname, '..', 'public', 'i18n')
const dstDir = path.join(__dirname, '..', 'src', 'i18n')

if (!fs.existsSync(srcDir)) {
  console.error(`[sync-i18n] No existe ${srcDir}`)
  process.exit(1)
}

fs.mkdirSync(dstDir, { recursive: true })

const files = fs.readdirSync(srcDir).filter((f) => f.endsWith('.json'))
for (const f of files) {
  fs.copyFileSync(path.join(srcDir, f), path.join(dstDir, f))
  console.log(`[sync-i18n] ${f}`)
}

console.log(`[sync-i18n] ${files.length} archivo(s) sincronizado(s) de public/i18n a src/i18n`)