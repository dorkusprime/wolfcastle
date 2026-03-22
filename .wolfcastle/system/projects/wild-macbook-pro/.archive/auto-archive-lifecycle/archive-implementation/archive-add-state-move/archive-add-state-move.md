# Archive Add State Move

Update the wolfcastle archive add CLI command to perform the full archive operation (state directory relocation + index update), not just Markdown rollup generation. Currently archive add only writes the Markdown entry. After this change, archive add becomes the manual equivalent of the daemon's auto-archive: it generates the rollup AND moves state to .archive/ AND updates the RootIndex. This requires either extracting the move logic from the daemon into a shared function or calling through an appropriate service layer. Must include >90% test coverage.
