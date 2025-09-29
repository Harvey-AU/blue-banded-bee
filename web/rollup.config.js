import { nodeResolve } from '@rollup/plugin-node-resolve';
import terser from '@rollup/plugin-terser';

export default [
  // Data Binding Library build
  {
    input: 'src/bb-data-binder.js',
    output: {
      file: 'dist/bb-data-binder.js',
      format: 'iife',
      name: 'BBDataBinder'
    },
    plugins: [
      nodeResolve({
        browser: true,
        preferBuiltins: false
      })
    ]
  },
  {
    input: 'src/bb-data-binder.js',
    output: {
      file: 'dist/bb-data-binder.min.js',
      format: 'iife',
      name: 'BBDataBinder'
    },
    plugins: [
      nodeResolve({
        browser: true,
        preferBuiltins: false
      }),
      terser()
    ]
  },
  // Dashboard Actions build
  {
    input: 'src/bb-dashboard-actions.js',
    output: {
      file: 'dist/bb-dashboard-actions.js',
      format: 'iife'
    },
    plugins: [
      nodeResolve({
        browser: true,
        preferBuiltins: false
      })
    ]
  }
];