const { execSync } = require('child_process');

execSync(`protoc \
  --plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
  --js_out=import_style=commonjs,binary:./src/generated \
  --ts_out=./src/generated \
  -I=../../proto \
  ../../proto/keel_common.proto \
  ../../proto/scheduler.proto`, { stdio: 'inherit' });