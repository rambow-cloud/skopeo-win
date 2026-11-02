// Run: npm install --prefix .cache/store-assets --ignore-scripts --no-audit --no-fund @resvg/resvg-js@2.6.2
// Then: node hack/generate-store-assets.cjs
// Rasterize the existing project artwork; no new production dependency is added.
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '..');
const { Resvg } = require(path.join(root, '.cache/store-assets/node_modules/@resvg/resvg-js'));
const source = fs.readFileSync(path.join(root, 'docs/skopeo.svg'), 'utf8');
const output = path.join(root, 'packaging/store/Assets');
fs.mkdirSync(output, { recursive: true });
for (const [name, size] of [['StoreLogo', 50], ['Square44x44Logo', 44], ['Square150x150Logo', 150]]) {
  // Use the badge portion without the wordmark so small shell icons stay legible.
  const svg = source.replace(/<svg\s[^>]*>/, (tag) => tag
    .replace(/width="[^"]*"/, `width="${size}"`)
    .replace(/height="[^"]*"/, `height="${size}"`)
    .replace(/viewBox="[^"]*"/, 'viewBox="35 0 100 100"'));
  fs.writeFileSync(path.join(output, `${name}.png`), new Resvg(svg).render().asPng());
}
