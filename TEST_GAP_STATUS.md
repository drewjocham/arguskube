# Test Gap Status

Last updated: 2026-05-10

## Utils (0 missing)
- [x] `view/src/utils/logHighlight.js`

## Composables (0 missing)
- [x] `view/src/composables/useBackgroundTasks.js`
- [x] `view/src/composables/useEvents.js`
- [x] `view/src/composables/useSpotCheck.js`
- [x] `view/src/composables/useWails.js` (barrel re-export — tested via domain composables)

## Stores (1 missing)
- [x] `view/src/stores/agentAnalysis.js`
- [x] `view/src/stores/appearance.js`
- [x] `view/src/stores/auth.js`
- [ ] `view/src/stores/navSearch.js` (not discovered in audit)

## Components (35 missing)

### HIGH Priority (8)
- [ ] `view/src/components/center/LogExplorer.vue`
- [ ] `view/src/components/workloads/PodList.vue`
- [ ] `view/src/components/center/MetricsExplorer.vue`
- [ ] `view/src/components/setup/SettingsPanel.vue`
- [ ] `view/src/components/center/AnomalyDetection.vue`
- [ ] `view/src/components/center/ArgusScanReport.vue`
- [ ] `view/src/components/center/S3Notebook.vue`
- [ ] `view/src/components/operations/WorkflowEditor.vue`

### MEDIUM Priority (19)
- [ ] `view/src/components/center/CodeBlockComponent.vue`
- [ ] `view/src/components/center/FinOpsView.vue`
- [ ] `view/src/components/center/LogStream.vue`
- [ ] `view/src/components/center/TopologyMap.vue`
- [ ] `view/src/components/center/VulnerabilityList.vue`
- [ ] `view/src/components/cluster/EventStream.vue`
- [ ] `view/src/components/cluster/NamespaceList.vue`
- [ ] `view/src/components/common/AgentAnalysisNotification.vue`
- [ ] `view/src/components/operations/ConfigAudit.vue`
- [ ] `view/src/components/operations/IncidentLog.vue`
- [ ] `view/src/components/operations/RunbookCodeBlock.vue`
- [ ] `view/src/components/operations/RunbooksView.vue`
- [ ] `view/src/components/resources/ResourceTable.vue`
- [ ] `view/src/components/resources/ResourceDetail.vue`
- [ ] `view/src/components/setup/SetupPanel.vue`
- [ ] `view/src/components/auth/LoginView.vue`
- [ ] `view/src/components/network/NetworkPolicyList.vue`
- [ ] `view/src/components/network/ServiceList.vue`
- [ ] `view/src/components/workloads/DeploymentList.vue`

### LOW Priority (8)
- [ ] `view/src/components/titlebar/EnvironmentSelector.vue`
- [ ] `view/src/components/titlebar/Titlebar.vue`
- [ ] `view/src/components/desktop/ProDesktopApp.vue`
- [ ] `view/src/components/shared/ProGateOverlay.vue`
- [ ] `view/src/components/ToastContainer.vue`
- [ ] `view/src/components/workloads/JobCronJobList.vue`
- [ ] `view/src/components/setup/DeployArtifactsPanel.vue`
- [ ] `view/src/components/notifications/NotificationsPanel.vue`

## E2E Flows (13 missing)
- [ ] Authentication — LoginView form submission and token persistence
- [ ] Pod lifecycle — list, filter, select, view logs
- [ ] Deployment scaling — scale up/down interaction
- [ ] Metrics Explorer — navigate, switch time ranges, verify chart rendering
- [ ] Log Explorer — search, filter by time, export
- [ ] ConfigMap create/edit — YAML editor interaction
- [ ] Network policies — list and filter rules
- [ ] Cluster switching — context dropdown, namespace filter
- [ ] Terminal/exec — WebSocket terminal session
- [ ] Notifications panel — open, read, dismiss
- [ ] Settings panel — toggle theme, configure preferences
- [ ] Global search — search across resources
- [ ] Anomaly Detection — run scan, view results
