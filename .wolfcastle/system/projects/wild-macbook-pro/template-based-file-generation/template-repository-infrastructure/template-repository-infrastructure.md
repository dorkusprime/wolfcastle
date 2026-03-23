# Template Repository Infrastructure

Extend PromptRepository to support a templates/ prefix alongside prompts/ for artifact template resolution. Add a RenderToFile convenience method that resolves a template, executes it with a typed data struct, and writes the result atomically. Define typed context structs for each template. This is the foundation the call-site migration depends on.
