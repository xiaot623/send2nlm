const { execSync } = require('child_process');

// In npm scripts, args can be passed via `npm run release -- patch`
// Node args start at index 2
const type = process.argv.slice(2).find(arg => !arg.startsWith('--')) || process.env.npm_config_type;
const validTypes = ['patch', 'minor', 'major'];

if (!validTypes.includes(type)) {
  console.error(`\n❌ Error: Please specify a valid release type (patch, minor, or major)`);
  console.error(`\nUsage:`);
  console.error(`  npm run release:patch  # 0.0.1 -> 0.0.2`);
  console.error(`  npm run release:minor  # 0.0.1 -> 0.1.0`);
  console.error(`  npm run release:major  # 0.0.1 -> 1.0.0\n`);
  process.exit(1);
}

try {
  // Ensure git tree is clean
  const status = execSync('git status --porcelain').toString();
  if (status) {
    console.error('\n❌ Error: Working directory is not clean. Please commit or stash your changes before releasing.');
    process.exit(1);
  }

  console.log(`\n🚀 Bumping ${type} version...`);
  execSync(`npm version ${type} -m "chore: release v%s"`, { stdio: 'inherit' });
  
  const branch = execSync('git rev-parse --abbrev-ref HEAD').toString().trim();
  
  console.log(`\n📦 Pushing to origin ${branch} with tags...`);
  execSync(`git push origin ${branch} --follow-tags`, { stdio: 'inherit' });
  
  console.log(`\n✅ Release successful! GitHub Actions will now build and upload the binaries.`);
  console.log(`Once the Action completes, you can run 'npm publish' to publish the JS package.`);
} catch (error) {
  console.error('\n❌ Release process failed.', error.message);
  process.exit(1);
}
