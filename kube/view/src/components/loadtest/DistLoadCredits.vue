<script setup>
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'

const store = useDistLoadStore()
const { creditBalance, creditHistory, creditsLoading } = storeToRefs(store)

function refresh() {
  store.loadCredits()
}

onMounted(() => {
  store.loadCredits()
})
</script>

<template>
  <div class="credits">
    <div class="credits-header">
      <h2>Credits &amp; Usage</h2>
      <button class="btn-sm" @click="refresh" :disabled="creditsLoading">
        {{ creditsLoading ? 'Loading…' : 'Refresh' }}
      </button>
    </div>

    <div v-if="creditsLoading && creditBalance == null" class="loading-state">Loading credit info…</div>

    <div v-else class="credits-content">
      <!-- Balance card.
           The "low credits" classes only kick in once we know the
           real balance — `creditBalance != null` guards against the
           initial-load flash where `creditBalance ?? 0` would have
           rendered 0 < 100 → "Low credits" warning while the value
           was still en route from the SaaS API. The audit flagged
           that as a confusing false alarm. -->
      <div class="balance-card" :class="{ low: creditBalance != null && creditBalance < 100 }">
        <div class="balance-amount">
          <span class="balance-number">{{ creditBalance != null ? creditBalance.toFixed(1) : '—' }}</span>
          <span class="balance-unit">credits</span>
        </div>
        <div class="balance-label">Available Balance</div>
        <div v-if="creditBalance != null && creditBalance < 100" class="balance-warning">
          Low credits — contact your Argus administrator to add more.
        </div>
      </div>

      <!-- Transaction history -->
      <div class="transactions-section">
        <h3>Transaction History</h3>
        <div v-if="!creditHistory?.length" class="empty-transactions">
          No credit transactions yet.
        </div>
        <table v-else class="transaction-table">
          <thead>
            <tr>
              <th>Date</th>
              <th>Type</th>
              <th>Amount</th>
              <th>Note</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="txn in creditHistory" :key="txn.id">
              <td class="cell-date">{{ txn.createdAt ? new Date(txn.createdAt).toLocaleDateString() : '—' }}</td>
              <td>
                <span class="txn-type" :class="`txn-${txn.type}`">{{ txn.type }}</span>
              </td>
              <td :class="txn.amount < 0 ? 'txn-debit' : 'txn-credit'">
                {{ txn.amount < 0 ? '-' : '+' }}{{ Math.abs(txn.amount).toFixed(1) }}
              </td>
              <td class="cell-note">{{ txn.note || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="credits-footer">
      Credits are managed on the Argus SaaS platform. Contact your administrator to add credits.
    </div>
  </div>
</template>

<style scoped>
.credits { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 16px; }
.credits-header { display: flex; justify-content: space-between; align-items: center; }
.credits-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
.btn-sm { padding: 4px 10px; border-radius: 6px; font-size: 11px; font-weight: 500; border: 1px solid var(--border); background: var(--bg3); color: var(--text2); cursor: pointer; }
.btn-sm:hover { background: var(--bg4); color: var(--text); }
.loading-state { text-align: center; padding: 40px; color: var(--text3); font-size: 13px; }
.credits-content { display: flex; flex-direction: column; gap: 20px; }
.balance-card { background: var(--bg2); border: 1px solid var(--border); border-radius: 12px; padding: 24px; text-align: center; }
.balance-card.low { border-color: rgba(208,156,88,0.3); background: rgba(208,156,88,0.05); }
.balance-amount { display: flex; align-items: baseline; justify-content: center; gap: 8px; }
.balance-number { font-size: 36px; font-weight: 700; color: var(--text); }
.balance-unit { font-size: 16px; color: var(--text3); }
.balance-label { font-size: 13px; color: var(--text3); margin-top: 4px; }
.balance-warning { margin-top: 12px; font-size: 12px; color: #d09c58; }
.transactions-section h3 { margin: 0 0 12px; font-size: 14px; font-weight: 600; color: var(--text); }
.empty-transactions { text-align: center; padding: 20px; color: var(--text3); font-size: 13px; }
.transaction-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.transaction-table th { text-align: left; padding: 8px 12px; color: var(--text3); font-weight: 500; border-bottom: 1px solid var(--border); }
.transaction-table td { padding: 8px 12px; border-bottom: 1px solid var(--border); color: var(--text2); }
.cell-date { white-space: nowrap; }
.txn-type { font-size: 10px; font-weight: 500; padding: 2px 6px; border-radius: 4px; }
.txn-purchase { color: #4fdc78; background: rgba(79,220,120,0.12); }
.txn-usage { color: var(--accent2); background: rgba(79,142,247,0.12); }
.txn-refund { color: #d09c58; background: rgba(208,156,88,0.12); }
.txn-debit { color: #d05858; }
.txn-credit { color: #4fdc78; }
.cell-note { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.credits-footer { text-align: center; font-size: 11px; color: var(--text3); padding: 12px; border-top: 1px solid var(--border); }
</style>
