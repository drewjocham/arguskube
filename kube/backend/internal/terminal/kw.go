package terminal

import "os"

func kwDomainEnv(domain Domain) []string {
	switch domain {
	case DomainK8s:
		return []string{"KW_DOMAIN=k8s"}
	case DomainKafka:
		return []string{"KW_DOMAIN=kafka"}
	case DomainCloud:
		return []string{"KW_DOMAIN=cloud"}
	default:
		return []string{"KW_DOMAIN=default"}
	}
}

func initKwScript() string {
	dir := os.TempDir()
	path := dir + "/.argus_kw.sh"
	if _, err := os.Stat(path); err == nil {
		return path
	}
	script := `# Argus kw — quick domain operations
kw() {
  local cmd="$1"; shift
  case "$cmd" in
    pods|po) kubectl get pods "$@" ;;
    deploy|deployments) kubectl get deployments "$@" ;;
    svc|services) kubectl get services "$@" ;;
    events|ev) kubectl get events --sort-by='.lastTimestamp' "$@" ;;
    logs) kubectl logs "$@" ;;
    ctx)
      if [ $# -eq 0 ]; then kubectl config current-context 2>/dev/null || echo "no context"
      else kubectl config use-context "$1"; fi ;;
    ns|namespace)
      if [ $# -eq 0 ]; then kubectl config view --minify -o jsonpath='{..namespace}'
      else kubectl config set-context --current --namespace "$1"; fi ;;
    desc|describe) kubectl describe "$@" ;;
    top) kubectl top "$@" ;;
    exec)
      if [ $# -lt 1 ]; then echo "Usage: kw exec <pod> [command]"; return 1; fi
      kubectl exec -it "$@" ;;
    kafka|kfk)
      local kcmd="$1"; shift
      case "$kcmd" in
        topics) kafka-topics --bootstrap-server "${KAFKA_BROKERS:-localhost:9092}" --list "$@" ;;
        consume) kcat -b "${KAFKA_BROKERS:-localhost:9092}" -t "$1" -o -10 -e ;;
        produce) kcat -b "${KAFKA_BROKERS:-localhost:9092}" -t "$1" -P ;;
        groups) kafka-consumer-groups --bootstrap-server "${KAFKA_BROKERS:-localhost:9092}" --list "$@" ;;
        *) kcat "$@" ;;
      esac ;;
    gcloud|gcp)
      if [ $# -eq 0 ]; then gcloud config list project 2>/dev/null || echo "gcloud not configured"
      else gcloud "$@"; fi ;;
    aws)
      if [ $# -eq 0 ]; then aws sts get-caller-identity 2>/dev/null || echo "AWS not configured"
      else aws "$@"; fi ;;
    help|--help|-h)
      echo "Argus kw — quick domain operations"
      echo ""
      echo "  Kubernetes:"
      echo "    kw pods, kw deploy, kw svc, kw events, kw logs <pod>"
      echo "    kw ctx [name], kw ns [name], kw exec <pod> [cmd]"
      echo "    kw top, kw desc <resource>"
      echo ""
      echo "  Kafka:"
      echo "    kw kafka topics, kw kafka consume <topic>"
      echo "    kw kafka produce <topic>, kw kafka groups"
      echo ""
      echo "  Cloud:"
      echo "    kw gcloud [args], kw aws [args]"
      ;;
    *) echo "kw: unknown command '$cmd'. Try: kw help" ;;
  esac
}
`
	_ = os.WriteFile(path, []byte(script), 0644)
	return path
}

func kwSourceCmd() string {
	path := initKwScript()
	if path == "" {
		return ""
	}
	return "source " + path + " 2>/dev/null\n"
}
