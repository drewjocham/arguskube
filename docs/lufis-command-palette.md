# lufis-terminal Command Palette — Design

Status: **proposed**, starter implementation in `lufis-terminal/internal/palette/` (k8s only). Solace / PubSub / RabbitMQ shell types ship as stubs the next PR fleshes out once the render layer is wired.

## What the user asked for (paraphrased)

> Solace, Pub/Sub and RabbitMQ need to be added to the terminal. Each shell populates tabs at the top with commonly needed commands. For example k8s would have pods, services, config-map, nodes etc. The user selects the name of the item — for example a pod's name. If a pod has 20 instances only one is rendered with text that looks like `PODNAME-*` to show it covers all of them. Clicking that opens a popup over the selected text with a scrollable textbox containing all of the pod names. The user clicks Copy or Run; Run drops the command into the main console. A hotkey or an icon runs the command in a new session in the Argus terminal.

## The shape

```
┌─ lufis-terminal window ──────────────────────────────────────┐
│ ┌─ shell tabs ──────────────────────────────────────────────┐│
│ │  [k8s] [solace] [pubsub] [rabbitmq] [shell]   ▼  ▢  ✕    ││
│ └───────────────────────────────────────────────────────────┘│
│ ┌─ command palette ─────────────────────────────────────────┐│
│ │  pods   services   configmaps   nodes   logs   exec   …   ││
│ └───────────────────────────────────────────────────────────┘│
│ ┌─ resource list (filtered by selected tab) ────────────────┐│
│ │  nginx-deployment-*                          (3 replicas) ││
│ │  redis-master-0                                            ││
│ │  payments-api-*                              (5 replicas) ││
│ │  argus-agent                                               ││
│ └───────────────────────────────────────────────────────────┘│
│ ┌─ main PTY ────────────────────────────────────────────────┐│
│ │ $ kubectl get pods                                         ││
│ │ NAME                          READY  STATUS  RESTARTS     ││
│ │ nginx-deployment-7c8-abc12    1/1    Running 0            ││
│ │ ...                                                        ││
│ │ $                                                          ││
│ └───────────────────────────────────────────────────────────┘│
└──────────────────────────────────────────────────────────────┘
```

Click on `nginx-deployment-*` → modal popup near the row:

```
┌─ Pods matching nginx-deployment-* (3) ──────────────┐
│ nginx-deployment-7c8-abc12                          │
│ nginx-deployment-7c8-def34                          │
│ nginx-deployment-7c8-ghi56                          │
├─────────────────────────────────────────────────────┤
│ [Copy]  [Run in current]  [Run in new shell ⌘↵]     │
└─────────────────────────────────────────────────────┘
```

The buttons each operate on the selected name — by default the first, but the user can click a row to select it before hitting Copy/Run.

## Why a separate palette layer

The render-side (GLFW, fonts, the popup chrome, hotkeys) is one concern. The "what commands does a k8s shell offer? how do I collapse 20 pods into one row? what's the resulting shell command?" is another. Splitting them lets:

- Each shell type (k8s, solace, pubsub, rabbitmq) own its tabs + commands without touching the render code.
- The collapsing rules be unit-tested in pure-Go land.
- The same palette ship inside Argus too (the dashboard could render its own version of the same picker without re-implementing the heuristics).

The starter implementation in this PR is the palette layer. The render layer is `_NEXT_` work (described below).

## Package layout

```
lufis-terminal/internal/palette/
  palette.go        # ShellPalette interface; Resource, Group, Palette types; Registry
  collapse.go       # name-pattern collapsing (PODNAME-* logic)
  command.go        # Command struct; how a tab + resource → shell command
  palette_test.go
  collapse_test.go

  k8s/
    k8s.go          # K8sPalette: pods, services, configmaps, nodes, logs, exec
    k8s_test.go

  solace/
    solace.go       # stub — Tabs() returns queues, topics, …; Execute() not wired yet

  pubsub/
    pubsub.go       # stub — subscriptions, topics

  rabbitmq/
    rabbitmq.go     # stub — exchanges, queues, bindings
```

## Core types

```go
// A ShellPalette is what the render layer asks for: "what commands
// does this shell offer, and what do their resource lists look like
// right now?"
type ShellPalette interface {
    // Name is the shell-tab label ("k8s", "solace", …).
    Name() string

    // Tabs are the command groups within the shell ("pods",
    // "services", …). Stable: the render layer caches them once per
    // shell instance.
    Tabs() []Tab

    // List asks the shell to enumerate resources for a tab. The
    // collapsing happens here — the palette returns Groups, each of
    // which is either one Resource or N collapsed under a wildcard.
    List(ctx context.Context, tabID string) ([]Group, error)

    // Command turns "user picked this Group, wants to do X" into a
    // shell-executable command. The render layer's Run buttons call
    // this and write the result into the PTY (or open a new PTY for
    // "Run in new shell").
    Command(tab string, action ActionID, picked Resource) (string, error)
}

type Tab struct {
    ID    string  // "pods", "services", …
    Label string  // user-facing
}

type Resource struct {
    Name      string // canonical name (no wildcard)
    Namespace string // when applicable
    Display   string // what the row shows; "<name>" or "<prefix>-*"
}

type Group struct {
    Display string     // "nginx-deployment-*" or "redis-master-0"
    Members []Resource // 1 element for un-collapsed; N for wildcard rows
}

// ActionID names the buttons the popup shows. The shell decides
// which apply (k8s: copy / run / exec / logs / describe; solace:
// copy / run / drain / browse).
type ActionID string

const (
    ActionCopy        ActionID = "copy"          // copy command to clipboard
    ActionRunCurrent  ActionID = "run-current"   // pipe into the active PTY
    ActionRunNewShell ActionID = "run-new-shell" // open a fresh shell tab + run
)
```

## Name-pattern collapsing — the heuristic

Three real-world patterns to fold:

| Source            | Examples                                                     | Suffix shape                  |
| ----------------- | ------------------------------------------------------------ | ----------------------------- |
| K8s Deployment    | `nginx-7c8d4-abc12`, `nginx-7c8d4-def34`                     | `<random5>`                   |
| K8s ReplicaSet    | `api-7c8d4-abc12`                                            | `<rs-hash9>-<random5>`        |
| Solace queue grid | `orders-queue-1`, `orders-queue-2`, … `orders-queue-12`      | numeric                       |
| Argus runner pods | `argus-run-9f23-eu`, `argus-run-9f23-us`                     | short alphanumeric suffix     |

A single rule covers all four — strip a trailing `-<short token>` (the token is alphanumeric, length 1–10) and group by the prefix. Members within a group must share the same prefix AND the same suffix *shape* (all numeric, all alphanumeric, …). That prevents over-collapsing `kafka-0` and `kafka-broker-1` into `kafka-*`.

Pseudocode:

```go
type token struct {
    prefix string
    suffix string
    shape  shapeKind // numeric | alpha | mixed
}

func Collapse(names []string, min int) []Group {
    bucket := map[token][]Resource{}
    for _, n := range names {
        t := tokenize(n)
        bucket[t.prefixKey()] = append(bucket[t.prefixKey()], Resource{Name: n})
    }
    out := []Group{}
    for k, members := range bucket {
        if len(members) < min {
            for _, m := range members {
                out = append(out, Group{Display: m.Name, Members: []Resource{m}})
            }
            continue
        }
        out = append(out, Group{Display: k.prefix + "-*", Members: members})
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Display < out[j].Display })
    return out
}
```

`min` (the collapse threshold) is configurable per palette — k8s defaults to 2 (collapse aggressively), solace defaults to 5 (the noise is lower).

## Render-layer contract (the part this PR DOESN'T ship)

The render layer is a separate PR. It needs to:

1. Read `ShellPalette.Name()` for the tab strip at the top.
2. On tab activation, read `ShellPalette.Tabs()` and render the sub-tab strip (pods / services / …).
3. On sub-tab activation, call `ShellPalette.List(ctx, tabID)` and render each `Group` as a row. Show `(N replicas)` after collapsed rows.
4. Click on a row → render the popup. Members come from `Group.Members`.
5. Each popup button calls `ShellPalette.Command(tab, action, picked)` and:
   - **Copy**: writes the result to the clipboard.
   - **Run in current**: writes `result + "\n"` to the active PTY.
   - **Run in new shell ⌘↵**: spawns a new shell tab, then writes `result + "\n"`.

The hotkey for "new shell" is `Cmd+Enter` on macOS, `Ctrl+Enter` elsewhere. The render layer owns that binding.

## Solace / PubSub / RabbitMQ — stubs in this PR, fleshed out next

The k8s palette wraps `kubectl` — no library deps required.
- Solace, PubSub, RabbitMQ need a backend channel that can enumerate their resources without the user typing the command first.
- For Solace: query the management plane via SEMP.
- For PubSub: `gcloud pubsub topics list` / `gcloud pubsub subscriptions list`.
- For RabbitMQ: management HTTP API.

The next PR adds those by implementing the `ShellPalette` interface with each broker's client library. The stubs in this PR pin the `Name()` / `Tabs()` shape so the render layer can already render the tab strip with the four labels.

## What this PR ships

- `lufis-terminal/internal/palette/palette.go` — the interface, types, registry
- `lufis-terminal/internal/palette/collapse.go` — the collapsing algorithm (with tests)
- `lufis-terminal/internal/palette/command.go` — command-string assembly
- `lufis-terminal/internal/palette/k8s/k8s.go` — k8s palette with pods + services + configmaps + nodes + logs + exec
- `lufis-terminal/internal/palette/solace/solace.go` — stub: tabs only
- `lufis-terminal/internal/palette/pubsub/pubsub.go` — stub: tabs only
- `lufis-terminal/internal/palette/rabbitmq/rabbitmq.go` — stub: tabs only
- Tests covering: collapse rules (5 corner cases incl. numeric vs alpha suffix, mixed prefixes, min-threshold), k8s command rendering, palette registry

## Open questions

1. **Real-time vs on-demand resource refresh.** Right now `List(ctx, tabID)` is on-demand — the render layer calls it when the sub-tab activates. Should it poll? For 1000-pod clusters polling is expensive; for 5-pod clusters it's free. Default: on-demand + manual refresh button, no polling.
2. **kubectl shells out from lufis or from Argus.** k8s palette runs `kubectl get pods -o name` in a subprocess. The kubeconfig is the one the spawned lufis-terminal inherited from Argus (see PR #82's KUBECONFIG handoff). No separate auth path needed.
3. **Multi-namespace.** The pod list currently inherits the kubeconfig's default namespace. Adding a namespace picker is a follow-up.

## Refs

- The user request that drove this design
- PR #82 (lufis launcher with context handoff) — KUBECONFIG already flows through
- PR #83 (terminal 403 fix) — the in-app overlay + lufis are the two places this palette ends up rendering
