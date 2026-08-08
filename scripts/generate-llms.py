#!/usr/bin/env python3
import os, json, frontmatter

SITE_URL = "https://leviyehonatan.github.io/ship"
DOCS_DIR = "docs"

# Collect all nav pages from mkdocs.yml
pages = [
    ("Home", "index.md"),
    ("Getting Started", "getting-started.md"),
    ("Providers", "providers.md"),
    ("Commands", "commands.md"),
    ("Secrets & Keys", "secrets.md"),
    ("Deploy", "deploy.md"),
    ("Migration", "migrating.md"),
    ("Configuration", "configuration.md"),
]

# Generate llms.txt
with open("site/llms.txt", "w") as f:
    f.write("# ship\n\n")
    f.write("Deploy containers to your own VPS. Build, push, run — one command.\n\n")
    f.write("## Docs\n\n")
    for title, path in pages:
        url = f"{SITE_URL}/{path.replace('.md', '/')}"
        f.write(f"- {title}: {url}\n")
    f.write(f"\n## Repo\n\n- https://github.com/leviyehonatan/ship\n")
    f.write(f"- Install: go install github.com/leviyehonatan/ship/cmd/ship@latest\n")
    f.write(f"\n## Full docs\n\n- {SITE_URL}/llms-full.txt\n")

# Generate llms-full.txt
with open("site/llms-full.txt", "w") as out:
    for _, path in pages:
        full_path = os.path.join(DOCS_DIR, path)
        if os.path.exists(full_path):
            with open(full_path) as f:
                content = f.read()
                post = frontmatter.loads(content)
                out.write(post.content)
                out.write("\n\n---\n\n")

print("Generated llms.txt and llms-full.txt")
