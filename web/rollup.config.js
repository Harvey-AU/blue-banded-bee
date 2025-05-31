import { nodeResolve } from '@rollup/plugin-node-resolve';
import terser from '@rollup/plugin-terser';

export default [
  // Web Components build
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
  },
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
  }
];