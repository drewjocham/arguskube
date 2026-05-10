package pkg

import (
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/workflows"
)

// routes returns the static dispatch table the HTTP API uses to route
// /api/<MethodName> calls. Adding or renaming a frontend-callable App method
// requires updating this map — that is intentional. The frontend already
// needs a corresponding callGo("MethodName", …) site, so the surface stays
// auditable.
//
// Keep this list grouped by feature; alphabetical inside each group.
func (a *App) routes() map[string]apiHandler {
	return map[string]apiHandler{
		// ---- Lifecycle / app mode ----
		"GetAppMode":  bind0R("GetAppMode", a.GetAppMode),
		"GetTier":     bind0R("GetTier", a.GetTier),
		"GetFeatures": bind0R("GetFeatures", a.GetFeatures),
		"SetPaused":   bind1V[bool]("SetPaused", a.SetPaused),

		// ---- Settings ----
		"GetSettings":           bind0R("GetSettings", a.GetSettings),
		"UpdateSettings":        bind1E[SettingsPayload]("UpdateSettings", a.UpdateSettings),
		"TestArgusCDConnection": bind0E("TestArgusCDConnection", a.TestArgusCDConnection),

		// ---- Cluster / contexts ----
		"GetClusterInfo":    bind0RE("GetClusterInfo", a.GetClusterInfo),
		"ListContexts":      bind0RE("ListContexts", a.ListContexts),
		"SwitchContext":     bind1E[string]("SwitchContext", a.SwitchContext),
		"ListAllNamespaces": bind0RE("ListAllNamespaces", a.ListAllNamespaces),

		// ---- Resources ----
		"ListResources":          bind2RE[string, string]("ListResources", a.ListResources),
		"GetResourceDetail":      bind3RE[string, string, string]("GetResourceDetail", a.GetResourceDetail),
		"DeletePod":              bind2E[string, string]("DeletePod", a.DeletePod),
		"RestartDeployment":      bind2E[string, string]("RestartDeployment", a.RestartDeployment),
		"ScaleDeployment":        bind3E[string, string, int32]("ScaleDeployment", a.ScaleDeployment),
		"GetDeploymentRevisions": bind3RE[string, string, int]("GetDeploymentRevisions", a.GetDeploymentRevisions),
		"GetServicePods":         bind2RE[string, string]("GetServicePods", a.GetServicePods),
		"GetVPARecommendations":  bind1RE[string]("GetVPARecommendations", a.GetVPARecommendations),
		"ListApplications":       bind1RE[string]("ListApplications", a.ListApplications),

		// ---- Topology / agent ----
		"GetTopology":      bind1RE[string]("GetTopology", a.GetTopology),
		"GetAgentTopology": bind1RE[string]("GetAgentTopology", a.GetAgentTopology),
		"ConnectToAgent":   bind1RE[string]("ConnectToAgent", a.ConnectToAgent),
		"GetAgentEventLog": bind0R("GetAgentEventLog", a.GetAgentEventLog),

		// ---- Metrics & cost ----
		"GetMetrics":             bind0RE("GetMetrics", a.GetMetrics),
		"QueryTimeSeriesMetrics": bind2RE[string, string]("QueryTimeSeriesMetrics", a.QueryTimeSeriesMetrics),
		"EstimateCosts":          bind1RE[string]("EstimateCosts", a.EstimateCosts),

		// ---- Logs ----
		"GetPodLogs":          bind3RE[string, string, int64]("GetPodLogs", a.GetPodLogs),
		"GetNodeLogs":         bind2RE[string, int]("GetNodeLogs", a.GetNodeLogs),
		"QueryLogs":           bind3RE[string, string, int]("QueryLogs", a.QueryLogs),
		"StreamPodLogsFollow": bind4RE[string, string, string, int64]("StreamPodLogsFollow", a.StreamPodLogsFollow),

		// ---- Alerts / AI / chat ----
		"GetAlerts":       bind0RE("GetAlerts", a.GetAlerts),
		"DiagnoseAlert":   bind1RE[string]("DiagnoseAlert", a.DiagnoseAlert),
		"GetAutoSummary":  bind1R[string, *ai.AutoSummary]("GetAutoSummary", a.GetAutoSummary),
		"GetChatHistory":  bind1R[string, []ai.ChatEntry]("GetChatHistory", a.GetChatHistory),
		"SendChatMessage": bind2RE[string, string]("SendChatMessage", a.SendChatMessage),

		// ---- Anomaly detection ----
		"GetAnomalyJobs":      bind0RE("GetAnomalyJobs", a.GetAnomalyJobs),
		"GetAnomalyRules":     bind0RE("GetAnomalyRules", a.GetAnomalyRules),
		"GetAnomalySettings":  bind0RE("GetAnomalySettings", a.GetAnomalySettings),
		"SaveAnomalyRule":     bind1E[anomaly.Rule]("SaveAnomalyRule", a.SaveAnomalyRule),
		"SaveAnomalySettings": bind1E[anomaly.Settings]("SaveAnomalySettings", a.SaveAnomalySettings),
		"ToggleAnomalyRule":   bind1RE[string]("ToggleAnomalyRule", a.ToggleAnomalyRule),
		"DeleteAnomalyRule":   bind1E[string]("DeleteAnomalyRule", a.DeleteAnomalyRule),

		// ---- Runbooks / incidents / workflows / notebooks ----
		"ListRunbooks":         bind0RE("ListRunbooks", a.ListRunbooks),
		"GetRunbook":           bind1RE[string]("GetRunbook", a.GetRunbook),
		"SaveRunbook":          bind2E[string, string]("SaveRunbook", a.SaveRunbook),
		"DeleteRunbook":        bind1E[string]("DeleteRunbook", a.DeleteRunbook),
		"CreateRunbook":        bind2RE[string, string]("CreateRunbook", a.CreateRunbook),
		"ListIncidents":        bind0R("ListIncidents", a.ListIncidents),
		"CreateIncident":       bind5RE[string, string, string, string, string]("CreateIncident", a.CreateIncident),
		"UpdateIncident":       bind3RE[string, string, string]("UpdateIncident", a.UpdateIncident),
		"DeleteIncident":       bind1E[string]("DeleteIncident", a.DeleteIncident),
		"ListNotebooks":        bind0RE("ListNotebooks", a.ListNotebooks),
		"GetNotebook":          bind1RE[string]("GetNotebook", a.GetNotebook),
		"SaveNotebook":         bind2E[string, string]("SaveNotebook", a.SaveNotebook),
		"DeleteNotebook":       bind1E[string]("DeleteNotebook", a.DeleteNotebook),
		"CreateNotebookFolder": bind1E[string]("CreateNotebookFolder", a.CreateNotebookFolder),
		"MoveNotebook":         bind2E[string, string]("MoveNotebook", a.MoveNotebook),
		"ListWorkflows":        bind0RE("ListWorkflows", a.ListWorkflows),
		"GetWorkflow":          bind1RE[string]("GetWorkflow", a.GetWorkflow),
		"SaveWorkflow":         bind1RE[workflows.Workflow]("SaveWorkflow", a.SaveWorkflow),
		"DeleteWorkflow":       bind1E[string]("DeleteWorkflow", a.DeleteWorkflow),

		// ---- ArgusCD ----
		"ListArgusCDApps":     bind1RE[string]("ListArgusCDApps", a.ListArgusCDApps),
		"GetArgusCDApp":       bind1RE[string]("GetArgusCDApp", a.GetArgusCDApp),
		"SyncArgusCDApp":      bind1RE[string]("SyncArgusCDApp", a.SyncArgusCDApp),
		"GetArgusCDDiffs":     bind1RE[string]("GetArgusCDDiffs", a.GetArgusCDDiffs),
		"GetArgusCDResources": bind1RE[string]("GetArgusCDResources", a.GetArgusCDResources),
		"GetArgusCDStatus":    bind0R("GetArgusCDStatus", a.GetArgusCDStatus),
		"RefreshArgusCDApp":   bind2E[string, bool]("RefreshArgusCDApp", a.RefreshArgusCDApp),
		"RollbackArgusCDApp":  bind2E[string, int64]("RollbackArgusCDApp", a.RollbackArgusCDApp),
		"SyncApplication":     bind2E[string, string]("SyncApplication", a.SyncApplication),

		// ---- Setup / scans ----
		"CheckToolStatus":     bind0R("CheckToolStatus", a.CheckToolStatus),
		"InstallArgusScan":    bind0RE("InstallArgusScan", a.InstallArgusScan),
		"DeployAgent":         bind1RE[string]("DeployAgent", a.DeployAgent),
		"UndeployAgent":       bind1RE[string]("UndeployAgent", a.UndeployAgent),
		"RunArgusScan":        bind0RE("RunArgusScan", a.RunArgusScan),
		"ScanImage":           bind2RE[string, string]("ScanImage", a.ScanImage),
		"ScanAllImages":       bind1RE[string]("ScanAllImages", a.ScanAllImages),
		"ListVulnerabilities": bind0RE("ListVulnerabilities", a.ListVulnerabilities),

		// ---- Code sandbox / suggestions ----
		"RunCodeSandbox":    bind2RE[string, string]("RunCodeSandbox", a.RunCodeSandbox),
		"GetCodeSuggestion": bind2RE[string, string]("GetCodeSuggestion", a.GetCodeSuggestion),

		// ---- Pipelines / PR review ----
		"GetPRGuidelines":        bind1RE[string]("GetPRGuidelines", a.GetPRGuidelines),
		"SavePRGuidelines":       bind2E[string, string]("SavePRGuidelines", a.SavePRGuidelines),
		"ListCodeReviewReports":  bind1RE[string]("ListCodeReviewReports", a.ListCodeReviewReports),
		"GetCodeReviewReport":    bind2RE[string, string]("GetCodeReviewReport", a.GetCodeReviewReport),
		"CreateCodeReviewReport": bind4RE[string, string, string, string]("CreateCodeReviewReport", a.CreateCodeReviewReport),
		"DeleteCodeReviewReport": bind2E[string, string]("DeleteCodeReviewReport", a.DeleteCodeReviewReport),

		// ---- Billing & usage ----
		"GetUsageSummary":    bind0RE("GetUsageSummary", a.GetUsageSummary),
		"ClearUsageHistory":  bind0E("ClearUsageHistory", a.ClearUsageHistory),
		"UpdateBillingRates": bind3E[float64, float64, float64]("UpdateBillingRates", a.UpdateBillingRates),

		// ---- Terminal / pod-exec ----
		"StartTerminal":     bind2E[int, int]("StartTerminal", a.StartTerminal),
		"SendTerminalInput": bind1E[string]("SendTerminalInput", a.SendTerminalInput),
		"ResizeTerminal":    bind2E[int, int]("ResizeTerminal", a.ResizeTerminal),
		"ExecPodShell":      bind5E[string, string, string, int, int]("ExecPodShell", a.ExecPodShell),
		"SendExecInput":     bind1E[string]("SendExecInput", a.SendExecInput),
		"ResizeExec":        bind2E[int, int]("ResizeExec", a.ResizeExec),
		"CloseExecSession":  bind0V("CloseExecSession", a.CloseExecSession),

		// ---- Auth / SaaS ----
		"LoginSaaS": bind1RE[string]("LoginSaaS", a.LoginSaaS),
	}
}
