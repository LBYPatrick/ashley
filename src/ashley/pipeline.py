"""Ashley Skill Pipelines.

Chains multiple skills into sequential execution. Pipelines can be
defined inline with '+' syntax (feat+commit+changelog) or as named
pipelines in ~/.ashley/config.yaml.

Each skill in the pipeline runs as a separate Claude Code session.
The first skill receives the user's question; subsequent skills
receive their standard slash command trigger.

Pipeline execution stops if any skill exits non-zero (fail-fast).
"""

import os
import shutil
import subprocess
import sys

import click

from ashley.config import AshleyConfig, get_hooks_for_skill, load_config
from ashley.hooks import run_after_hooks, run_before_hooks


def resolve_pipeline(
    pipeline_spec: str, config: AshleyConfig
) -> list[str]:
    """Resolve a pipeline spec into a list of skill names.

    Accepts:
      - '+' separated: "feat+commit+changelog"
      - Named pipeline from config: "ship" → config.pipelines["ship"]
    """
    # Check named pipelines first
    if "+" not in pipeline_spec and pipeline_spec in config.pipelines:
        return config.pipelines[pipeline_spec]

    # Parse '+' syntax
    skills = [s.strip() for s in pipeline_spec.split("+") if s.strip()]
    if not skills:
        return []

    return skills


def run_pipeline(
    pipeline_spec: str,
    question: str = "",
    dangerously_skip_permissions: bool = False,
    auto_mode: bool = False,
    away_from_keyboard: bool = False,
) -> int:
    """Execute a pipeline of skills sequentially.

    Returns the exit code of the last failed skill, or 0 if all succeeded.
    """
    config = load_config()
    skills = resolve_pipeline(pipeline_spec, config)

    if not skills:
        click.echo("Error: empty pipeline.", err=True)
        return 1

    claude_bin = shutil.which("claude")
    if not claude_bin:
        click.echo("Error: Claude Code not found.", err=True)
        return 1

    cwd = os.getcwd()

    # Display pipeline plan
    click.echo()
    click.echo(f"\033[1m  Pipeline: {' → '.join(skills)}\033[0m")
    if question:
        display_q = question[:60] + "..." if len(question) > 60 else question
        click.echo(f"  Question: {display_q}")
    click.echo()

    for i, skill in enumerate(skills, 1):
        step_label = f"[{i}/{len(skills)}]"
        click.echo(f"\033[1m  {step_label} Running: {skill}\033[0m")
        click.echo(f"  {'─' * 40}")

        # Build permission args
        from ashley.cli import _build_claude_invocation

        # Only pass question to the first skill
        skill_question = question if i == 1 else ""

        claude_args, permission_mode = _build_claude_invocation(
            skill,
            skill_question,
            dangerously_skip_permissions,
            auto_mode,
            away_from_keyboard,
        )

        hooks = get_hooks_for_skill(config, skill)

        # Before hooks
        if not run_before_hooks(hooks, skill, skill_question, cwd, permission_mode):
            click.echo(
                f"\033[0;31m  ✗ {step_label} before_run hook failed for {skill}\033[0m"
            )
            return 1

        # Record in history
        from ashley.history import record

        record(
            skill=skill,
            question=skill_question,
            cwd=cwd,
            permission=permission_mode,
        )

        # Run skill
        result = subprocess.run(claude_args)

        # After hooks
        run_after_hooks(
            hooks, skill, skill_question, cwd,
            exit_code=result.returncode,
            permission=permission_mode,
        )

        if result.returncode != 0:
            click.echo(
                f"\n\033[0;31m  ✗ Pipeline failed at step {i}: {skill} "
                f"(exit {result.returncode})\033[0m"
            )
            return result.returncode

        click.echo(f"\033[0;32m  ✓ {step_label} {skill} complete\033[0m")
        click.echo()

    click.echo(f"\033[0;32m  ✓ Pipeline complete: {' → '.join(skills)}\033[0m")
    click.echo()
    return 0
