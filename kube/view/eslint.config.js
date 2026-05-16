export default [
  {
    files: ['src/**/*.js'],
    rules: {
      'no-unused-vars': 'warn',
      'no-undef': 'off',
    },
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
    },
  },
  {
    ignores: ['dist/*', 'node_modules/*', 'src/wailsjs/*'],
  },
]
