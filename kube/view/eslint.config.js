import tseslint from 'typescript-eslint'

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
  // TypeScript lint — scoped to profile files for now. Expanding to
  // every `**/*.ts` would surface a lot of pre-existing findings the
  // profiles PR has no business carrying. Other directories can opt
  // in by adding their globs to this `files` array as they get
  // ts-lint clean.
  {
    files: [
      'src/stores/profiles.ts',
      'src/stores/profilesSync.ts',
      'src/stores/__tests__/profiles.test.ts',
    ],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: 'latest',
        sourceType: 'module',
      },
    },
    plugins: {
      '@typescript-eslint': tseslint.plugin,
    },
    rules: {
      // Catch unused imports / locals — the most common bug-shape in
      // refactors.
      '@typescript-eslint/no-unused-vars': ['error', {
        argsIgnorePattern: '^_',
        varsIgnorePattern: '^_',
      }],
      // Block accidental `any` leaks. Tests can use it freely (see
      // override below); production code must be typed.
      '@typescript-eslint/no-explicit-any': 'error',
      // Catch `Promise` returns dropped on the floor. The profiles
      // store uses `void pushX()` deliberately for fire-and-forget;
      // the rule allows that pattern via `ignoreVoid: true`.
      '@typescript-eslint/no-floating-promises': 'off', // requires type info
      // Force `import type` for type-only imports — keeps the runtime
      // bundle small.
      '@typescript-eslint/consistent-type-imports': ['error', {
        prefer: 'type-imports',
      }],
    },
  },
  {
    // Test files are allowed `any` — vi.fn() typing is intentionally
    // loose and chasing `unknown` for every mock value adds noise.
    files: ['src/**/__tests__/**/*.test.ts'],
    rules: {
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  {
    ignores: ['dist/*', 'node_modules/*', 'src/wailsjs/*'],
  },
]
