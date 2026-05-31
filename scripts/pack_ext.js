const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const extensionDir = path.join(__dirname, '../extension');
const buildExtDir = path.join(extensionDir, 'build_ext');
const binDir = path.join(__dirname, '../bin');

try {
  console.log('📦 Packing extension...');

  if (fs.existsSync(buildExtDir)) {
    fs.rmSync(buildExtDir, { recursive: true, force: true });
  }

  fs.mkdirSync(buildExtDir, { recursive: true });
  fs.mkdirSync(binDir, { recursive: true });

  const itemsToCopy = ['dist', 'icons', '_locales', 'manifest.json'];
  for (const item of itemsToCopy) {
    const src = path.join(extensionDir, item);
    const dest = path.join(buildExtDir, item);
    if (fs.existsSync(src)) {
      execSync(`cp -r "${src}" "${dest}"`);
    }
  }

  console.log('Creating .crx file...');
  execSync(`npx --yes crx3 pack "${buildExtDir}" -o "${path.join(binDir, 'send2nlm.crx')}"`, { stdio: 'inherit', cwd: extensionDir });

  fs.rmSync(buildExtDir, { recursive: true, force: true });

  console.log('✅ Extension packed successfully into bin/send2nlm.crx');
} catch (error) {
  console.error('❌ Failed to pack extension:', error.message);
  process.exit(1);
}
