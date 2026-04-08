# README Writing Guidelines

Write a README that is beautiful, informative, and concise. Follow this structure:

## Structure

### Header
- Centered project logo (if available) or a styled title using HTML `<h1 align="center">`.
- One-line description in bold below the title.
- Badge row for key metadata (language version, platform, version).

```markdown
<p align="center">
  <img src="res/img/logo.svg" alt="Project Name" width="120" />
</p>

<h1 align="center">Project Name</h1>

<p align="center">
  <strong>One-line project description</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/python-3.13+-3776AB?logo=python&logoColor=white" alt="Python" />
  <img src="https://img.shields.io/badge/version-1.0.0-blue" alt="Version" />
</p>
```

### Overview
- 2-3 sentences describing what the project does.
- Bulleted list of key features.

### Quick Start
- Installation: `make install`
- Build/Run: show the primary commands
- Cleanup: `make clean` / `make uninstall`

### Prerequisites
- Table format with Requirement, Version, and Notes columns.

### Project Structure
- ASCII tree diagram of the key directories.

### Usage
- Table of all Makefile targets with descriptions.
- Code blocks for common workflows.

### Development
- How to format code: `make format`
- How to commit: `make commit`
- How to run tests: `make test`

### Troubleshooting
- Use collapsible `<details>` sections for common issues.

### Contributing
- Brief workflow (branch, change, format, PR).

### License
- One line.

## Style Rules

- Keep it concise — no walls of text.
- Use tables over prose for structured information.
- Use horizontal rules (`---`) to separate major sections.
- Use code blocks generously for commands.
- Prefer imperative mood ("Install dependencies" not "You can install dependencies by...").
