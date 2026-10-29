// After `expo export`: writes into dist/sw.js the list of bundles to precache and a version
// (hash of their names) so each deploy installs fresh and drops the previous cache.
import { createHash } from 'node:crypto';
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, relative, sep } from 'node:path';

const dist = new URL('../dist/', import.meta.url).pathname;
const files = readdirSync(join(dist, '_expo/static'), { recursive: true, withFileTypes: true })
  .filter((f) => f.isFile())
  .map((f) => `/${relative(dist, join(f.parentPath, f.name)).split(sep).join('/')}`)
  .sort();
if (files.length === 0) throw new Error('precache: dist/_expo/static is empty; run expo export first');

const version = createHash('sha256').update(files.join('\n')).digest('hex').slice(0, 12);
const swPath = join(dist, 'sw.js');
const sw = readFileSync(swPath, 'utf8');
const out = sw
  .replace("const VERSION = 'dev';", `const VERSION = '${version}';`)
  .replace('const PRECACHE = [];', `const PRECACHE = ${JSON.stringify(files)};`);
if (out === sw || !out.includes(version) || !out.includes(files[0])) throw new Error('precache: placeholders not found in dist/sw.js');
writeFileSync(swPath, out);
console.log(`precache: ${files.length} files, version ${version}`);
