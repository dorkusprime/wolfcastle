# Daemon Coverage

Raise internal/daemon from 85.8% to 94%+. Major gaps: RunWithSupervisor (42.9%), dfsFindPlanning (62.5%), Run (68.7%), lock functions (75%), runInboxLoop (75%), autoCommitPartialWork (76.5%), checkReplanningTriggers (78.0%), runPlanningPass (79.2%). Requires SleepFunc injection (Category D) plus Category A/B test additions.
