**Decomposition required.** This task has failed too many times to continue as-is.
Break this leaf into smaller sub-tasks using the wolfcastle CLI:

1. Create child nodes: `wolfcastle project create --node {{.NodeAddr}} --type leaf "<name>"`
2. Add tasks to each child: `wolfcastle task add --node {{.NodeAddr}}/<child-slug> --class coding/go "<description>"` (use the most specific class key from `wolfcastle config show task_classes`; each task gets one class; split tasks that would need multiple classes)
3. Emit WOLFCASTLE_YIELD when decomposition is complete.

The parent node will automatically convert from leaf to orchestrator when the first child is created.
