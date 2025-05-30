import { nodeResolve } from '@rollup/plugin-node-resolve';
import terser from '@rollup/plugin-terser';

export default [
  {
    input: 'src/index.js',
    output: {
      file: 'dist/bb-components.js',
      format: 'iife',
      name: 'BBComponents'
    },
    plugins: [
      nodeResolve({
        browser: true,
        preferBuiltins: false
      })
    ]
  },
  {
    input: 'src/index.js',
    output: {
      file: 'dist/bb-components.min.js',
      format: 'iife',
      name: 'BBComponents'
    },
    plugins: [
      nodeResolve({
        browser: true,
        preferBuiltins: false
      }),
      terser()
    ]
  }
];