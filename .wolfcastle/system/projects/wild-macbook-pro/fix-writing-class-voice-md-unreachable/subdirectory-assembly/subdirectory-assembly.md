# Subdirectory Assembly

Modify ClassRepository.Resolve to append subdirectory assets (key/*.md) after resolving the main class file. After resolving key.md, call PromptRepository.ListFragments on prompts/classes/key/ to find any subdirectory .md files and concatenate their content. Add tests covering: subdirectory files are appended, missing subdirectory is a no-op, tier overrides of subdirectory files work correctly, multiple subdirectory files are sorted and appended.
