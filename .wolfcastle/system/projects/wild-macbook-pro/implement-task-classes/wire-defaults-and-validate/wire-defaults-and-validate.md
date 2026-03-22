# Wire Defaults and Validate

Wire the authored class prompts into the config system and add validation at daemon startup and CLI time. Add task_classes entries to config.Defaults() for every class that has a prompt file (50+ entries). Wire ClassRepository.Validate() into daemon startup to warn about missing prompt files. Add --class validation to the task add CLI command so it rejects unknown class values. Auto-assign Class 'audit' to audit tasks at claim time when their class is empty.
