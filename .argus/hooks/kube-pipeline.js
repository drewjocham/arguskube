/**
 * KubeWatcher Agent Orchestration Pipeline
 *
 * Session hooks that run specialized sub-agents before and after each
 * development session. Each agent receives scoped context (git diff,
 * coverage reports, upstream findings) instead of scanning the full repo.
 *
 * Pipeline stages:
 *   PRE-FLIGHT:  guard-rail (reads .context.md, warns about fragile code)
 *   POST-SESSION: build-check → test-suite → qa-tester → architect → test-gen → context-keeper
 *
 * Usage:
 *   Import and register with your agent SDK client:
 *
 *   import { KubePipeline } from './.kube-watcher/hooks/kube-pipeline.js'
 *   const hooks = await KubePipeline({ client })
 */

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function getChangedFiles(client) {
  const result = await client.exec('git diff --name-only HEAD~1 2>/dev/null || git diff --name-only --cached')
  return result.trim().split('\n').filter(Boolean)
}

async function getFullDiff(client) {
  return client.exec('git diff HEAD~1 2>/dev/null || git diff --cached')
}

function categorizeFiles(files) {
  const go = files.filter(f => f.endsWith('.go'))
  const vue = files.filter(f => f.endsWith('.vue') || f.endsWith('.js') || f.endsWith('.ts'))
  const config = files.filter(f => f.includes('go.mod') || f.includes('package.json') || f.includes('.config'))
  return { go, vue, config, total: files.length }
}

// ---------------------------------------------------------------------------
// Agent Definitions
// ---------------------------------------------------------------------------

const agents = {

  /** Pre-flight: loads project context and warns about known debt/fragile areas */
  'guard-rail': {
    phase: 'before',
    run: async (client, _context) => {
      return client.session.spawn({
        agentId: 'guard-rail',
        task: [
          'Read .context.md and MEMORY.md from the project root.',
          'Brief me on:',
          '1. Known critical debt and open TODOs',
          '2. Files flagged as fragile (race conditions, nil derefs)',
          '3. Any merge freezes or blockers from project memories',
          'Keep it under 200 words. Focus on what I should NOT break.',
        ].join('\n'),
      })
    },
  },

  /** Stage 1: Compile check — gates everything downstream */
  'build-check': {
    phase: 'after',
    blocking: true,
    run: async (client, context) => {
      const { go, vue } = context.categories
      const tasks = []

      if (go.length > 0) {
        tasks.push(client.exec('cd backend && go vet ./...'))
      }
      if (vue.length > 0) {
        tasks.push(client.exec('cd view && npm run build'))
      }
      if (tasks.length === 0) {
        return { passed: true, message: 'No Go or Vue files changed.' }
      }

      const results = await Promise.allSettled(tasks)
      const failed = results.filter(r => r.status === 'rejected')

      return {
        passed: failed.length === 0,
        message: failed.length === 0
          ? `Build passed (${go.length} Go, ${vue.length} Vue files).`
          : `Build FAILED:\n${failed.map(f => f.reason).join('\n')}`,
      }
    },
  },

  /** Stage 2: Run test suites — gates downstream agents */
  'test-suite': {
    phase: 'after',
    blocking: true,
    dependsOn: ['build-check'],
    run: async (client, context) => {
      const { go, vue } = context.categories
      const results = {}

      if (go.length > 0) {
        try {
          results.go = await client.exec('cd backend && go test -race -count=1 ./...')
        } catch (e) {
          results.go = `FAIL: ${e.message}`
        }
      }

      if (vue.length > 0) {
        try {
          results.vue = await client.exec('cd view && npx vitest run --reporter=verbose')
        } catch (e) {
          results.vue = `FAIL: ${e.message}`
        }
      }

      const passed = !Object.values(results).some(r => r.startsWith?.('FAIL'))
      return { passed, results }
    },
  },

  /** Stage 3: QA sweep of changed files only */
  'qa-tester': {
    phase: 'after',
    blocking: false,
    dependsOn: ['build-check'],
    run: async (client, context) => {
      if (context.files.length === 0) return { skipped: true }

      return client.session.spawn({
        agentId: 'qa-tester',
        task: [
          'Perform a functional QA sweep of ONLY these changed files:',
          context.files.map(f => `  - ${f}`).join('\n'),
          '',
          'Here is the diff:',
          '```',
          context.diff.slice(0, 8000),
          '```',
          '',
          'Check for:',
          '1. Nil pointer dereferences / undefined access',
          '2. Race conditions (concurrent map/slice access without locks)',
          '3. Error paths that swallow errors silently',
          '4. Logic bugs (off-by-one, wrong comparison operator)',
          '5. Missing input validation on public functions',
          '',
          'Report: severity (critical/high/medium) + file:line + one-line description.',
          'Under 300 words.',
        ].join('\n'),
      })
    },
  },

  /** Stage 4: Architecture review — only on significant changes */
  'architect': {
    phase: 'after',
    blocking: false,
    dependsOn: ['build-check'],
    condition: (context) => {
      // Only run when >3 files changed or new packages introduced
      return context.files.length > 3 ||
        context.files.some(f => f.includes('go.mod') || f.includes('package.json'))
    },
    run: async (client, context) => {
      return client.session.spawn({
        agentId: 'architect',
        task: [
          `Review these ${context.files.length} changed files for architectural concerns:`,
          context.files.map(f => `  - ${f}`).join('\n'),
          '',
          'Check for:',
          '1. SOLID violations (god objects, tight coupling)',
          '2. Missing abstractions (duplicated patterns that should be extracted)',
          '3. Dependency direction violations (internal importing from cmd)',
          '4. New packages: are they in the right place? Is the naming consistent?',
          '',
          'Reference .context.md for existing decisions. Report only NEW issues.',
          'Under 200 words.',
        ].join('\n'),
      })
    },
  },

  /** Stage 5: Generate missing tests for changed code */
  'test-gen': {
    phase: 'after',
    blocking: false,
    dependsOn: ['test-suite'],
    run: async (client, context) => {
      const testable = context.files.filter(f =>
        (f.endsWith('.go') && !f.endsWith('_test.go')) ||
        (f.endsWith('.js') && !f.includes('.test.') && !f.includes('.spec.'))
      )
      if (testable.length === 0) return { skipped: true, reason: 'No testable files changed.' }

      return client.session.spawn({
        agentId: 'test-gen',
        task: [
          'Write missing tests for these changed files:',
          testable.map(f => `  - ${f}`).join('\n'),
          '',
          'Rules:',
          '- Go: table-driven tests, t.TempDir(), slog discard handler, -race safe',
          '- JS/Vue: Vitest, vi.fn() mocks, vi.stubGlobal() for window/fetch',
          '- Test the happy path + 1 error path + 1 edge case per function',
          '- Write the test files directly (do not just describe them)',
          '',
          'Coverage report from test-suite:',
          JSON.stringify(context.upstream?.['test-suite']?.results || 'not available'),
        ].join('\n'),
      })
    },
  },

  /** Stage 6: Update .context.md with session results — always runs last */
  'context-keeper': {
    phase: 'after',
    blocking: true,
    dependsOn: ['qa-tester', 'architect', 'test-gen'],
    run: async (client, context) => {
      const findings = Object.entries(context.upstream || {})
        .map(([agent, result]) => `### ${agent}\n${JSON.stringify(result, null, 2)}`)
        .join('\n\n')

      return client.session.spawn({
        agentId: 'context-keeper',
        task: [
          'Update .context.md in the project root with results from this session.',
          '',
          'Changed files:',
          context.files.map(f => `  - ${f}`).join('\n'),
          '',
          'Agent findings:',
          findings.slice(0, 6000),
          '',
          'Instructions:',
          '1. Update "Last Updated" date',
          '2. Add any new architectural decisions to the Decision Log',
          '3. Update Known Technical Debt (mark resolved items, add new ones)',
          '4. Keep the file concise — this is a reference doc, not a changelog',
          '5. Do NOT add routine changes (file edits, build fixes) — only decisions and debt',
        ].join('\n'),
      })
    },
  },
}

// ---------------------------------------------------------------------------
// Pipeline Orchestrator
// ---------------------------------------------------------------------------

export const KubePipeline = async ({ client }) => {
  return {

    /**
     * Pre-flight: guard-rail agent loads context before work begins.
     */
    'session.start.before': async (session) => {
      console.log('🛡️  Running pre-flight guard-rail...')
      try {
        const result = await agents['guard-rail'].run(client)
        console.log('🛡️  Guard-rail briefing complete.')
        return result
      } catch (e) {
        console.warn('⚠️  Guard-rail failed (non-blocking):', e.message)
      }
    },

    /**
     * Post-session: full pipeline with dependency resolution.
     */
    'session.finish.before': async (session) => {
      console.log('🛠️  Starting KubeWatcher post-session pipeline...')

      // Gather context once for all agents
      const files = await getChangedFiles(client)
      if (files.length === 0) {
        console.log('📭 No changed files detected. Skipping pipeline.')
        return
      }

      const diff = await getFullDiff(client)
      const categories = categorizeFiles(files)
      const context = { files, diff, categories, upstream: {} }

      console.log(`📦 ${files.length} files changed (${categories.go.length} Go, ${categories.vue.length} Vue)`)

      // Resolve execution order from dependsOn
      const order = ['build-check', 'test-suite', 'qa-tester', 'architect', 'test-gen', 'context-keeper']

      for (const name of order) {
        const agent = agents[name]
        if (!agent || agent.phase !== 'after') continue

        // Check condition gate
        if (agent.condition && !agent.condition(context)) {
          console.log(`⏭️  Skipping ${name} (condition not met)`)
          continue
        }

        // Check dependency results for blocking failures
        const blockedBy = (agent.dependsOn || []).find(dep => {
          const upResult = context.upstream[dep]
          return upResult && upResult.passed === false
        })
        if (blockedBy) {
          console.log(`🚫 Skipping ${name} (blocked by failed ${blockedBy})`)
          continue
        }

        console.log(`▶️  Running ${name}...`)
        try {
          const result = await agent.run(client, context)
          context.upstream[name] = result

          if (agent.blocking && result?.passed === false) {
            console.error(`❌ ${name} FAILED — halting pipeline.`)
            console.error(result.message || JSON.stringify(result))
            break
          }

          console.log(`✅ ${name} complete.`)
        } catch (e) {
          console.error(`❌ ${name} threw:`, e.message)
          context.upstream[name] = { error: e.message }

          if (agent.blocking) {
            console.error(`🛑 Blocking agent failed — halting pipeline.`)
            break
          }
        }
      }

      console.log('🏁 KubeWatcher pipeline complete.')
    },
  }
}

// ---------------------------------------------------------------------------
// Standalone execution (for testing outside SDK hooks)
// ---------------------------------------------------------------------------

export const runPipelineManually = async (client) => {
  const hooks = await KubePipeline({ client })
  await hooks['session.finish.before']()
}
