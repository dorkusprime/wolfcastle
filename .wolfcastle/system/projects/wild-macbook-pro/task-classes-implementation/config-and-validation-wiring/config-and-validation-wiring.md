# Config and Validation Wiring

Wire default task_classes entries into config.Defaults() for every class that has a prompt file. Wire ClassRepository.Validate() into daemon startup to warn about missing prompts. Add --class validation to the task add CLI (reject unknown values). Auto-assign Class 'audit' to audit tasks at claim time when their class is empty.
